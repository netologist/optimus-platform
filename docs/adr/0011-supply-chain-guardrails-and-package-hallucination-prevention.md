# Supply Chain Guardrails and Package Hallucination Prevention

## Status
Accepted

## Context
Generative AI models and coding agents are susceptible to hallucinating package names or incorporating obsolete, vulnerable, or typosquatted open-source dependencies when generating Go modules (`go.mod`) or Python virtual environments (`pyproject.toml` / `uv.lock`). Malicious actors exploit this vulnerability by publishing malicious packages matching common LLM hallucinations ("slop packages").

Furthermore, autonomous agents may introduce heavy transitive dependencies that violate project licensing guidelines (e.g., non-compliant GPL/AGPL variants in client SDKs) or introduce unvetted cryptographic algorithms.

## Decision
1. **Automated Vulnerability and Supply-Chain Scanning**:
   - **Go**: Enforce `govulncheck ./...` in CI on every PR. Any known vulnerability with an active exploit path fails the build.
   - **Python**: Enforce `uv run pip-audit` against the locked environment to verify no compromised PyPI packages are introduced.
   - **Software Bill of Materials (SBOM)**: Generate container SBOMs using `syft` and verify against vulnerability databases with `grype` prior to publishing images.

2. **Package Inception and Lockfile Verification**:
   - All dependency additions in Python must be managed through `uv add` with deterministic hashes recorded in `uv.lock`. Direct, unlocked modifications to `pyproject.toml` are prohibited.
   - Go dependencies must be cleanly verified via `go mod verify` and tidied with `go mod tidy`. Unverified checksums in `go.sum` immediately fail CI.
   - CI runs with isolated network egress during test execution (offline test flag where applicable) to prevent dynamic downloading of unpinned third-party code.

3. **License Compatibility Checks**:
   - Enforce license checks (`go-licenses` or `license-checker`) to ensure all introduced packages comply with the enterprise permissive license policy (Apache-2.0, MIT, BSD). Copyleft viral licenses are mechanically blocked from platform client binaries.

## Consequences

### Positive
- Fully neutralizes the risk of LLM package hallucination and typosquatting attacks entering production container images.
- Guarantees complete reproducibility across local development, Kind clusters, and CI runners.
- Provides cryptographic assurance of provenance through signed container digests.

### Negative / Trade-offs
- Adding a legitimate new library requires updating lockfiles and passing automated vulnerability scans.
- Occasional false positives in vulnerability scanners may require explicit triage, suppression annotations, or upstream patching.
