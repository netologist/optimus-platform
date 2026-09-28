package asset_failure_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

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
		kafka    *helpers.KafkaClient
		jaeger   *helpers.JaegerClient
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
			kafka = helpers.NewKafkaClient()
			jaeger = helpers.NewJaegerClient()
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
		traceparent := helpers.NewTraceparent()
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
			return
		}

		// The remaining assertions need the real stack: a Temporal worker to run the
		// workflow, the relay to publish, and Jaeger to have collected the spans.
		ctx := context.Background()

		By("6. verifying the work order event reached Redpanda exactly once")
		// Consuming from the start of the topic is what makes the delivery count
		// meaningful: a second record under the same key is a duplicate the relay
		// produced, not something the test's own read position invented.
		woEvents, err := kafka.WaitForEvents(ctx, "events.work_order.created", workflowID, 45*time.Second, 2*time.Second)
		Expect(err).To(BeNil(), "work_order.created never reached Redpanda")
		Expect(woEvents).To(HaveLen(1), "the outbox relay must deliver the work order event once")

		woEvent := woEvents[0]
		Expect(woEvent.Key).To(Equal(workflowID))

		var woPayload map[string]any
		Expect(json.Unmarshal(woEvent.Value, &woPayload)).To(Succeed())
		Expect(woPayload["tenant_id"]).To(Equal("acme"))
		Expect(woPayload["asset_id"]).To(Equal("P-104"))
		Expect(woPayload["work_order_id"]).ToNot(BeEmpty())
		Expect(woPayload["status"]).To(Equal("OPEN"))

		By("7. verifying the signal event reached Redpanda on the same correlation id, first")
		// The run is addressable by one id only if the ingestion event carries the same
		// key, and the work order that id produced must be published after it.
		sigEvents, err := kafka.WaitForEvents(ctx, "events.signal.received", workflowID, 30*time.Second, 2*time.Second)
		Expect(err).To(BeNil(), "signal.received never reached Redpanda")
		Expect(sigEvents).To(HaveLen(1), "the outbox relay must deliver the signal event once")

		sigEvent := sigEvents[0]
		Expect(sigEvent.Key).To(Equal(workflowID))
		Expect(sigEvent.Header("traceparent")).To(Equal(traceparent),
			"the signal event must carry the trace that started the run")
		Expect(sigEvent.Timestamp.After(woEvent.Timestamp)).To(BeFalse(),
			"the signal was published after the work order it produced")

		var sigPayload map[string]any
		Expect(json.Unmarshal(sigEvent.Value, &sigPayload)).To(Succeed())
		Expect(sigPayload["tenant_id"]).To(Equal("acme"))
		Expect(sigPayload["asset_id"]).To(Equal("P-104"))
		Expect(sigPayload["workflow_id"]).To(Equal(workflowID))
		Expect(sigPayload["signal_id"]).To(Equal(resp["signal_id"]))

		By("8. verifying the work order event carries the originating W3C traceparent")
		Expect(woEvent.Header("traceparent")).To(Equal(traceparent),
			"the Kafka record must carry the trace of the request that caused it")

		By("9. verifying the audit trail joins the signal and the work order")
		trail, err := client.GetAuditTrail("acme", workflowID)
		Expect(err).To(BeNil())
		Expect(trail).To(HaveLen(2), "one workflow should yield exactly a signal and a work order event")

		eventTypes := []string{}
		for _, e := range trail {
			eventTypes = append(eventTypes, e.EventType)
			Expect(e.CorrelationID).To(Equal(workflowID))
			Expect(e.Published).To(BeTrue(), "every audit entry should have been delivered to Redpanda")
		}
		Expect(eventTypes).To(Equal([]string{"signal.received", "work_order.created"}))

		By("10. verifying the decision audit persisted the policy version that governed it")
		decisionID, _ := decData["decision_id"].(string)
		Expect(decisionID).ToNot(BeEmpty())
		decAudit, err := client.GetDecisionAudit("acme", decisionID)
		Expect(err).To(BeNil())
		Expect(decAudit.PolicyVersion).To(Equal("policy_v1"))
		Expect(decAudit.RequiresApproval).To(BeTrue())

		By("11. verifying the trace spans services end to end")
		// These spans are the evidence that the trace survived into the workflow: without
		// an explicit traceparent the worker's activity contexts carry no span at all, so
		// each of these would be missing or land in a disconnected trace of its own.
		trace, err := jaeger.WaitForTrace(ctx, "platform", workflowID, 30*time.Second, time.Hour,
			"IngestSignal", "RunDecision", "fsm.create_work_order")
		Expect(err).To(BeNil(), "trace never spanned the expected services")
		Expect(trace.HasAllSpans("IngestSignal", "RunDecision", "fsm.create_work_order")).To(BeTrue())
	})
})
