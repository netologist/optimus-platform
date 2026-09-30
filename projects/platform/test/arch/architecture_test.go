package arch_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestArchitectureInvariants enforces the Clean Architecture & Layering rules
// defined in AGENTS.md and ADR-0008:
// 1. internal/domain must NOT import internal/app, internal/infra, internal/transport, internal/workflow
// 2. internal/app must NOT import internal/transport or internal/infra (except abstractions)
// 3. internal/tenant must NOT import internal/app, internal/infra, or internal/transport
// 4. No package under internal/ may import temporal workflow execution directly outside internal/infra or internal/workflow
func TestArchitectureInvariants(t *testing.T) {
	root := "../../internal"

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("failed to parse file %s: %v", path, err)
		}

		normalizedPath := filepath.ToSlash(path)

		for _, imp := range node.Imports {
			importPath := strings.Trim(imp.Path.Value, `"`)

			// Rule 1: domain cannot import app, infra, transport, workflow
			if strings.Contains(normalizedPath, "/internal/domain/") {
				if strings.Contains(importPath, "internal/app") ||
					strings.Contains(importPath, "internal/infra") ||
					strings.Contains(importPath, "internal/transport") ||
					strings.Contains(importPath, "internal/workflow") {
					t.Errorf("Architecture violation: domain file %s imports disallowed layer %s", path, importPath)
				}
			}

			// Rule 2: app cannot import transport
			if strings.Contains(normalizedPath, "/internal/app/") {
				if strings.Contains(importPath, "internal/transport") {
					t.Errorf("Architecture violation: app file %s imports transport %s", path, importPath)
				}
			}

			// Rule 3: tenant cannot import app, transport, infra
			if strings.Contains(normalizedPath, "/internal/tenant/") {
				if strings.Contains(importPath, "internal/app") ||
					strings.Contains(importPath, "internal/infra") ||
					strings.Contains(importPath, "internal/transport") {
					t.Errorf("Architecture violation: tenant file %s imports disallowed layer %s", path, importPath)
				}
			}

			// Rule 4: non-workflow packages cannot import temporal workflow sdk
			if !strings.Contains(normalizedPath, "/internal/workflow/") &&
				!strings.Contains(normalizedPath, "/internal/infra/temporal") &&
				!strings.Contains(normalizedPath, "/internal/bootstrap") {
				if strings.Contains(importPath, "go.temporal.io/sdk/workflow") {
					t.Errorf("Architecture violation: file %s imports temporal workflow SDK outside workflow/bootstrap: %s", path, importPath)
				}
			}
		}

		return nil
	})

	if err != nil {
		t.Fatalf("failed to walk internal directory: %v", err)
	}
}
