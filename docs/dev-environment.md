# Optimus — Local Development Environment

This document explains how to set up the Optimus platform from scratch on a local machine, the day-to-day development workflow, and the most frequently used commands.

---

## Requirements

### Prerequisite Tools (Manual Installation)

The following tools are not managed by `mise`; they must be installed on the machine by hand, once.

#### 1. Homebrew (macOS package manager)

```bash
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
```

#### 2. Colima (Docker engine — free Docker Desktop alternative)

> Using Colima instead of Docker Desktop is recommended: no license restrictions, lower resource consumption.

```bash
brew install colima docker docker-compose

# Start Colima (ample resources: 4 CPU, 8 GB RAM, 60 GB disk)
colima start --cpu 4 --memory 8 --disk 60

# For automatic startup (optional):
brew services start colima
```

> **If you use Docker Desktop** you do not need to install Colima. Do not run both at the same time.

#### 3. Kind (Kubernetes in Docker)

```bash
brew install kind

# Verify the installation:
kind version
```

#### 4. kubectl

```bash
brew install kubectl

# Verify the installation:
kubectl version --client
```

#### 5. mkcert (local TLS certificate generator)

```bash
brew install mkcert nss   # nss: required for Firefox

# Add the CA to the system keychain (prompts for the sudo password):
mkcert -install

# Verify the installation:
mkcert -CAROOT
```

> The `mkcert -install` command adds the CA to the macOS Keychain. Without this step, browsers and `curl` mark the certificates as untrusted.

**Two keychains, two different browser families** — this is the most common cause of the "why does the certificate look invalid" question:

| Keychain | Read by | Script installs it? |
|---|---|---|
| `login.keychain-db` | Safari, `curl` (SecureTransport) | Yes — no sudo needed |
| `/Library/Keychains/System.keychain` | **Chrome / Brave / Edge** (Chromium-based) | Yes — sudo **required** |

`mkcert -install` writes only to the login keychain. Because Chromium-based browsers read their trust settings primarily from the **System keychain**, Brave/Chrome may still reject the certificate — while Safari opens it without issue. If `mise run ingress:certs` cannot find the CA in the System keychain, it prints the required command for you:

```bash
sudo security add-trusted-cert -d -r trustRoot \
  -k /Library/Keychains/System.keychain "$(mkcert -CAROOT)/rootCA.pem"
```

> ⚠️ **The Chromium trust store is read only at startup.** After adding the CA, **quit the browser completely** (Cmd+Q — closing the window is not enough) and reopen it. Otherwise the old certificate error keeps being served from cache.

**Verification** — `security verify-cert` calls the same `SecTrustEvaluate` engine the browsers use:

```bash
echo | openssl s_client -servername grafana.optimus.local -connect 127.0.0.1:443 2>/dev/null \
  | openssl x509 -outform PEM > /tmp/served.crt
security verify-cert -c /tmp/served.crt -p ssl -n grafana.optimus.local
# Expected: ...certificate verification successful.
```

**Wildcard certificate (managed by `scripts/setup-kind-ingress.sh`):**

| Field | Value |
|---|---|
| Location | `~/.kind-certs/optimus.local.crt` + `.key` |
| Validity | ~2 years (mkcert default) |
| Covered SANs | `optimus.local`, `*.optimus.local`, `localhost`, `127.0.0.1`, `::1` |
| Issuer | `mkcert development CA` (trusted in the login keychain) |

> **The wildcard covers a single level:** `grafana.optimus.local` ✓ — `a.b.optimus.local` ✗ (DNS standard; an extra SAN is required).

If the certificate is out of date (missing SAN, expired, or key/cert mismatch), the script regenerates it automatically:

```bash
mise run ingress:certs           # renew if needed + sync the secrets
mise run ingress:certs-force     # force regeneration (drops the existing key pair)
```

#### 6. mise (Tool version manager + task runner)

```bash
brew install mise

# Shell integration (for bash):
echo 'eval "$(mise activate bash)"' >> ~/.bashrc

# Shell integration (for zsh):
echo 'eval "$(mise activate zsh)"' >> ~/.zshrc

# Open a new shell, or:
source ~/.zshrc

# Verify the installation:
mise --version
```

#### 7. jq (JSON processing — used by scripts/setup-kind-ingress.sh)

```bash
brew install jq
```

#### 8. dnsmasq (Wildcard DNS — for `*.optimus.local`)

> Instead of adding lines to `/etc/hosts` one by one, dnsmasq automatically routes all `*.optimus.local` addresses to `127.0.0.1`. When you add a new service, there is nothing you need to do.

```bash
brew install dnsmasq
```

After installation, **two sudo commands** are required (once). `mise run kind-create` tries to run them automatically; if it cannot prompt for the password, run the following commands by hand:

```bash
# 1. Tell macOS to use dnsmasq for this domain
sudo sh -c 'mkdir -p /etc/resolver && printf "nameserver 127.0.0.1\nport 53\n" > /etc/resolver/optimus.local'

# 2. Start dnsmasq as a system service (also starts automatically on reboot)
sudo brew services start dnsmasq
```

**Automated setup (prompts for the sudo password):**

```bash
mise run ingress:dns       # dnsmasq config + resolver + service start + verification
```

**Verification:**

```bash
# Through the system resolver (the same path as browsers/curl):
ping -c1 anything.optimus.local                  # → 127.0.0.1
dscacheutil -q host -a name grafana.optimus.local  # → ip_address: 127.0.0.1

# Test dnsmasq directly:
dig +short @127.0.0.1 new-service.optimus.local  # → 127.0.0.1 (every subdomain automatic)

# All DNS + TLS ingress checks in a single command:
mise run ingress:verify
```

> **How does it work?** When macOS sees the `/etc/resolver/optimus.local` file, it routes all DNS queries ending in `.optimus.local` to `127.0.0.1:53` (dnsmasq). dnsmasq, in turn, returns `127.0.0.1` for all of them via the `address=/.optimus.local/127.0.0.1` rule. `/etc/hosts` is never modified.
>
> **Note:** `dig` (without arguments) does not use the `/etc/resolver/` mechanism on macOS — it goes straight to the `resolv.conf` servers. Use `ping`, `curl`, or `dscacheutil` to test the system resolver.

---

### Tools Installed Automatically by mise

Once the tools above are installed, the following commands download every tool defined in the repo's `.mise.toml`. You do **not** need to install them by hand.

| Tool | Version | Usage |
|---|---|---|
| `go` | 1.26 | Platform, decision-service, operator, mocks |
| `python` | 3.14 | AI Runtime (managed with uv) |
| `node` | 24 | Tooling (markdownlint etc.) |
| `kubectl` | latest | Kubernetes CLI |
| `helm` | latest | Kubernetes package manager |
| `kind` | latest | Local Kubernetes cluster |
| `kustomize` | latest | K8s manifest management |
| `buf` | latest | Protobuf compilation |
| `sqlc` | latest | SQL → Go code generation |
| `goose` | latest | DB migration tool |
| `golangci-lint` | latest | Go linter |
| `lefthook` | latest | Git hook manager |

```bash
# In the repo root:
mise install        # downloads all tools from .mise.toml
mise run bootstrap  # install Go modules, uv sync, lefthook hooks
```


---

## Full Setup in a Single Command

```bash
mise run setup
```

This command performs the following, in order:
1. Creates the Kind cluster + a local Docker registry
2. Deploys the `ingress-nginx` controller (ports 80/443 are mapped to the host)
3. Issues the `*.optimus.local` wildcard TLS certificate with `mkcert` and adds it to the keychain
4. Builds all container images and pushes them to the `localhost:5001` registry
5. Deploys all services using the Kustomize overlays (`kind-dev`)
6. Runs the DB migration and seed jobs
7. Creates the TLS Ingress resources for all services

---

## Step-by-Step Setup

```bash
# 1. Kind cluster + ingress-nginx + TLS certificates
mise run kind-create

# 2. Build and push the container images
mise run build

# 3. Deploy the services (including Ingress)
mise run deploy:dev

# 4. Reconfigure only the Ingresses (after the services are deployed)
mise run ingress:all
```

---

## Service Addresses (`*.optimus.local`)

After the `mise run kind-create` or `mise run deploy:dev` command has run, the following addresses become active.

> **DNS works automatically via dnsmasq** — if the dnsmasq setup from the Requirements section has been done, there is nothing you need to add to `/etc/hosts`. The `*.optimus.local` wildcard resolves automatically to `127.0.0.1`. Adding a new service also requires zero configuration.

| Service | URL | Usage |
|---|---|---|
| **Grafana** | https://grafana.optimus.local | admin / admin |
| **Prometheus** | https://prometheus.optimus.local | Metric queries |
| **Jaeger** | https://jaeger.optimus.local | Distributed trace viewing |
| **Redpanda Admin** | https://redpanda.optimus.local | Kafka admin API |
| **Kong API Gateway** | https://api.optimus.local/v1/ | Platform API entry point |
| **Platform API** | https://platform.optimus.local | Direct API access |
| **Decision Service** | https://decision.optimus.local | Decision service |

TLS certificates are stored under `~/.kind-certs/`. The `mkcert` CA is added to the login keychain.

### Adding a New Service

Thanks to dnsmasq, no DNS configuration is needed for a new service — just create an Ingress rule:

```bash
# Example: adding the Temporal UI
./scripts/setup-kind-ingress.sh ingress temporal-ui optimus temporal 8233 temporal.optimus.local
# https://temporal.optimus.local is immediately reachable
```

---

## Daily Development Workflow

### Fast Feedback Loop (no Kind required)

```bash
# After making a code change:
mise run test    # all Go projects + Python tests (race detector enabled)
mise run lint    # golangci-lint + ruff + mypy
```

### Pushing an Image to the Cluster and Deploying

```bash
# Rebuild and push a specific service:
docker build -t localhost:5001/optimus/platform:dev projects/platform/
docker push localhost:5001/optimus/platform:dev

# Rebuild all images:
mise run build

# Update the services (pods pull the new image with imagePullPolicy:Always):
kubectl rollout restart deployment/platform -n optimus
```

### Demo and E2E Tests

```bash
mise run demo          # Interactive terminal demo (P-104 Pump scenario)
mise run demo:mock     # Offline demo that does not require a cluster
mise run e2e           # Ginkgo E2E test suite (requires a Kind cluster)
mise run e2e:live      # E2E against the live cluster (via port-forward)
```

---

## `scripts/setup-kind-ingress.sh` — Direct Usage

The script is invoked automatically by `mise run kind-create` and `mise run deploy:dev`. It can also be used directly when needed:

```bash
# Bring up the cluster and install ingress-nginx + TLS + dnsmasq (idempotent)
./scripts/setup-kind-ingress.sh up

# Create Ingresses for all known services
./scripts/setup-kind-ingress.sh ingress-all

# Add a custom Ingress for a single service
./scripts/setup-kind-ingress.sh ingress <name> <namespace> <service> <port> <host>
# Example:
./scripts/setup-kind-ingress.sh ingress jaeger optimus jaeger 16686 jaeger.optimus.local

# Install/repair the dnsmasq wildcard DNS only (prompts for the sudo password)
./scripts/setup-kind-ingress.sh dns

# Issue/renew the wildcard certificate and sync the secrets
./scripts/setup-kind-ingress.sh certs
./scripts/setup-kind-ingress.sh certs --force     # force regeneration

# Verify DNS + TLS ingress status
./scripts/setup-kind-ingress.sh verify

# Delete the cluster
./scripts/setup-kind-ingress.sh down
```

| Subcommand | What it does | Sudo? |
|---|---|---|
| `up` | Cluster + ingress-nginx + TLS + dnsmasq | Yes (dnsmasq/`/etc/resolver`) |
| `ingress <...>` | TLS Ingress for a single service + `/etc/hosts` fallback | No |
| `ingress-all` | Ingress for all known services | No |
| `dns` | dnsmasq config + resolver + service start + verification | Yes |
| `certs [--force]` | Issue/renew the wildcard certificate + secret sync + nginx reload | No |
| `verify` | DNS resolution + test all Ingress endpoints | No |
| `down` | Delete the cluster | No |

### Adding a New Tool Later

Thanks to dnsmasq, no DNS configuration is needed for a new service — just run the `ingress` subcommand:

```bash
./scripts/setup-kind-ingress.sh ingress temporal-ui optimus temporal 8233 temporal.optimus.local
# https://temporal.optimus.local works immediately — no /etc/hosts or DNS change
```

---

## mise Task Reference

| Command | Description |
|---|---|
| `mise run setup` | Full setup from scratch (cluster + build + deploy) |
| `mise run kind-create` | Create the Kind cluster + ingress-nginx + TLS |
| `mise run kind-destroy` | Delete the Kind cluster and registry |
| `mise run build` | Build and push all container images |
| `mise run deploy` | Deploy the services (kind-dev overlay) |
| `mise run deploy:dev` | Same as `deploy`, explicit name |
| `mise run deploy:ci` | Deploy for the CI Kind cluster (kind-ci overlay) |
| `mise run migrate` | Apply the PostgreSQL schema migrations |
| `mise run seed` | Load fixture data (tenant, asset, document) |
| `mise run test` | Fast tests — no Kind required |
| `mise run lint` | Run all linters |
| `mise run e2e` | Full Kind E2E suite |
| `mise run e2e:live` | Live cluster E2E (via port-forward) |
| `mise run demo` | Interactive live demo |
| `mise run demo:mock` | Offline demo (no cluster required) |
| `mise run ingress:up` | Install ingress-nginx + TLS + dnsmasq (idempotent) |
| `mise run ingress:all` | Create Ingresses for all services |
| `mise run ingress:dns` | Set up dnsmasq wildcard DNS (prompts for the sudo password) |
| `mise run ingress:certs` | Issue/renew the wildcard certificate + secret sync |
| `mise run ingress:certs-force` | Force-regenerate the certificate |
| `mise run ingress:verify` | Verify DNS + all TLS Ingress endpoints |
| `mise run ingress:down` | Delete the cluster |
| `mise run crds-install` | Install the operator CRDs |

---

## Cluster Architecture (Kind)

```
Host Machine (macOS)
│
├── DNS: *.optimus.local → 127.0.0.1   (dnsmasq, port 53, /etc/resolver/optimus.local)
├── 127.0.0.1:80  → optimus-control-plane:80  → ingress-nginx
├── 127.0.0.1:443 → optimus-control-plane:443 → ingress-nginx (TLS termination)
├── 127.0.0.1:5001 → kind-registry:5000        → Docker image registry
│
└── Kind Cluster (optimus)
    ├── optimus-control-plane   ← ingress-nginx runs here (hostPort 80/443)
    ├── optimus-worker          ← application pods
    └── optimus-worker2         ← application pods
```

**Namespace:** All Optimus services run in the `optimus` namespace.  
**DNS:** `dnsmasq` → `*.optimus.local` wildcard → `127.0.0.1` (zero configuration for a new service).  
**TLS:** `~/.kind-certs/optimus.local.crt` — signed by the `mkcert` CA, valid until 2028.  
**Registry:** `localhost:5001/optimus/<service>:dev` — all `imagePullPolicy: Always`.

### Data Persistence

| Component | Storage | Note |
|---|---|---|
| PostgreSQL | `postgres-data` PVC (5Gi, `local-path`) | **Data is preserved** across pod restarts |
| Redpanda | Ephemeral | Dev environment — topics are reset along with the pod |
| Temporal | PostgreSQL (`temporal` + `temporal_visibility` DB) | Persisted via the PostgreSQL PVC |

**The Temporal databases** are created automatically on first setup by the `postgres-init` ConfigMap (`docker-entrypoint-initdb.d`). If the PVC is already populated, this script does not run; in that case, by hand:

```bash
kubectl exec -n optimus deployment/postgres -- \
  psql -U optimus -d postgres -c "CREATE DATABASE temporal OWNER optimus;"
kubectl exec -n optimus deployment/postgres -- \
  psql -U optimus -d postgres -c "CREATE DATABASE temporal_visibility OWNER optimus;"
kubectl rollout restart deployment/temporal -n optimus
```

### DB Migration / Seed Jobs

`optimus-db-migrate` and `optimus-db-seed` are Kubernetes **Jobs** — once completed, `kubectl apply` does not re-run them. Every time `scripts/deploy.sh` runs, it deletes and recreates these jobs, so the schema and fixtures are refreshed on every deploy.

---

## Common Issues

### Port 80/443 in Use

```bash
lsof -iTCP:80 -sTCP:LISTEN
lsof -iTCP:443 -sTCP:LISTEN
```

If Docker Desktop or another application is using these ports, shut it down.

### The Browser Does Not Trust the Certificate

**First prove that the server side is correct** — the problem is on the client side 99% of the time:

```bash
# 1. Is the server serving the right certificate?
echo | openssl s_client -servername grafana.optimus.local -connect 127.0.0.1:443 2>/dev/null \
  | openssl x509 -noout -subject -issuer -ext subjectAltName

# 2. Does the macOS trust engine (the SAME one browsers use) accept it?
echo | openssl s_client -servername grafana.optimus.local -connect 127.0.0.1:443 2>/dev/null \
  | openssl x509 -outform PEM > /tmp/served.crt
security verify-cert -c /tmp/served.crt -p ssl -n grafana.optimus.local
# "...certificate verification successful." → the server and trust chain are flawless
```

If `verify-cert` succeeds, the problem is in the **browser's trust store**, not the server. The most common cause is the CA being present only in the login keychain:

```bash
# Which keychains contain the CA?
security find-certificate -a -c "mkcert" -Z ~/Library/Keychains/login.keychain-db 2>/dev/null | grep -c "SHA-256"
security find-certificate -a -c "mkcert" -Z /Library/Keychains/System.keychain 2>/dev/null | grep -c "SHA-256"
```

If the second returns `0`, **Chromium-based browsers (Brave/Chrome/Edge)** reject the certificate — while Safari opens it without issue:

```bash
sudo security add-trusted-cert -d -r trustRoot \
  -k /Library/Keychains/System.keychain "$(mkcert -CAROOT)/rootCA.pem"
```

Then **quit the browser completely** (Cmd+Q — closing the window is not enough) and reopen it. The Chromium trust store is read only at startup; otherwise the old certificate error keeps being served from cache.

`mise run ingress:certs` performs this check automatically on every run and, if the CA is missing from the System keychain, prints the required command for you.

### The ingress-nginx Pod Does Not Start

```bash
kubectl describe pod -n ingress-nginx -l app.kubernetes.io/component=controller
kubectl logs -n ingress-nginx deployment/ingress-nginx-controller
```

### `*.optimus.local` Addresses Are Not Resolving

```bash
# 1. Is dnsmasq running?
brew services list | grep dnsmasq

# 2. Is DNS coming from dnsmasq?
dig +short @127.0.0.1 grafana.optimus.local   # → 127.0.0.1 expected

# 3. Does the macOS resolver file exist?
cat /etc/resolver/optimus.local

# 4. If not, create it and restart dnsmasq:
sudo sh -c 'mkdir -p /etc/resolver && printf "nameserver 127.0.0.1\nport 53\n" > /etc/resolver/optimus.local'
sudo brew services restart dnsmasq

# 5. Ingress and TLS secret check:
kubectl get ingress -n optimus
kubectl get secret optimus-local-tls -n optimus
```

### dnsmasq Does Not Start

```bash
# Config syntax check:
dnsmasq --test -C "$(brew --prefix)/etc/dnsmasq.conf"

# Manual debug start:
sudo "$(brew --prefix)/sbin/dnsmasq" --no-daemon -C "$(brew --prefix)/etc/dnsmasq.conf"
```

### Restart the Cluster from Scratch

```bash
mise run kind-destroy
mise run kind-create
mise run build
mise run deploy:dev
```
