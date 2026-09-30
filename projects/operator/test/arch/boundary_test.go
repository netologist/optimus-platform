package arch_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestOperatorBoundaryInvariants enforces the non-negotiable Operator boundary rules:
// Defined in AGENTS.md, PRD.md §7, and ADR-0008:
// 1. Operator packages must NEVER import Temporal SDK (Temporal owns workflows, Operator owns infra lifecycle).
// 2. Operator packages must NEVER import projects/decision-service directly.
// 3. Operator packages must NEVER import projects/platform directly.
func TestOperatorBoundaryInvariants(t *testing.T) {
	rootDirs := []string{"../../internal", "../../api", "../../cmd"}

	for _, root := range rootDirs {
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

			for _, imp := range node.Imports {
				importPath := strings.Trim(imp.Path.Value, `"`)

				// Invariant 1: No Temporal
				if strings.Contains(importPath, "go.temporal.io") {
					t.Errorf("Boundary violation: Operator file %s imports Temporal SDK: %s (Operator never runs business workflows)", path, importPath)
				}

				// Invariant 2: No decision-service direct imports
				if strings.Contains(importPath, "projects/decision-service") {
					t.Errorf("Boundary violation: Operator file %s imports decision-service directly: %s", path, importPath)
				}

				// Invariant 3: No platform direct imports
				if strings.Contains(importPath, "projects/platform") {
					t.Errorf("Boundary violation: Operator file %s imports platform directly: %s", path, importPath)
				}
			}

			return nil
		})

		if err != nil {
			t.Fatalf("failed to walk directory %s: %v", root, err)
		}
	}
}
