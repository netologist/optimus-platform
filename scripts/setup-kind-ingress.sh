#!/usr/bin/env bash
set -euo pipefail

# ==============================================================================
# OPTIMUS — Kind Kubernetes TLS Ingress & Certificate Manager
#
# Usage:
#   ./scripts/setup-kind-ingress.sh up
#   ./scripts/setup-kind-ingress.sh ingress <name> <namespace> <service> <port> <host>
#   ./scripts/setup-kind-ingress.sh ingress-all
#   ./scripts/setup-kind-ingress.sh down
# ==============================================================================

CLUSTER_NAME="optimus"
CERTS_DIR="${HOME}/.kind-certs"
CERT_FILE="${CERTS_DIR}/optimus.local.crt"
KEY_FILE="${CERTS_DIR}/optimus.local.key"
SECRET_NAME="optimus-local-tls"
DOMAIN="optimus.local"
WILDCARD_DOMAIN="*.${DOMAIN}"
# SAN set passed to mkcert. The wildcard covers a single level: tool.optimus.local ✓, a.b.optimus.local ✗
CERT_SANS=("${DOMAIN}" "${WILDCARD_DOMAIN}" "localhost" "127.0.0.1" "::1")
# Subset checked during verification. IPv6 is skipped: openssl expands it to 0:0:0:0:0:0:0:1.
CERT_REQUIRED_SANS=("${DOMAIN}" "${WILDCARD_DOMAIN}" "localhost" "127.0.0.1")
INGRESS_NGINX_DEPLOY_URL="https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml"
BREW_PREFIX="$(brew --prefix 2>/dev/null || echo /opt/homebrew)"
DNSMASQ_CONF="${BREW_PREFIX}/etc/dnsmasq.d/optimus.conf"
RESOLVER_FILE="/etc/resolver/optimus.local"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# Support being in either repo root or scripts/
if [ -d "${SCRIPT_DIR}/../scripts" ]; then
  ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
else
  ROOT_DIR="${SCRIPT_DIR}"
fi
KUBECONFIG_PATH="${ROOT_DIR}/.kube/kind-optimus.yaml"

# Colors
GREEN='\033[0;32m'
CYAN='\033[0;36m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BOLD='\033[1m'
NC='\033[0m'

log_info() {
  echo -e "${CYAN}==>${NC} ${BOLD}$*${NC}"
}

log_success() {
  echo -e "${GREEN}==>${NC} ${BOLD}$*${NC}"
}

log_warn() {
  echo -e "${YELLOW}==> WARNING:${NC} $*"
}

log_error() {
  echo -e "${RED}==> ERROR:${NC} $*"
}

ensure_prereqs() {
  local missing=()
  for cmd in kind kubectl mkcert docker; do
    if ! command -v "${cmd}" >/dev/null 2>&1; then
      missing+=("${cmd}")
    fi
  done

  if [ ${#missing[@]} -gt 0 ]; then
    log_error "Missing required CLI tools: ${missing[*]}"
    echo "Please install them via Homebrew or mise:"
    echo "  brew install ${missing[*]}"
    exit 1
  fi
}

ensure_kubeconfig() {
  if [ -f "${KUBECONFIG_PATH}" ]; then
    export KUBECONFIG="${KUBECONFIG_PATH}"
  elif kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
    mkdir -p "$(dirname "${KUBECONFIG_PATH}")"
    kind get kubeconfig --name "${CLUSTER_NAME}" > "${KUBECONFIG_PATH}"
    export KUBECONFIG="${KUBECONFIG_PATH}"
  fi
}

cert_is_current() {
  [ -f "${CERT_FILE}" ] || return 1
  [ -f "${KEY_FILE}" ] || return 1

  # Does the key match the cert?
  local cert_mod key_mod
  cert_mod="$(openssl x509 -noout -modulus -in "${CERT_FILE}" 2>/dev/null | openssl md5)"
  key_mod="$(openssl rsa -noout -modulus -in "${KEY_FILE}" 2>/dev/null | openssl md5)"
  [ "${cert_mod}" = "${key_mod}" ] || return 1

  # Are all expected SANs present? Try both the DNS: and IP Address: prefixes.
  local sans san
  sans="$(openssl x509 -noout -ext subjectAltName -in "${CERT_FILE}" 2>/dev/null)"
  for san in "${CERT_REQUIRED_SANS[@]}"; do
    if printf '%s' "${sans}" | grep -qF "DNS:${san}"; then
      continue
    fi
    if printf '%s' "${sans}" | grep -qF "IP Address:${san}"; then
      continue
    fi
    return 1
  done

  # Has it expired? (renew if less than 60 days remain)
  openssl x509 -checkend 5184000 -noout -in "${CERT_FILE}" >/dev/null 2>&1 || return 1

  return 0
}

trust_ca_in_keychain() {
  [ "$(uname)" = "Darwin" ] || return 0
  local caroot
  caroot="$(mkcert -CAROOT)"
  [ -f "${caroot}/rootCA.pem" ] || return 0

  # 1. Login keychain — Safari and SecureTransport (curl) read from here, no sudo needed.
  security add-trusted-cert -d -r trustRoot \
    -k "${HOME}/Library/Keychains/login.keychain-db" \
    "${caroot}/rootCA.pem" 2>/dev/null || true

  # 2. System keychain — Chromium-based browsers (Chrome/Brave/Edge) read trust
  #    settings from here first; a CA added to the login keychain may not be
  #    enough for them. Requires sudo.
  if ca_in_system_keychain; then
    return 0
  fi

  if sudo_available; then
    log_info "Adding CA to System keychain (Chromium browsers need this)..."
    sudo security add-trusted-cert -d -r trustRoot \
      -k /Library/Keychains/System.keychain \
      "${caroot}/rootCA.pem" 2>/dev/null || true
  else
    log_warn "CA is missing from the System keychain — Chrome/Brave/Edge may reject it."
    log_warn "Run this to fix it permanently:"
    log_warn "  sudo security add-trusted-cert -d -r trustRoot \\"
    log_warn "    -k /Library/Keychains/System.keychain \"${caroot}/rootCA.pem\""
  fi
}

# Is the mkcert CA registered in the System keychain? (fingerprint comparison)
ca_in_system_keychain() {
  local caroot want have
  caroot="$(mkcert -CAROOT)"
  want="$(openssl x509 -in "${caroot}/rootCA.pem" -noout -fingerprint -sha256 2>/dev/null \
    | cut -d= -f2 | tr -d ':')"
  [ -n "${want}" ] || return 1
  have="$(security find-certificate -a -c "mkcert" -Z /Library/Keychains/System.keychain 2>/dev/null \
    | awk -F': ' '/SHA-256 hash/ {print $2}' | tr -d ':')"
  [ -n "${have}" ] || return 1
  printf '%s\n' "${have}" | grep -qiF "${want}"
}

ensure_certs() {
  # ALWAYS verify CA trust: even if the certificate is current, the trust setting
  # may have been lost (especially on the System keychain side that Chromium reads).
  trust_ca_in_keychain

  # Do not regenerate if a current certificate already exists
  if cert_is_current; then
    return 0
  fi

  mkdir -p "${CERTS_DIR}"

  if [ -f "${CERT_FILE}" ]; then
    log_info "Existing certificate is stale or incomplete — regenerating..."
  else
    log_info "Generating trusted wildcard TLS certificate for ${WILDCARD_DOMAIN}..."
  fi

  # Expand the SAN list
  mkcert -cert-file "${CERT_FILE}" -key-file "${KEY_FILE}" "${CERT_SANS[@]}" >/dev/null 2>&1
  chmod 600 "${KEY_FILE}"
  chmod 644 "${CERT_FILE}"

  log_success "Certificate issued for: ${CERT_SANS[*]}"
}

# Adds the --default-ssl-certificate argument to ingress-nginx idempotently.
# Provides a fallback for hosts that have no spec.tls block.
ensure_default_ssl_cert() {
  local arg="--default-ssl-certificate=default/${SECRET_NAME}"
  local args
  args="$(kubectl get deployment ingress-nginx-controller -n ingress-nginx \
    -o jsonpath='{.spec.template.spec.containers[0].args[*]}' 2>/dev/null || true)"

  if printf '%s' "${args}" | grep -qF -- "${arg}"; then
    return 0
  fi

  log_info "Setting wildcard TLS as default SSL certificate..."
  local patch
  patch="$(kubectl get deployment ingress-nginx-controller -n ingress-nginx -o json 2>/dev/null \
    | jq -c --arg a "${arg}" '{spec:{template:{spec:{containers:[{name:.spec.template.spec.containers[0].name,args:(.spec.template.spec.containers[0].args + [$a])}]}}}}')"
  kubectl patch deployment ingress-nginx-controller -n ingress-nginx --type=strategic -p "${patch}" >/dev/null
}

sync_tls_secret() {
  local target_ns="$1"
  ensure_certs
  ensure_kubeconfig

  # Check if namespace exists, create if not
  if ! kubectl get namespace "${target_ns}" >/dev/null 2>&1; then
    kubectl create namespace "${target_ns}" >/dev/null 2>&1 || true
  fi

  kubectl create secret tls "${SECRET_NAME}" \
    --cert="${CERT_FILE}" \
    --key="${KEY_FILE}" \
    --namespace="${target_ns}" \
    --dry-run=client -o yaml | kubectl apply -f - >/dev/null
}

# If dnsmasq is present there is no need to write /etc/hosts; kept as a fallback
update_hosts_file() {
  local host="$1"
  local ip="127.0.0.1"

  # If dnsmasq is active, *.optimus.local already resolves via the wildcard, skip
  if dnsmasq_is_active; then
    return 0
  fi

  if grep -q -E "^[[:space:]]*${ip}[[:space:]]+${host}([[:space:]]|$)" /etc/hosts; then
    return 0
  fi

  if sudo -n true 2>/dev/null; then
    echo "${ip} ${host}" | sudo tee -a /etc/hosts >/dev/null
    log_success "Added ${ip} ${host} to /etc/hosts"
  else
    echo -e "${YELLOW}[!] Add:${NC} ${BOLD}echo '${ip} ${host}' | sudo tee -a /etc/hosts${NC}"
  fi
}

dnsmasq_is_active() {
  # dnsmasq installed + resolver file present + listening on 127.0.0.1:53
  command -v dnsmasq >/dev/null 2>&1 \
    && [ -f "${RESOLVER_FILE}" ] \
    && dig +short +time=1 @127.0.0.1 test.optimus.local >/dev/null 2>&1
}

# Is sudo access available? Prompts for a password on an interactive terminal, otherwise checks for passwordless.
sudo_available() {
  if [ -t 0 ] && [ -t 1 ]; then
    sudo -v 2>/dev/null
  else
    sudo -n true 2>/dev/null
  fi
}

verify_dns() {
  local probe="probe.${DOMAIN}"
  local ok=0

  # (a) dnsmasq directly — is the config correct?
  local direct
  direct="$(dig +short +time=1 +tries=1 @127.0.0.1 "${probe}" 2>/dev/null | head -1)"
  if [ "${direct}" = "127.0.0.1" ]; then
    log_success "dnsmasq: *.${DOMAIN} → 127.0.0.1"
    ok=1
  else
    log_warn "dnsmasq is not responding (${probe} → '${direct:-empty}')"
  fi

  # (b) system resolver (mDNSResponder) — /etc/resolver integration
  local system_resolved
  system_resolved="$(dscacheutil -q host -a name "${probe}" 2>/dev/null | awk '/^ip_address:/ {print $2; exit}')"
  if [ "${system_resolved}" = "127.0.0.1" ]; then
    log_success "system resolver: *.${DOMAIN} → 127.0.0.1"
    ok=1
  else
    log_warn "system resolver did not resolve (${RESOLVER_FILE} may be missing or dnsmasq may be stopped)"
  fi

  [ "${ok}" = "1" ]
}

ensure_dnsmasq() {
  # If it is already running, do nothing (exit without prompting for sudo)
  if dnsmasq_is_active; then
    verify_dns || true
    return 0
  fi

  log_info "Configuring dnsmasq for *.${DOMAIN} wildcard DNS..."

  # 1. Install (if missing)
  if ! command -v dnsmasq >/dev/null 2>&1; then
    log_info "Installing dnsmasq via Homebrew..."
    brew install dnsmasq >/dev/null 2>&1
  fi

  # 2. Write optimus.conf (idempotent)
  mkdir -p "${BREW_PREFIX}/etc/dnsmasq.d"
  cat > "${DNSMASQ_CONF}" <<EOF
# Optimus — wildcard DNS (auto-generated by setup-kind-ingress.sh)
address=/.${DOMAIN}/127.0.0.1
EOF

  # 3. Enable the conf-dir line
  local main_conf="${BREW_PREFIX}/etc/dnsmasq.conf"
  local conf_dir_line="conf-dir=${BREW_PREFIX}/etc/dnsmasq.d/,*.conf"
  if ! grep -q "^conf-dir=.*dnsmasq.d" "${main_conf}" 2>/dev/null; then
    if grep -q "^#conf-dir=.*dnsmasq.d/,\*.conf" "${main_conf}" 2>/dev/null; then
      sed -i '' "s|^#conf-dir=.*dnsmasq.d/,\*.conf|${conf_dir_line}|" "${main_conf}"
    else
      echo "${conf_dir_line}" >> "${main_conf}"
    fi
  fi

  # 4. Steps that require sudo: /etc/resolver + service start + verification
  if sudo_available; then
    if [ ! -f "${RESOLVER_FILE}" ]; then
      sudo mkdir -p /etc/resolver
      printf "nameserver 127.0.0.1\nport 53\n" | sudo tee "${RESOLVER_FILE}" >/dev/null
      log_success "Created ${RESOLVER_FILE}"
    fi

    if ! dnsmasq_is_active; then
      sudo brew services restart dnsmasq >/dev/null 2>&1 \
        || sudo brew services start dnsmasq >/dev/null 2>&1 || true
    fi

    sleep 1
    verify_dns || true
  else
    local one_liner="sudo sh -c 'mkdir -p /etc/resolver && printf \"nameserver 127.0.0.1\\\\nport 53\\\\n\" > /etc/resolver/optimus.local'"
    echo ""
    echo -e "${YELLOW}${BOLD}┌──────────────────────────────────────────────────────────────────┐${NC}"
    echo -e "${YELLOW}${BOLD}│  ONE-TIME SUDO REQUIRED — run this in your terminal             │${NC}"
    echo -e "${YELLOW}${BOLD}└──────────────────────────────────────────────────────────────────┘${NC}"
    echo ""
    printf '  %s%s%s\n' "${BOLD}" "${one_liner}" "${NC}"
    echo -e "  ${BOLD}sudo brew services start dnsmasq${NC}"
    echo ""
    echo -e "  Verification:"
    echo -e "  ${CYAN}ping -c1 anything.${DOMAIN}${NC}   # → 127.0.0.1"
    echo -e "  ${CYAN}mise run ingress:verify${NC}       # full DNS + TLS check"
    echo ""
    echo -e "  ${YELLOW}Note: if these steps are skipped, ${DOMAIN} addresses will not resolve.${NC}"
    echo ""
  fi
}

cmd_up() {
  log_info "Initializing Kind cluster with Ingress support..."
  ensure_prereqs

  # 0. dnsmasq wildcard DNS — no /etc/hosts entry needed
  ensure_dnsmasq

  # 1. Cluster check / create
  if ! kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
    if [ "${OPTIMUS_KIND_CREATING:-0}" = "1" ]; then
      log_error "Kind cluster '${CLUSTER_NAME}' not found during creation."
      exit 1
    fi
    log_info "Kind cluster '${CLUSTER_NAME}' does not exist. Creating via kind-create.sh..."
    OPTIMUS_KIND_CREATING=1 "${ROOT_DIR}/scripts/kind-create.sh"
    return 0
  else
    log_info "Kind cluster '${CLUSTER_NAME}' is already running."
    ensure_kubeconfig
  fi

  ensure_kubeconfig

  # 2. Deploy ingress-nginx for kind if not present
  if ! kubectl get namespace ingress-nginx >/dev/null 2>&1 || \
     ! kubectl get deployment ingress-nginx-controller -n ingress-nginx >/dev/null 2>&1; then
    log_info "Deploying ingress-nginx controller for Kind..."
    kubectl apply -f "${INGRESS_NGINX_DEPLOY_URL}"
  else
    log_info "ingress-nginx deployment already exists."
  fi

  # 3. Patch ingress-nginx to run on control-plane (which has host port 80/443 mapped)
  log_info "Scheduling ingress-nginx controller on control-plane node..."
  kubectl patch deployment ingress-nginx-controller -n ingress-nginx --type=strategic --patch '{
    "spec": {
      "template": {
        "spec": {
          "nodeSelector": {"ingress-ready": "true"},
          "tolerations": [
            {"key": "node-role.kubernetes.io/control-plane", "operator": "Equal", "effect": "NoSchedule"},
            {"key": "node-role.kubernetes.io/master", "operator": "Equal", "effect": "NoSchedule"}
          ]
        }
      }
    }
  }' 2>/dev/null || true

  # 4. Assign the wildcard TLS as the default certificate (fallback for undefined hosts)
  #    Note: Ingresses with a spec.tls block work independently of this setting.
  ensure_default_ssl_cert

  log_info "Waiting for ingress-nginx controller to be Ready on control-plane..."
  kubectl wait --namespace ingress-nginx \
    --for=condition=ready pod \
    --selector=app.kubernetes.io/component=controller \
    --timeout=120s

  # 5. Generate TLS certificates and load secrets
  log_info "Configuring TLS secrets..."
  sync_tls_secret "default"
  sync_tls_secret "optimus"

  log_success "Ingress controller & TLS certificates are ready!"
}

cmd_ingress() {
  local name="${1:-}"
  local namespace="${2:-}"
  local service="${3:-}"
  local port="${4:-}"
  local host="${5:-}"

  if [ -z "${name}" ] || [ -z "${namespace}" ] || [ -z "${service}" ] || [ -z "${port}" ] || [ -z "${host}" ]; then
    log_error "Usage: $0 ingress <name> <namespace> <service> <port> <host>"
    echo "Example: $0 ingress grafana optimus grafana 3000 grafana.optimus.local"
    exit 1
  fi

  ensure_kubeconfig
  sync_tls_secret "${namespace}"

  # Remove any other Ingress requesting the same host (prevents webhook host conflicts)
  local conflicts
  conflicts="$(kubectl get ingress -n "${namespace}" -o json 2>/dev/null \
    | jq -r --arg host "${host}" --arg self "${name}" \
      '.items[] | select(.metadata.name != $self) | select([.spec.rules[]?.host] | index($host)) | .metadata.name' 2>/dev/null || true)"
  if [ -n "${conflicts}" ]; then
    log_warn "Removing conflicting Ingress(es) for host ${host}: ${conflicts}"
    # shellcheck disable=SC2086
    kubectl delete ingress ${conflicts} -n "${namespace}" --ignore-not-found=true >/dev/null
  fi

  log_info "Applying Ingress resource: ${name} (${host} -> ${namespace}/${service}:${port})..."

  cat <<EOF | kubectl apply -f -
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: "${name}"
  namespace: "${namespace}"
  annotations:
    ingress.class: nginx
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
    nginx.ingress.kubernetes.io/proxy-body-size: "64m"
spec:
  ingressClassName: nginx
  tls:
    - hosts:
        - "${host}"
      secretName: "${SECRET_NAME}"
  rules:
    - host: "${host}"
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: "${service}"
                port:
                  number: ${port}
EOF

  update_hosts_file "${host}"
  log_success "Ingress '${name}' ready at: https://${host}"
}

cmd_ingress_all() {
  log_info "Setting up Ingress routes for all Optimus platform tools..."
  ensure_kubeconfig

  # Define services to expose: name namespace service port host
  local services=(
    "grafana:optimus:grafana:3000:grafana.optimus.local"
    "prometheus:optimus:prometheus:9090:prometheus.optimus.local"
    "jaeger:optimus:jaeger:16686:jaeger.optimus.local"
    "redpanda:optimus:redpanda:9644:redpanda.optimus.local"
    "kong-proxy:optimus:kong:8000:api.optimus.local"
    "platform:optimus:platform:8080:platform.optimus.local"
    "decision-service:optimus:decision-service:8082:decision.optimus.local"
  )

  for entry in "${services[@]}"; do
    IFS=':' read -r name ns svc port host <<< "${entry}"
    if kubectl get svc "${svc}" -n "${ns}" >/dev/null 2>&1; then
      cmd_ingress "${name}" "${ns}" "${svc}" "${port}" "${host}"
    else
      log_warn "Service '${svc}' in namespace '${ns}' not found yet. Skipping ingress."
    fi
  done

  echo ""
  log_success "Ingress setup complete! Check your /etc/hosts for the hosts listed above."
}

cmd_down() {
  log_info "Tearing down Kind cluster..."
  if [ -f "${ROOT_DIR}/scripts/kind-destroy.sh" ]; then
    "${ROOT_DIR}/scripts/kind-destroy.sh"
  else
    kind delete cluster --name "${CLUSTER_NAME}"
  fi
  rm -f "${KUBECONFIG_PATH}"
  log_success "Cluster destroyed."
}

cmd_verify() {
  ensure_kubeconfig
  local failures=0

  log_info "1. DNS resolution (*.${DOMAIN})..."
  if verify_dns; then :; else failures=$((failures + 1)); fi

  log_info "2. TLS Ingress endpoints..."
  local entries
  entries="$(kubectl get ingress -n optimus \
    -o jsonpath='{range .items[*]}{.metadata.name}{" "}{.spec.rules[0].host}{"\n"}{end}' 2>/dev/null || true)"

  if [ -z "${entries}" ]; then
    log_warn "No Ingress resources found in namespace 'optimus'."
    failures=$((failures + 1))
  else
    while read -r name host; do
      [ -z "${host}" ] && continue
      local code probe path
      code="$(curl -sk -o /dev/null -w '%{http_code}' \
        --resolve "${host}:443:127.0.0.1" \
        --max-time 5 "https://${host}/" 2>/dev/null || echo "ERR")"

      # API services may return 404 on the root path; get the real signal from /healthz
      path="/"
      if [ "${code}" = "404" ]; then
        local hcode
        hcode="$(curl -sk -o /dev/null -w '%{http_code}' \
          --resolve "${host}:443:127.0.0.1" \
          --max-time 5 "https://${host}/healthz" 2>/dev/null || echo "ERR")"
        if [ "${hcode}" = "200" ]; then
          code="${hcode}"
          path="/healthz"
        fi
      fi

      if [ "${code}" = "ERR" ] || { [ "${code}" -ge 500 ] 2>/dev/null; }; then
        printf "  ${RED}✗${NC} %-28s %s %s\n" "${host}" "${code}" "${path}"
        failures=$((failures + 1))
      else
        printf "  ${GREEN}✓${NC} %-28s HTTP %s %s\n" "${host}" "${code}" "${path}"
      fi
    done <<< "${entries}"
  fi

  echo ""
  if [ "${failures}" -eq 0 ]; then
    log_success "All checks passed."
  else
    log_warn "${failures} check(s) failed — see troubleshooting in docs/dev-environment.md"
    return 1
  fi
}

cmd_certs() {
  local force="${1:-}"

  if [ "${force}" = "--force" ] || [ "${force}" = "-f" ]; then
    log_info "Forcing certificate regeneration..."
    rm -f "${CERT_FILE}" "${KEY_FILE}"
  fi

  ensure_certs

  echo ""
  log_info "Certificate: ${CERT_FILE}"
  openssl x509 -noout -subject -issuer -dates -ext subjectAltName -in "${CERT_FILE}" 2>/dev/null \
    | sed 's/^ *//; s/^/  /'

  # Update the secrets so the new certificate takes effect immediately
  if ensure_kubeconfig && kubectl get namespace ingress-nginx >/dev/null 2>&1; then
    echo ""
    log_info "Syncing TLS secret into cluster namespaces..."
    for ns in default optimus; do
      if kubectl get namespace "${ns}" >/dev/null 2>&1; then
        sync_tls_secret "${ns}"
        log_success "Secret '${SECRET_NAME}' updated in namespace '${ns}'"
      fi
    done

    # Trigger ingress-nginx to load the new certificate
    ensure_default_ssl_cert
    kubectl rollout restart deployment/ingress-nginx-controller -n ingress-nginx >/dev/null 2>&1 || true
    kubectl rollout status deployment/ingress-nginx-controller -n ingress-nginx --timeout=120s >/dev/null 2>&1 || true
    log_success "ingress-nginx reloaded."
  fi

  echo ""
  log_info "Serving certificate check (via https://${DOMAIN}):"
  echo | openssl s_client -servername "probe.${DOMAIN}" -connect 127.0.0.1:443 2>/dev/null \
    | openssl x509 -noout -subject -issuer -dates 2>/dev/null \
    | sed 's/^ *//; s/^/  /' || log_warn "Could not read served certificate (is the cluster up?)"
}

case "${1:-}" in
  up)
    cmd_up
    ;;
  ingress)
    shift
    cmd_ingress "$@"
    ;;
  ingress-all|all)
    cmd_ingress_all
    ;;
  dns|dnsmasq)
    ensure_dnsmasq
    ;;
  certs)
    shift
    cmd_certs "$@"
    ;;
  verify)
    cmd_verify
    ;;
  down)
    cmd_down
    ;;
  *)
    echo "Usage: $0 {up|ingress|ingress-all|dns|certs|verify|down}"
    echo ""
    echo "Commands:"
    echo "  up                                              Bootstrap cluster, ingress-nginx, TLS certs & DNS"
    echo "  ingress <name> <ns> <svc> <port> <host>         Create TLS Ingress for a specific service"
    echo "  ingress-all                                     Create TLS Ingresses for all deployed services"
    echo "  dns                                             Setup dnsmasq wildcard DNS (*.${DOMAIN})"
    echo "  certs [--force]                                 Issue/renew the wildcard cert and sync secrets"
    echo "  verify                                          Verify TLS Ingress + DNS resolution"
    echo "  down                                            Delete Kind cluster"
    exit 1
    ;;
esac
