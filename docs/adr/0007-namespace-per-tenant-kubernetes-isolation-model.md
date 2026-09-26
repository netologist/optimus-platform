# Namespace-per-Tenant Kubernetes Isolation Model

## Status
Accepted

## Context
Deploying an AI-native multi-tenant enterprise SaaS platform requires strict operational, network, and resource isolation between competing enterprise tenants. Deploying a dedicated Kubernetes cluster per tenant (Cluster-per-Tenant) introduces unsustainable cloud infrastructure costs, operational sprawl, and complex cross-cluster upgrade cycles. Conversely, running all tenant workloads in a single shared namespace with purely logical software isolation risks noisy-neighbor resource starvation, secret leakage, and unrestricted pod-to-pod network lateral movement.

## Decision
We adopt a **Namespace-per-Tenant Isolation Model** within a shared multi-tenant Kubernetes cluster:
1. **Dedicated Tenant Namespaces:** Each tenant environment declared via `EnterpriseEnvironment` is provisioned into a dedicated Kubernetes namespace (`tenant-{id}`).
2. **Cluster-Wide Operator Manager:** A single instance of the `optimus-operator` runs in `optimus-system`, watching and reconciling CRDs across all tenant namespaces with scoped RBAC.
3. **Defense-in-Depth Isolation:**
   - **Network Isolation:** Default-deny `NetworkPolicy` resources restrict inter-pod communication strictly to the tenant's own integration adapters and the shared platform ingress. Cross-tenant pod traffic is blocked at the CNI layer.
   - **Resource Governance:** `ResourceQuota` and `LimitRange` objects cap CPU, memory, and pod counts per tenant, preventing noisy-neighbor starvation.
   - **Secret Isolation:** Kubernetes Secrets (vendor API keys, S3 credentials) remain strictly bound to the tenant's namespace.
   - **Data Layer Isolation:** Complemented at the database layer by PostgreSQL Row-Level Security (`SET LOCAL app.current_tenant`).

## Consequences
- Achieves high cluster resource utilization and cost efficiency while enforcing hard infrastructure boundaries.
- Provides compliance auditors with clear physical namespace, network policy, and secret perimeters.
- Keeps operator maintenance centralized in a single controller deployment without managing multi-cluster federation.
