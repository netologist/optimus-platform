package postgres_test

import (
	"testing"

	"github.com/optimus/projects/platform/internal/app"
	"github.com/optimus/projects/platform/internal/infra/postgres"
)

func TestStorageImplementsAppStorage(t *testing.T) {
	var _ app.Storage = (*postgres.Storage)(nil)
}
