#!/usr/bin/env bash
set -euo pipefail

CLUSTER="${CLUSTER:-aether-router-test}"
NS="${NS:-aether-router-test}"
IMAGE="${IMAGE:-aethercode-router:k3d-test}"
HOST_PORT="${HOST_PORT:-18080}"
ADMIN_KEY="${ADMIN_KEY:-admin-key}"
PUBLIC_KEY="${PUBLIC_KEY:-public-key}"
KEEP_K3D="${KEEP_K3D:-0}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
AETHER_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
PROJECTS_DIR="$(cd "$AETHER_DIR/.." && pwd)"
TMP_DIR="$(mktemp -d)"
PIDS=()

cleanup() {
  for pid in "${PIDS[@]:-}"; do
    kill "$pid" >/dev/null 2>&1 || true
  done
  rm -rf "$TMP_DIR"
  if [[ "$KEEP_K3D" != "1" ]]; then
    k3d cluster delete "$CLUSTER" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

require() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "missing required command: $1" >&2
    exit 1
  }
}

require docker
require k3d
require kubectl
require curl
require python3

echo "building $IMAGE"
docker build -f "$AETHER_DIR/router/Dockerfile" -t "$IMAGE" "$PROJECTS_DIR"

echo "creating k3d cluster $CLUSTER"
k3d cluster delete "$CLUSTER" >/dev/null 2>&1 || true
k3d cluster create "$CLUSTER" --agents 1

echo "importing image"
k3d image import "$IMAGE" -c "$CLUSTER"

echo "deploying test stack"
kubectl apply -f "$AETHER_DIR/router/deploy/k3d-test.yaml"
kubectl -n "$NS" rollout status deployment/postgres --timeout=180s
kubectl -n "$NS" rollout status deployment/mock-provider --timeout=180s
kubectl -n "$NS" rollout status deployment/router --timeout=180s

echo "forwarding router service to localhost:${HOST_PORT}"
kubectl -n "$NS" port-forward svc/router "${HOST_PORT}:80" >"$TMP_DIR/pf-router-service.log" 2>&1 &
router_service_pf_pid="$!"
PIDS+=("$router_service_pf_pid")

echo "waiting for router service"
for _ in $(seq 1 120); do
  if curl -fsS "http://127.0.0.1:${HOST_PORT}/healthz" >/dev/null; then
    break
  fi
  if ! kill -0 "$router_service_pf_pid" >/dev/null 2>&1; then
    echo "router service port-forward exited early" >&2
    cat "$TMP_DIR/pf-router-service.log" >&2 || true
    exit 1
  fi
  sleep 1
done
curl -fsS "http://127.0.0.1:${HOST_PORT}/healthz" >/dev/null

echo "creating provider config through load-balanced router service"
CREATE_BODY='{
  "name": "k3d-mock-provider",
  "provider": "openai",
  "base_url": "http://mock-provider.aether-router-test.svc.cluster.local/v1",
  "api_key": "upstream-key",
  "models": "k3d-public-model",
  "groups": "default",
  "model_mapping": {"k3d-public-model": "k3d-upstream-model"},
  "headers": {"X-Aether-K3D-Test": "true"},
  "status": 1,
  "priority": 100,
  "weight": 100
}'
create_response="$TMP_DIR/create-provider-response.txt"
if ! curl -fsS -o "$create_response" -X POST "http://127.0.0.1:${HOST_PORT}/internal/providers" \
  -H "Authorization: Bearer ${ADMIN_KEY}" \
  -H "Content-Type: application/json" \
  -d "$CREATE_BODY"; then
  echo "provider create failed:" >&2
  cat "$create_response" >&2 || true
  kubectl -n "$NS" logs deployment/router --tail=200 >&2 || true
  exit 1
fi

pods="$(kubectl -n "$NS" get pods -l app=router -o jsonpath='{.items[*].metadata.name}')"
if [[ -z "$pods" ]]; then
  echo "no router pods found" >&2
  exit 1
fi

start_port_forward() {
  local pod="$1"
  local port="$2"
  local log_file="$TMP_DIR/pf-${pod}.log"
  kubectl -n "$NS" port-forward "pod/${pod}" "${port}:8080" >"$log_file" 2>&1 &
  local pid="$!"
  PIDS+=("$pid")
  for _ in $(seq 1 60); do
    if curl -fsS "http://127.0.0.1:${port}/healthz" >/dev/null 2>&1; then
      return 0
    fi
    if ! kill -0 "$pid" >/dev/null 2>&1; then
      echo "port-forward for $pod exited early" >&2
      cat "$log_file" >&2 || true
      return 1
    fi
    sleep 0.5
  done
  echo "port-forward for $pod did not become ready" >&2
  cat "$log_file" >&2 || true
  return 1
}

assert_status_synced() {
  local pod="$1"
  local port="$2"
  local status_json="$3"
  STATUS_JSON="$status_json" POD="$pod" python3 - <<'PY'
import json
import os
import sys

data = json.loads(os.environ["STATUS_JSON"])
pod = os.environ["POD"]
cache = data["cache"]
db_version = data["database"]["provider_version"]
if data["instance_id"] != pod:
    print(f"instance mismatch: expected {pod}, got {data['instance_id']}", file=sys.stderr)
    sys.exit(1)
if not data["in_sync"]:
    print(f"pod {pod} not in sync: {data}", file=sys.stderr)
    sys.exit(1)
if cache["version"] != db_version:
    print(f"cache/db version mismatch: {data}", file=sys.stderr)
    sys.exit(1)
if cache["provider_count"] != 1 or cache["enabled_provider_count"] != 1:
    print(f"unexpected provider counts: {data}", file=sys.stderr)
    sys.exit(1)
PY
}

assert_chat_response() {
  local pod="$1"
  local port="$2"
  local headers="$TMP_DIR/headers-${pod}.txt"
  local body="$TMP_DIR/body-${pod}.json"
  curl -fsS -D "$headers" -o "$body" \
    -X POST "http://127.0.0.1:${port}/v1/chat/completions" \
    -H "Authorization: Bearer ${PUBLIC_KEY}" \
    -H "Content-Type: application/json" \
    -d '{"model":"k3d-public-model","messages":[{"role":"user","content":"ping"}]}' >/dev/null

  tr -d '\r' <"$headers" >"${headers}.clean"
  if ! grep -qi "^X-Aether-Router-Instance: ${pod}$" "${headers}.clean"; then
    echo "missing router instance header for $pod" >&2
    cat "${headers}.clean" >&2
    exit 1
  fi
  if ! grep -qi "^X-Aether-Provider-Id: 1$" "${headers}.clean"; then
    echo "missing provider id header for $pod" >&2
    cat "${headers}.clean" >&2
    exit 1
  fi
  if ! grep -qi "^X-Aether-Provider-Version: 2$" "${headers}.clean"; then
    echo "missing provider version header for $pod" >&2
    cat "${headers}.clean" >&2
    exit 1
  fi
  if ! grep -q "k3d-upstream-model" "$body"; then
    echo "mapped upstream model not found in response for $pod" >&2
    cat "$body" >&2
    exit 1
  fi
}

assert_stream_response() {
  local port="$1"
  local body="$TMP_DIR/stream.txt"
  curl -fsS -N \
    -X POST "http://127.0.0.1:${port}/v1/chat/completions" \
    -H "Authorization: Bearer ${PUBLIC_KEY}" \
    -H "Content-Type: application/json" \
    -d '{"model":"k3d-public-model","stream":true,"messages":[{"role":"user","content":"stream"}]}' >"$body"
  grep -q "data: " "$body"
  grep -q "\\[DONE\\]" "$body"
}

echo "verifying every router pod has the same provider config and can route"
idx=0
first_port=""
for pod in $pods; do
  port=$((19080 + idx))
  idx=$((idx + 1))
  start_port_forward "$pod" "$port"
  first_port="${first_port:-$port}"

  synced=""
  for _ in $(seq 1 60); do
    status_json="$(curl -fsS "http://127.0.0.1:${port}/internal/status" -H "Authorization: Bearer ${ADMIN_KEY}")"
    if STATUS_JSON="$status_json" python3 - <<'PY'
import json
import os
data = json.loads(os.environ["STATUS_JSON"])
raise SystemExit(0 if data.get("in_sync") and data.get("cache", {}).get("provider_count") == 1 else 1)
PY
    then
      synced="$status_json"
      break
    fi
    sleep 1
  done
  if [[ -z "$synced" ]]; then
    echo "pod $pod did not sync provider config" >&2
    kubectl -n "$NS" logs "pod/$pod" --tail=100 >&2 || true
    exit 1
  fi

  assert_status_synced "$pod" "$port" "$synced"
  assert_chat_response "$pod" "$port"
done

echo "verifying stream passthrough"
assert_stream_response "$first_port"

echo "k3d distributed router test passed"
