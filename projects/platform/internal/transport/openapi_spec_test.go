package transport_test

import (
	"encoding/json"
	"testing"

	"github.com/optimus/projects/platform/internal/transport"
)

func TestOpenAPISpecIsValidJSONAndDocumentsAudit(t *testing.T) {
	var spec map[string]any
	if err := json.Unmarshal(transport.GetOpenAPISpec(), &spec); err != nil {
		t.Fatalf("served OpenAPI spec is not valid JSON: %v", err)
	}
	paths, _ := spec["paths"].(map[string]any)
	if _, ok := paths["/v1/tenants/{tenant_id}/audit"]; !ok {
		t.Error("served OpenAPI spec does not document the audit endpoint")
	}
}
