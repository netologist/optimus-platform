#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
echo "==> Running full environment cleanup..."
"${SCRIPT_DIR}/kind-destroy.sh" || true
rm -rf .kube/ bin/ dist/ coverage/
echo "==> Cleanup complete."
