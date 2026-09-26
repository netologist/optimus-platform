#!/usr/bin/env bash
set -euo pipefail

# Seed tenant acme operational fixtures into PostgreSQL
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
KUBECONFIG_PATH="${ROOT_DIR}/.kube/kind-optimus.yaml"

if [ -f "${KUBECONFIG_PATH}" ]; then
  export KUBECONFIG="${KUBECONFIG_PATH}"
fi

echo "==> Seeding operational fixtures for tenant 'acme' into PostgreSQL..."

SQL_SEED=$(cat <<'EOF'
-- 1. Tenant Acme
INSERT INTO tenants (id, name)
VALUES ('acme', 'Acme Industrial Manufacturing Ltd.')
ON CONFLICT (id) DO NOTHING;

-- 2. Asset P-104
INSERT INTO assets (id, tenant_id, name, asset_type, plant, status)
VALUES ('P-104', 'acme', 'Centrifugal Cooling Pump P-104', 'Centrifugal Pump', 'Manchester', 'OPERATIONAL')
ON CONFLICT (tenant_id, id) DO UPDATE SET status = 'OPERATIONAL';

-- 3. Document PLM-COOL-4021
INSERT INTO documents (id, tenant_id, title, doc_type)
VALUES ('PLM-COOL-4021', 'acme', 'Centrifugal Pump Cooling Loop Maintenance Manual', 'MAINTENANCE_MANUAL')
ON CONFLICT (tenant_id, id) DO NOTHING;

-- 4. Document Chunks
INSERT INTO document_chunks (tenant_id, document_id, chunk_index, content)
VALUES (
    'acme',
    'PLM-COOL-4021',
    1,
    'Recurrent overheating in pump cooling manifolds typically indicates thermostat failure (part SP-COOL-9981). Replace thermostat and flush secondary cooling line immediately.'
)
ON CONFLICT DO NOTHING;
EOF
)

echo "${SQL_SEED}" | kubectl exec -i -n optimus deploy/postgres -- psql -U optimus -d optimus

echo "==> Operational fixtures seeded successfully!"
