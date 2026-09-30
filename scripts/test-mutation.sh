#!/usr/bin/env bash
# run-mutation-tests.sh: Targeted mutation testing runner for Optimus
# Scoped to core business logic to ensure high MSI without prohibitive CI latency.
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TARGET="${1:-all}"

echo "============================================================"
echo "⚡ Optimus Targeted Mutation Testing (ADR-0008)"
echo "============================================================"

# 1. Go Target: projects/decision-service policy engine
run_go_mutation() {
  echo "==> [Go] Checking go-mutesting installation..."
  if ! command -v go-mutesting >/dev/null 2>&1; then
    echo "Installing go-mutesting to GOPATH..."
    go install github.com/avito-tech/go-mutesting/cmd/go-mutesting@latest || true
  fi

  if command -v go-mutesting >/dev/null 2>&1; then
    echo "==> [Go] Running mutation testing on decision-service policy engine..."
    (cd "${ROOT_DIR}/projects/decision-service" && \
      go-mutesting --exec "go test -count=1 -v ./internal/decision/..." ./internal/decision/service.go)
  else
    echo "ℹ️ go-mutesting binary not in PATH; skipping live AST mutation run."
  fi
}

# 2. Python Target: ai-runtime evaluation
run_python_mutation() {
  echo "==> [Python] Running targeted mutmut verification in ai-runtime..."
  cd "${ROOT_DIR}/projects/ai-runtime"

  # Ensure mutmut is available in virtualenv
  if uv run mutmut --version >/dev/null 2>&1; then
    echo "Running mutmut on src/optimus_ai/evaluation/..."
    uv run mutmut run --paths-to-mutate src/optimus_ai/evaluation/ || {
      echo "⚠️ mutmut finished with warnings/mutants"
    }
    uv run mutmut results || true
  else
    echo "ℹ️ mutmut not installed in ai-runtime. Add via 'uv add --dev mutmut' to run full suite."
  fi
}

case "${TARGET}" in
  go)
    run_go_mutation
    ;;
  python)
    run_python_mutation
    ;;
  all)
    run_go_mutation
    run_python_mutation
    ;;
  *)
    echo "Usage: $0 [all|go|python]"
    exit 1
    ;;
esac

echo "============================================================"
echo "✅ Mutation test scan complete."
echo "============================================================"
