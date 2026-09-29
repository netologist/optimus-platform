# TD-0003: Centralized Logging — Elasticsearch + Kibana (ELK) with Deferred Log Shipping

- **Status:** Open / Accepted Tech Debt (partially implemented, local-dev only)
- **Impact Area:** `deployments/overlays/kind-dev/observability-local/`, `docker-compose.yaml`
- **Target Release:** Phase 9 (Production-Quality Engineering / Enterprise Hardening)
- **Component Owner:** Platform & Observability Engineering

---

## 1. Problem & Current State

Observability covers two of the three pillars — metrics (Prometheus), traces (Jaeger), and
dashboards (Grafana) — but **logs** are not centralized. Every service writes to its own
stdout; tracing a cross-service failure (an AI investigation, a typed decision, an outbox
publish) means `kubectl logs` across eight pods with no correlation between them.

This change adds **Elasticsearch** (single-node, security disabled for the local demo) and
**Kibana** (log search/visualization UI).

## 2. What is implemented now

- `deployments/overlays/kind-dev/observability-local/elasticsearch.yaml` — single-node
  Elasticsearch 8.15.3, `xpack.security.enabled=false`, port 9200.
- `deployments/overlays/kind-dev/observability-local/kibana.yaml` — Kibana 8.15.3 pointing
  at `http://elasticsearch.optimus.svc:9200`, `XPACK_SECURITY_ENABLED=false`, port 5601.
- Ingress route `kibana.optimus.local` wired into `scripts/setup-kind-ingress.sh`
  (`cmd_ingress_all`); it is created by `mise run deploy:dev` only, because `scripts/deploy.sh`
  runs `ingress-all` only for the `kind-dev` overlay.
- Local-dev parity in `docker-compose.yaml` (services `elasticsearch` and `kibana`).

## 3. Deployment shape (kind-dev only)

Elasticsearch and Kibana — together with Prometheus and Grafana — are **not** part of
`deployments/base/observability/` any more. That base now contains Jaeger only, and the four
local UIs live in the `kind-dev`-only overlay:

```
deployments/base/observability/kustomization.yaml   → jaeger.yaml          (both overlays)
deployments/overlays/kind-dev/observability-local/  → prometheus.yaml, grafana.yaml,
                                                      elasticsearch.yaml, kibana.yaml
deployments/overlays/kind-dev/kustomization.yaml    → resources: + observability-local
deployments/overlays/kind-ci/kustomization.yaml     → unchanged (intentionally excludes them)
```

Rationale — none of the four has a consumer inside the cluster:

- The E2E suite asserts on Jaeger spans only (`e2e/helpers/jaeger.go` reads `JAEGER_URL`;
  `e2e/scenarios/asset-failure/asset_failure_test.go` waits for `IngestSignal`, `RunDecision`,
  `fsm.create_work_order`). Jaeger therefore stays in the base.
- Nothing scrapes Prometheus, renders Grafana, or ships logs into Elasticsearch — see §4.1.
- In CI the four pods only added startup cost and flake surface to the
  `kubectl wait --all --timeout=480s` gate in `.github/workflows/build-and-test.yaml`, so
  they are deliberately absent from the `kind-ci` overlay.

Cost of the split: the `kind-dev` overlay produces 12 objects more than `kind-ci`
(4 Deployments, 4 Services, 3 Grafana ConfigMaps, 1 Prometheus ConfigMap); Elasticsearch is
the heaviest of them and is skipped entirely in CI.

### 3.1 Image pull behaviour on the Kind nodes

`docker.elastic.co` is reachable only **intermittently** from the Kind nodes here: TCP probes
to the registry IP (`34.56.16.77:443`) failed in bursts (0/3 and 5/5 in two samples), and the
kubelet reported `ErrImagePull` → `ImagePullBackOff` for the first attempts
(`dial tcp 34.56.16.77:443: i/o timeout`).

The retries do land. Without any change to this repository, the nodes pulled
`elasticsearch:8.15.3` (511 MB, on `optimus-worker`) and `kibana:8.15.3` (423 MB, on
`optimus-worker2`) and both pods reached `1/1 Running`. So the first `mise run deploy:dev` can
show these two pods in `ImagePullBackOff` for a few minutes — it converges on its own, because
the kubelet keeps retrying with backoff.

The Docker daemon is **not** a better path: `docker pull` through Colima failed the same way in
the same window, so there is no pre-pull / `kind load docker-image` step in the deploy flow.
Should a pull ever stick, the manual nudge is:

```bash
kubectl get pods -n optimus -l app.kubernetes.io/name=elasticsearch -o wide   # which node?
docker exec optimus-worker crictl pull docker.elastic.co/elasticsearch/elasticsearch:8.15.3
```

Neither manifest defines a readiness probe, so `Running` alone proves nothing. Verify the stack
functionally:

```bash
kubectl exec -n optimus deploy/elasticsearch -- curl -s localhost:9200/_cluster/health  # → "status":"green"
kubectl exec -n optimus deploy/kibana        -- curl -s localhost:5601/api/status        # → "level":"available"
```

Measured on 2026-09-29: `es health: green | nodes: 1`, `kibana overall: available`.

Verify the split:

```bash
# kind-ci must NOT contain the four local UIs (expected: no output)
kustomize build deployments/overlays/kind-ci | grep -E '^  name: (grafana|prometheus|elasticsearch|kibana)$'

# kind-dev must contain all four, in namespace optimus
kustomize build deployments/overlays/kind-dev | grep -E '^  name: (grafana|prometheus|elasticsearch|kibana)$'
```

## 4. What remains deferred (why this is still tech debt)

1. **Log shipping (Fluent Bit / Filebeat).** Elasticsearch and Kibana are deployed, but
   nothing yet ships application logs into them. Until a DaemonSet shipper is added — chosen
   to fit the existing OTel/W3C story — Kibana has no data to search. The UI and the store
   are placeholders without it; this is the real remaining piece.
2. **Retention & ILM.** No index lifecycle management; indices grow unbounded in a
   long-lived cluster.
3. **Auth hardening.** ES security is disabled for the local demo; a production shape would
   re-enable `xpack.security` and wire Kibana with a service account token.
4. **Structured logging.** Services currently log unstructured; log shipping only pays off
   once logs are structured (JSON) and carry a `trace_id` to join with Jaeger traces.

Note: because the log pillar has no consumer and no shipper, keeping it out of CI costs
nothing. When the shipper lands, the question "does CI need Elasticsearch?" must be
re-opened — a shipper that CI asserts on would move these manifests back into the base.

## 5. Why this shape

- Single-node ES with security disabled matches the demo's other local tooling
  (Grafana `admin/admin`, Jaeger all-in-one) — zero-friction, no Secret bootstrap.
- ELK was chosen for the log pillar to complete the metrics → traces → logs triangle, with
  Grafana remaining the unified dashboard front-end. Each pillar keeps its right-sized tool:
  Prometheus for metrics, Jaeger for traces, Elasticsearch+Kibana for logs.
- Local-dev-only placement keeps the CI cluster to exactly the components CI consumes, while
  `mise run deploy:dev` keeps the full demo experience (dashboards + console UIs) and the
  `*.optimus.local` Ingress routes.

## 6. References & Related Documents

- [PRD.md §3](../../PRD.md) — Observability stack
- [`deployments/overlays/kind-dev/observability-local/`](../../deployments/overlays/kind-dev/observability-local/)
- [README §5.4](../../README.md) — Local TLS service URLs
