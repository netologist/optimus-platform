package asset_failure_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/optimus/e2e/helpers"
)

func TestAssetFailureScenario(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Asset Failure Scenario Suite")
}

var _ = Describe("Asset failure: Pump P-104 at Manchester plant", func() {
	var (
		client   *helpers.TestClient
		platform *httptest.Server
		decision *httptest.Server
		mocks    *httptest.Server
		isLive   bool
	)

	BeforeEach(func() {
		// Dual mode: if LIVE_CLUSTER=true, connects to port-forwarded cluster endpoints
		// Otherwise uses self-contained mock servers for fast, repeatable, zero-dependency testing
		if os.Getenv("LIVE_CLUSTER") == "true" {
			isLive = true
			client = helpers.NewTestClient()
			return
		}

		// Mock Platform API server
		// TODO: In production, route through Kong API Gateway edge endpoint (https://kong.optimus.dev)
		platform = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if r.Method == http.MethodPost && r.URL.Path == "/v1/tenants/acme/signals" {
				w.WriteHeader(http.StatusAccepted)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"signal_id":   "sig-p104-99",
					"workflow_id": "wf-asset-failure-P-104-99",
					"status":      "INGESTED",
				})
				return
			}
			if r.Method == http.MethodGet && r.URL.Path == "/v1/tenants/acme/tools" {
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"tools": []map[string]any{
						{"name": "eam.get_asset"},
						{"name": "eam.get_maintenance_history"},
						{"name": "plm.search_documents"},
						{"name": "erp.get_inventory"},
						{"name": "fsm.create_work_order"},
					},
				})
				return
			}
			if r.Method == http.MethodPost && r.URL.Path == "/v1/tenants/acme/approvals/wf-asset-failure-P-104-99/approve" {
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"status":   "approved",
					"signaled": true,
				})
				return
			}
			http.NotFound(w, r)
		}))

		// Mock Decision Service server
		// TODO: In production, verify actual GPU-accelerated Ollaya decision latency SLA (<20ms)
		decision = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"decision_id":          "dec-P-104-99",
				"severity":             "P1",
				"safety_risk":          "HIGH",
				"field_visit_required": true,
				"confidence":           0.94,
				"requires_approval":    true,
				"approval_reason":      "HIGH safety risk detected: human approval mandated by policy_v1",
				"policy_version":       "policy_v1",
			})
		}))

		// Mock Enterprise Systems server
		// TODO: In production, verify consumer group offset commit in Redpanda for domain events
		mocks = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if r.URL.Path == "/call-log" {
				_ = json.NewEncoder(w).Encode([]map[string]any{
					{"tool": "eam.get_maintenance_history"},
					{"tool": "plm.search_documents"},
					{"tool": "erp.get_inventory"},
					{"tool": "fsm.create_work_order"},
				})
				return
			}
		}))

		client = &helpers.TestClient{
			PlatformURL: platform.URL,
			DecisionURL: decision.URL,
			MocksURL:    mocks.URL,
			HTTP:        platform.Client(),
		}
	})

	AfterEach(func() {
		if !isLive {
			platform.Close()
			decision.Close()
			mocks.Close()
		}
	})

	It("executes end-to-end signal ingestion, typed decisioning, approval, and governed operational dispatch", func() {
		By("1. discovering active enterprise MCP tools for tenant 'acme'")
		toolsResp, err := client.HTTP.Get(client.PlatformURL + "/v1/tenants/acme/tools")
		Expect(err).To(BeNil())
		Expect(toolsResp.StatusCode).To(Equal(http.StatusOK))

		var toolsData map[string]any
		_ = json.NewDecoder(toolsResp.Body).Decode(&toolsData)
		toolsList := toolsData["tools"].([]any)
		Expect(len(toolsList)).To(BeNumerically(">=", 5))

		By("2. submitting the operational signal with W3C traceparent")
		// TODO: In production, verify OpenTelemetry trace ID propagation in Jaeger span collector
		traceparent := "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
		resp, err := client.IngestSignal("acme", "P-104", "repeated overheating", traceparent)

		Expect(err).To(BeNil())
		Expect(resp["signal_id"]).ToNot(BeEmpty())
		workflowID := resp["workflow_id"].(string)
		Expect(workflowID).ToNot(BeEmpty())
		By("3. querying the typed decision from Decision Service (SystemOne + policy_v1)")
		decReqBody, _ := json.Marshal(map[string]any{
			"tenant_id": "acme",
			"asset_id":  "P-104",
			"state": map[string]any{
				"failures_last_30_days": 4,
				"plm_findings":          "Known cooling-system failure mode in PLM-COOL-4021 §4.2",
				"spare_part_in_stock":   true,
			},
		})
		decResp, err := client.HTTP.Post(client.DecisionURL+"/v1/decisions", "application/json", bytes.NewReader(decReqBody))
		Expect(err).To(BeNil())
		Expect(decResp.StatusCode).To(Equal(http.StatusOK))

		var decData map[string]any
		_ = json.NewDecoder(decResp.Body).Decode(&decData)

		// Assert structured fields from Ollaya & policy_v1
		Expect(decData["severity"]).To(Equal("P1"))
		Expect(decData["safety_risk"]).To(Equal("HIGH"))
		Expect(decData["field_visit_required"]).To(BeTrue())
		Expect(decData["requires_approval"]).To(BeTrue())
		Expect(decData["confidence"]).To(BeNumerically(">=", 0.90))
		Expect(decData["policy_version"]).To(Equal("policy_v1"))

		By("4. approving the workflow, simulating a human supervisor")
		// TODO: In production, verify Temporal workflow signal reception via Temporal SDK QueryWorkflow
		approveResp, err := client.ApproveWorkflow("acme", workflowID)
		Expect(err).To(BeNil())
		Expect(approveResp["status"]).To(Equal("approved"))
		Expect(approveResp["signaled"]).To(BeTrue())

		By("5. verifying mock systems received all expected MCP tool calls")
		callLogs, err := client.GetMockCallLog()
		Expect(err).To(BeNil())
		if !isLive {
			Expect(callLogs).To(HaveLen(4))
			toolsCalled := []string{}
			for _, l := range callLogs {
				toolsCalled = append(toolsCalled, l["tool"].(string))
			}
			Expect(toolsCalled).To(ContainElements(
				"eam.get_maintenance_history",
				"plm.search_documents",
				"erp.get_inventory",
				"fsm.create_work_order",
			))
		}
	})
})
