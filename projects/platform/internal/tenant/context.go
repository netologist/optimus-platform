package tenant

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

type tenantKey struct{}

// Context holds tenant identification and boundaries
type Context struct {
	TenantID string
}

// WithTenant stores the tenant context
func WithTenant(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, tenantKey{}, &Context{TenantID: tenantID})
}

// FromContext extracts the tenant context
func FromContext(ctx context.Context) (*Context, error) {
	tc, ok := ctx.Value(tenantKey{}).(*Context)
	if !ok || tc == nil || tc.TenantID == "" {
		return nil, errors.New("missing or invalid tenant context")
	}
	return tc, nil
}

// Middleware extracts tenant from URL path or X-Tenant-ID header
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headerTenant := r.Header.Get("X-Tenant-ID")

		var pathTenant string
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		for i, p := range parts {
			if p == "tenants" && i+1 < len(parts) {
				pathTenant = parts[i+1]
				break
			}
		}

		if headerTenant != "" && pathTenant != "" && headerTenant != pathTenant {
			http.Error(w, `{"error":"tenant mismatch between X-Tenant-ID header and URL path"}`, http.StatusUnauthorized)
			return
		}

		tenantID := headerTenant
		if tenantID == "" {
			tenantID = pathTenant
		}

		if tenantID == "" {
			http.Error(w, `{"error":"missing tenant context (X-Tenant-ID or /v1/tenants/{id})"}`, http.StatusUnauthorized)
			return
		}
		ctx := WithTenant(r.Context(), tenantID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// SetDBSessionTenant sets PostgreSQL Row-Level Security current_tenant session parameter
func SetDBSessionTenant(ctx context.Context, db *sql.DB, tenantID string) error {
	_, err := db.ExecContext(ctx, fmt.Sprintf("SET LOCAL app.current_tenant = '%s'", tenantID))
	return err
}
