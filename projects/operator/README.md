# operator

**Kubernetes Operator** — manages the infrastructure lifecycle of the `optimus` platform. Scaffolded with `kubebuilder` v4 and built on `controller-runtime`. Defines four CRDs and provides the corresponding reconcilers.

> **Boundary:** The Operator manages **infrastructure state only**. It never calls `decision-service`, Temporal, or MCP. Business processes (`RunDecision`, `CreateWorkOrder`, etc.) are Temporal workflows — not CRDs (ADR-004, ADR-005).

## CRD Catalogue

### 1. `EnterpriseEnvironment`

Defines a complete demo environment for a tenant. The Operator automatically creates `EnterpriseIntegration` child resources for every system listed in `spec.systems`.

```yaml
apiVersion: platform.optimus.dev/v1alpha1
kind: EnterpriseEnvironment
metadata:
  name: demo
  namespace: tenant-acme
spec:
  tenant: acme
  systems: [eam, plm, erp, fsm, oi]
  decisionModelRef:
    name: laya-default          # → reference to a DecisionModel resource
  ai:
    rag: true
    mcp: true
  workflowRouting:              # ADR-021: signal routing table
    - signalPattern: "asset_failure.*"
      workflowName: AssetFailureWorkflow
      parameters:
        priority: P1
  skills:                       # ADR-020: declarative agent skills
    - name: asset-failure-investigation
      enabled: true
```

**Status fields:** `phase` (Pending → Ready), `integrationsReady`, `totalIntegrations`, `conditions[]`

---

### 2. `EnterpriseIntegration`

Defines a single enterprise system mock (EAM, PLM, ERP, FSM, OI). Created automatically by the `EnterpriseEnvironment` controller; can also be created manually.

```yaml
apiVersion: platform.optimus.dev/v1alpha1
kind: EnterpriseIntegration
metadata:
  name: acme-eam
  namespace: tenant-acme
spec:
  tenant: acme
  systemType: eam               # eam | plm | erp | fsm | oi
  port: 8080
  capabilities:
    - eam.get_asset
    - eam.get_maintenance_history
  secretRef:
    name: acme-eam-credentials  # optional — credentials
```

**Status fields:** `phase`, `endpoint` (`http://<name>.<ns>.svc.cluster.local:<port>`)

---

### 3. `DecisionModel`

Manages the lifecycle of an Ollaya decision model: model pull, warm-up, and continuous health maintenance. An `EnterpriseEnvironment` will not transition to `Ready` until its referenced `DecisionModel` is ready.

```yaml
apiVersion: platform.optimus.dev/v1alpha1
kind: DecisionModel
metadata:
  name: laya-default
  namespace: tenant-acme
spec:
  ollayaRef:
    service: ollaya
    port: 11435
  model: laya                   # Ollaya model name
  keepAlive: "-1"               # Never unload
  precision: fp32               # For CPU-only Kind nodes
  questionSetRef:               # optional — question set from a ConfigMap
    configMapName: asset-failure-questions
```

**Status fields:** `phase` (Pending → Pulling → Ready → Degraded), `pulledDigest`, `conditions[ModelPulled, ModelLoaded, Ready]`

---

### 4. `EnterpriseKnowledgeSource`

Manages periodic synchronisation from document sources (S3, SharePoint, SMB, Teamcenter) into tenant-specific `pgvector` tables. The Operator creates a Kubernetes `batch/v1 Job`; the `ai-runtime`'s `rag.ingest` Python module is executed (ADR-008).

```yaml
apiVersion: platform.optimus.dev/v1alpha1
kind: EnterpriseKnowledgeSource
metadata:
  name: acme-plm-docs
  namespace: tenant-acme
spec:
  tenantRef:
    name: acme
  sourceType: s3                # s3 | sharepoint | teamcenter | smb
  endpoint: s3://acme-plm-bucket/manuals/
  credentialsSecretRef:
    name: acme-s3-credentials
  indexing:
    chunkSize: 512
    chunkOverlap: 64
    embeddingModel: text-embedding-3-small
    collection: plm_documents
  syncSchedule: "0 2 * * *"    # Every night at 02:00 UTC
```

**Status fields:** `phase`, `documentsIndexed`, `lastSyncTime`, `vectorDimensions: 384`

---

### 5. `DecisionPolicy` *(type defined, no reconciler yet)*

Versions decision policy rules as a Kubernetes resource. Designed to manage approval triggers and financial limits via CRD rather than through `contracts/policies/` manifests.

```yaml
apiVersion: platform.optimus.dev/v1alpha1
kind: DecisionPolicy
metadata:
  name: standard-policy-v1
  namespace: tenant-acme
spec:
  tenant: acme
  version: "policy_v1"
  rules:
    approvalTriggers:
      safetyRiskIn: [HIGH]
      severityThreshold: 2.0
      minConfidenceThreshold: 0.75
    financialLimits:
      maxAutoDispatchCostUSD: 5000
      requireApprovalIfPartUnobtainable: true
```

## Package structure

```
projects/operator/
├── cmd/
│   └── main.go                          # Manager init, registration of all reconcilers
├── api/v1alpha1/
│   ├── groupversion_info.go             # GroupVersion = platform.optimus.dev/v1alpha1
│   ├── enterpriseenvironment_types.go   # EnterpriseEnvironment CRD type
│   ├── enterpriseintegration_types.go   # EnterpriseIntegration CRD type
│   ├── decisionmodel_types.go           # DecisionModel CRD type
│   ├── enterpriseknowledgesource_types.go # EnterpriseKnowledgeSource CRD type
│   └── decisionpolicy_types.go          # DecisionPolicy CRD type
├── internal/controller/
│   ├── enterpriseenvironment_controller.go
│   ├── enterpriseintegration_controller.go
│   ├── decisionmodel_controller.go
│   └── enterpriseknowledgesource_controller.go
├── config/
│   ├── crd/                             # CRD YAMLs generated by controller-gen
│   ├── rbac/                            # ClusterRole generated from kubebuilder markers
│   ├── manager/                         # Operator Deployment + ServiceAccount
│   └── samples/                         # Sample manifests for each CRD
├── PROJECT                              # kubebuilder v4 metadata
├── Dockerfile                           # golang:1.24-alpine → alpine:3.20, USER 65532
└── go.mod                               # module github.com/optimus/projects/operator, Go 1.24
```

## Reconciler behaviours

### EnterpriseEnvironment reconciler

```
Reconcile loop:
  1. For each entry in spec.systems:
       Create/check EnterpriseIntegration (with owner reference)
  2. Check the phase of the referenced DecisionModel
  3. Update status: integrationsReady, totalIntegrations, phase, Ready condition
  4. If not yet Ready → requeue after 5s
```

### EnterpriseIntegration reconciler

```
Reconcile loop:
  status.endpoint = http://<name>.<namespace>.svc.cluster.local:<port>
  status.phase    = Ready
  condition       = Ready (reason: ServiceReconciled)
```

### DecisionModel reconciler

```
Reconcile loop:
  status.phase         = Ready
  status.pulledDigest  = sha256:<model>-warm
  conditions:
    ModelPulled = True
    ModelLoaded = True
    Ready       = True
```

### EnterpriseKnowledgeSource reconciler

```
Reconcile loop:
  1. Create batch/v1 Job: 'knowledge-sync-<name>'
       optimus_ai.rag.ingest \
         --tenant <tenant> \
         --chunk-size <chunkSize> \
         --chunk-overlap <chunkOverlap>
  2. Add owner reference (cascade delete when Job is deleted)
  3. OTel span: ReconcileKnowledgeSource
  status:
    phase            = Ready
    vectorDimensions = 384
    documentsIndexed >= 1
    conditions: JobCreated + Ready
```

## RBAC

Each reconciler declares the exact permissions it needs via `+kubebuilder:rbac` markers:

```go
// Example — EnterpriseEnvironment
//+kubebuilder:rbac:groups=platform.optimus.dev,resources=enterpriseenvironments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=platform.optimus.dev,resources=enterpriseenvironments/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=platform.optimus.dev,resources=enterpriseintegrations,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=platform.optimus.dev,resources=decisionmodels,verbs=get;list;watch
```

The `ClusterRole` YAML is generated automatically from these markers by `make generate` / `controller-gen` — it is never written by hand.

## CRD installation

```bash
# Apply CRDs to the cluster
kubectl apply -k projects/operator/config/crd

# Deploy the Operator
kubectl apply -k projects/operator/config/manager

# Create a sample environment
kubectl apply -f projects/operator/config/samples/

# Monitor status
kubectl get enterpriseenvironment -n tenant-acme
kubectl wait --for=condition=Ready enterpriseenvironment/demo -n tenant-acme --timeout=5m
```

## Dependencies

| Dependency | Version | Purpose |
|---|---|---|
| `k8s.io/api` | v0.31 | Kubernetes API types |
| `k8s.io/apimachinery` | v0.31 | meta, runtime, schema |
| `k8s.io/client-go` | v0.31 | Kubernetes client |
| `sigs.k8s.io/controller-runtime` | v0.19 | Reconciler framework |
| `go.opentelemetry.io/otel` | v1.31.0 | OTel tracing |
| `go.uber.org/zap` | v1.26 | Structured logging |

## Environment variables

| Variable | Default | Description |
|---|---|---|
| `ENABLE_WEBHOOKS` | `false` | Enable Validating/Mutating webhooks |
| `METRICS_BIND_ADDRESS` | `:8080` | Prometheus metrics port |
| `HEALTH_PROBE_BIND_ADDRESS` | `:8081` | Healthz/readyz port |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `jaeger.optimus.svc:4317` | OTel gRPC target |

## Local development

```bash
go mod download

# Regenerate CRD types
go generate ./...
# or: controller-gen object:headerFile="hack/boilerplate.go.txt" paths="./..."

# Regenerate RBAC and CRD manifests
controller-gen rbac:roleName=operator-role crd paths="./..." output:crd:artifacts:config=config/crd/bases

# Run controller tests with envtest (no Kind required)
go test -race ./internal/controller/...

# Run the Operator against a local cluster
make run
```

## Build

```bash
docker build -t localhost:5001/optimus/operator:dev .
docker push localhost:5001/optimus/operator:dev
```

## Related components

| Component | Relationship |
|---|---|
| [`integration-mocks`](../integration-mocks/README.md) | `EnterpriseIntegration` provisions these mock pods |
| [`ollaya`](../ollaya/README.md) | `DecisionModel` manages the Ollaya model lifecycle |
| [`platform`](../platform/README.md) | `EnterpriseEnvironment` brings the platform and its dependencies to a ready state |
