# TD-0003: Centralized Logging — Elasticsearch + Kibana (ELK), shipped by an OTel Collector DaemonSet

- **Status:** Open / Accepted Tech Debt (log shipping implemented for local dev; retention and structured logging deferred)
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
- `deployments/overlays/kind-dev/observability-local/otel-collector.yaml` — an OTel Collector
  DaemonSet (contrib 0.111.0) that tails `/var/log/pods/*/*/*.log` on every node, recovers the
  pod identity from the log path, enriches it through the Kubernetes API and bulk-indexes into
  the `optimus-logs` index (`mapping.mode: none`, a single index).
- `deployments/overlays/kind-dev/observability-local/kibana-data-view.yaml` — a Job that
  registers the `Optimus Logs` data view over `optimus-logs`, so the UI is usable straight
  after `mise run deploy:dev` (idempotent: HTTP 409 counts as success).

The shipping path is pure infrastructure: every service already writes to stdout/stderr (Go
stdlib `log`, Python stdlib `logging`, the operator's zap console encoder), so **no file under
`projects/` had to change**. Correlating log lines with Jaeger traces does need code — §4.4.

## 3. Deployment shape (kind-dev only)

Elasticsearch and Kibana — together with Prometheus and Grafana — are **not** part of
`deployments/base/observability/` any more. That base now contains Jaeger only, and the four
local UIs live in the `kind-dev`-only overlay:

```
deployments/base/observability/kustomization.yaml   → jaeger.yaml          (both overlays)
deployments/overlays/kind-dev/observability-local/  → prometheus.yaml, grafana.yaml,
                                                      elasticsearch.yaml, kibana.yaml,
                                                      otel-collector.yaml, kibana-data-view.yaml
deployments/overlays/kind-dev/kustomization.yaml    → resources: + observability-local
deployments/overlays/kind-ci/kustomization.yaml     → unchanged (intentionally excludes them)
```

Rationale — none of these has a consumer inside the cluster:

- The E2E suite asserts on Jaeger spans only (`e2e/helpers/jaeger.go` reads `JAEGER_URL`;
  `e2e/scenarios/asset-failure/asset_failure_test.go` waits for `IngestSignal`, `RunDecision`,
  `fsm.create_work_order`). Jaeger therefore stays in the base.
- Nothing scrapes Prometheus, nothing renders Grafana, and the only log producer — the
  collector that indexes into Elasticsearch — is itself part of this overlay.
- In CI the pods only added startup cost and flake surface to the
  `kubectl wait --all --timeout=480s` gate in `.github/workflows/build-and-test.yaml`, so
  they are deliberately absent from the `kind-ci` overlay.

Cost of the split: the `kind-dev` overlay produces more objects than `kind-ci`
(5 Deployments, 1 DaemonSet, 1 Job, 5 Services, 4 ConfigMaps, RBAC for the collector);
Elasticsearch is the heaviest of them and is skipped entirely in CI.

### 3.1 Image pull behaviour on the Kind nodes

`docker.elastic.co` is reachable only **intermittently** from the Kind nodes here. The
evidence is the runtime itself: containerd reports
`dial tcp 34.56.16.77:443: i/o timeout` and the kubelet logs `ErrImagePull` →
`ImagePullBackOff` for the first attempts.

(The Kind node image ships no `nc`/`curl`, so ad-hoc TCP probes from inside a node are not
usable as evidence — the container runtime's own errors are.)

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

The two shipping images hit the same wall: `otel/opentelemetry-collector-contrib:0.111.0` and
`curlimages/curl:8.10.1` (both on `registry-1.docker.io`) sat in `ImagePullBackOff` when this
was written, and the Colima Docker daemon failed those pulls 5/5 in the same window — so
neither the nodes nor the daemon could fetch them. The DaemonSet and the Job apply and
schedule correctly; they start as soon as a registry window opens. The same flakiness has been
seen against `docker.elastic.co` and `registry-1.docker.io`; it is an environment condition,
not a manifest one.

The collector **configuration** was therefore validated without a container, against the exact
version:

```bash
curl -fsSL https://github.com/open-telemetry/opentelemetry-collector-releases/releases/download/v0.111.0/otelcol-contrib_0.111.0_darwin_arm64.tar.gz | tar xz otelcol-contrib
python3 -c "import yaml;d=[x for x in yaml.safe_load_all(open('deployments/overlays/kind-dev/observability-local/otel-collector.yaml')) if x][0];print(d['data']['config.yaml'])" > /tmp/collector-config.yaml
./otelcol-contrib validate --config /tmp/collector-config.yaml    # exit 0
```

and the pipeline mechanics were exercised end-to-end locally, against the real in-cluster
Elasticsearch through a port-forward, on a synthetic `/var/log/pods`-shaped file: the CRI
envelope was parsed (`body` = the message, `log.iostream` = stdout/stderr), the pod identity
was extracted from the log path (`namespace`, `pod_name`, `uid`, `container_name`,
`restart_count`), and two records landed in the target index with the expected `@timestamp`.
What remains unverified on this machine is the in-cluster DaemonSet reading `/var/log/pods` —
that needs the image pull above.

Verify the split:

```bash
# kind-ci must NOT contain the four local UIs (expected: no output)
kustomize build deployments/overlays/kind-ci | grep -E '^  name: (grafana|prometheus|elasticsearch|kibana)$'

# kind-dev must contain all four, in namespace optimus
kustomize build deployments/overlays/kind-dev | grep -E '^  name: (grafana|prometheus|elasticsearch|kibana)$'
```

## 4. What remains deferred (why this is still tech debt)

1. **Retention & ILM.** One index, no index lifecycle management; it grows unbounded in a
   long-lived cluster.
2. **Auth hardening.** ES security is disabled for the local demo; a production shape would
   re-enable `xpack.security` and give the collector's exporter and Kibana credentials.
3. **Structured logging.** Services still log prose (`log.Printf`, stdlib `logging`) and the
   collector indexes the message verbatim; JSON with stable fields is the prerequisite for
   anything beyond substring search.
4. **Trace correlation.** Nothing injects the OTel trace id into a log line — the indexed
   documents carry `TraceFlags: 0` and no `trace_id` — so a log line cannot be joined to its
   Jaeger trace yet, even though both exist. This is the one item that needs code changes in
   `projects/` (a JSON handler plus the span context), and it is deliberately not part of this
   change.
5. **File-based application logs.** The DaemonSet reads each container's stdout/stderr only;
   an app that writes to a file inside its container needs a volume mount to be picked up.

Because the shipper lives in the `kind-dev` overlay, the CI cluster still has no log store and
no shipper, so the `kubectl wait --all` budget is untouched. If an E2E assertion ever reads
logs, the "does CI need Elasticsearch?" question must be re-opened and these manifests moved
into the base.

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
