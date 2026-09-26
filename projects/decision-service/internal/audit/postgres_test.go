package audit_test

import (
	"testing"

	"github.com/optimus/projects/decision-service/internal/audit"
)

func TestPostgresStoreImplementsStore(t *testing.T) {
	var _ audit.Store = (*audit.PostgresStore)(nil)
}
