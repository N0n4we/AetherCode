# AetherCode Tencent Cloud Runbook

This runbook describes how to rebuild, update, verify, and operate the current
AetherCode deployment on Tencent Cloud TKE.

## Scope

IaC lives under:

- Terraform: `platform/terraform/tencentcloud`
- Kubernetes: `platform/k8s/tke`
- Router application source and image build input: `router`

Terraform manages Tencent Cloud infrastructure. Kubernetes manifests manage the
runtime resources inside TKE. Application data, generated API keys, and raw
secrets are intentionally not stored in this repository.

Current public endpoints:

- Relay: `http://lb-74kbv10l-5xczlkyth7osdqr8.clb.sh-tencentclb.com`
- OpenAI-compatible base URL:
  `http://lb-74kbv10l-5xczlkyth7osdqr8.clb.sh-tencentclb.com/v1`
- Account service:
  `http://lb-l5vk4kbl-i1jarzn7ckm3sf2o.clb.sh-tencentclb.com`

Current Tencent Cloud resources managed by Terraform include VPC, subnets, NAT,
EIP, default route, TKE cluster, node pool, public Kubernetes endpoint,
security groups, CAM Auto Scaling role attachments, and the node-pool SSH key.

Known non-Terraform resources:

- TCR Personal repository
  `ccr.ccs.tencentyun.com/aethercode-100034871923/router`. TencentCloud
  Terraform provider coverage is for TCR Enterprise, so the Personal repository
  is documented but not managed by Terraform.
- CLBs behind `relay-public` and `account-public`. These are created by the TKE
  cloud controller from Kubernetes `Service type=LoadBalancer`.
- Kubernetes workloads, Services, Secrets, ConfigMaps, Jobs, and CronJobs. These
  are managed by `platform/k8s/tke`.

## Prerequisites

Local tools:

- `tenv` with Terraform available
- `tccli`, authenticated with the default profile
- `kubectl`
- `docker`
- `jq`
- `python3`

Tencent Cloud assumptions:

- Region: `ap-shanghai`
- TKE cluster: `cls-26zqizrl`
- TAT-capable node for fallback operations: `ins-3k2t5zjf`
- Local Terraform credentials come from `~/.tccli`

Do not commit:

- `platform/k8s/tke/secrets.env`
- generated relay API keys
- OpenRouter keys
- kubeconfig files
- any TCR login credentials

## 1. Check Terraform Drift

Run from the repository root:

```sh
cd /Users/noname/Desktop/Projects/AetherCode/platform/terraform/tencentcloud

TENCENTCLOUD_SHARED_CREDENTIALS_DIR="$HOME/.tccli" \
TENCENTCLOUD_PROFILE=default \
terraform init

TENCENTCLOUD_SHARED_CREDENTIALS_DIR="$HOME/.tccli" \
TENCENTCLOUD_PROFILE=default \
terraform plan -input=false -detailed-exitcode
```

Interpretation:

- Exit code `0`: Terraform-managed infrastructure matches Tencent Cloud.
- Exit code `2`: Terraform sees changes. Review carefully before applying.
- Exit code `1`: Terraform or provider error.

List managed resources:

```sh
terraform state list
```

The current expected clean result is:

```text
No changes. Your infrastructure matches the configuration.
```

## 2. Apply Terraform Infrastructure

Only apply after reviewing the plan:

```sh
cd /Users/noname/Desktop/Projects/AetherCode/platform/terraform/tencentcloud

TENCENTCLOUD_SHARED_CREDENTIALS_DIR="$HOME/.tccli" \
TENCENTCLOUD_PROFILE=default \
terraform apply -input=false
```

Useful outputs:

```sh
terraform output
```

Important variables:

- `kube_api_allowed_cidrs`: CIDRs allowed to reach the public Kubernetes API.
  Current default is `188.253.117.150/32`.
- `node_pool_max_size`: max autoscaling size for the TKE node pool. Current
  default is `2`; set to `0` only when intentionally parking the environment.

## 3. Build And Push Router Image

The Dockerfile expects the build context to contain both `AetherCode` and the
sibling `gollm` checkout. Build from `/Users/noname/Desktop/Projects`:

```sh
cd /Users/noname/Desktop/Projects

TAG="$(date +%Y%m%d-%H%M%S)"
IMAGE="ccr.ccs.tencentyun.com/aethercode-100034871923/router:${TAG}"

docker build -f AetherCode/router/Dockerfile -t "$IMAGE" .
docker push "$IMAGE"
```

Then update `platform/k8s/tke/kustomization.yaml`:

```yaml
images:
  - name: ghcr.io/aethercode/router
    newName: ccr.ccs.tencentyun.com/aethercode-100034871923/router
    newTag: <TAG>
```

Current deployed image:

```text
ccr.ccs.tencentyun.com/aethercode-100034871923/router:20260703-183944
```

## 4. Prepare Kubernetes Secrets

Create the local secret file:

```sh
cd /Users/noname/Desktop/Projects/AetherCode
cp platform/k8s/tke/secrets.env.example platform/k8s/tke/secrets.env
```

Fill these values:

```text
sql-dsn=postgres://router:<url-encoded-postgres-password>@postgres.aether-relay.svc.cluster.local:5432/router?sslmode=disable
postgres-password=<raw-postgres-password>
router-api-key=<optional-static-public-relay-key>
router-admin-key=<admin-key-for-internal-provider-apis>
api-key-hash-secret=<random-hmac-secret-for-relay-api-keys>
account-service-key=<bearer-key-for-account-service>
openrouter-api-key=<openrouter-key>
```

Use the raw password for `postgres-password`. URL-encode the same password in
`sql-dsn`, especially if it contains `/`, `+`, `=`, `%`, `@`, or other reserved
characters.

Example URL-encoding helper:

```sh
python3 - <<'PY'
import getpass
import urllib.parse

print(urllib.parse.quote(getpass.getpass("Postgres password: "), safe=""))
PY
```

Generate random secrets when needed:

```sh
openssl rand -base64 32
```

## 5. Get Kubernetes Access

Normal path:

```sh
tccli tke DescribeClusterKubeconfig \
  --region ap-shanghai \
  --ClusterId cls-26zqizrl \
  --IsExtranet true \
  --output json | jq -r '.Kubeconfig' > /tmp/aether-kubeconfig

export KUBECONFIG=/tmp/aether-kubeconfig
kubectl get nodes
```

If local Kubernetes API access fails with TLS EOF or network issues, run
`kubectl` from a cluster node through Tencent Cloud TAT. The node already has
`/root/.kube/config`:

```sh
CONTENT="$(printf '%s' 'set -eu
export KUBECONFIG=/root/.kube/config
kubectl get nodes
kubectl -n aether-relay get all -o wide
' | base64 | tr -d '\n')"

tccli tat RunCommand \
  --cli-unfold-argument \
  --region ap-shanghai \
  --CommandType SHELL \
  --CommandName aether-kubectl-check \
  --Content "$CONTENT" \
  --InstanceIds ins-3k2t5zjf \
  --Timeout 120 \
  --Username root \
  --output json
```

Poll TAT output:

```sh
tccli tat DescribeInvocationTasks \
  --cli-unfold-argument \
  --region ap-shanghai \
  --Offset 0 \
  --Limit 10 \
  --HideOutput False \
  --Filters.0.Name invocation-id \
  --Filters.0.Values <invocation-id> \
  --output json |
jq -r '.InvocationTaskSet[] |
  "status=" + .TaskStatus + " exit=" + ((.TaskResult.ExitCode // -999)|tostring) +
  "\n" + ((.TaskResult.Output // "") | @base64d)'
```

## 6. Deploy Kubernetes Runtime

Apply the TKE overlay:

```sh
cd /Users/noname/Desktop/Projects/AetherCode
kubectl apply -k platform/k8s/tke
```

Wait for workloads:

```sh
kubectl -n aether-relay rollout status statefulset/postgres --timeout=300s
kubectl -n aether-relay rollout status deployment/relay --timeout=300s
kubectl -n aether-relay rollout status deployment/account --timeout=300s
kubectl -n aether-relay wait \
  --for=condition=complete \
  job/sync-openrouter-provider-channels-once \
  --timeout=300s
```

Check Services:

```sh
kubectl -n aether-relay get svc relay-public account-public
```

Runtime resources created by the overlay:

- Namespace `aether-relay`
- Postgres StatefulSet and Service
- Relay Deployment and Service
- Account Deployment and Service
- Public Tencent Cloud LoadBalancer Services
- Runtime ConfigMap
- Runtime Secret generated from `secrets.env`
- OpenRouter provider-channel sync ConfigMap, one-shot Job, and CronJob

## 7. OpenRouter Provider Channel Drift Sync

`platform/k8s/tke/openrouter-seed.yaml` now reconciles provider channels
declaratively.

Managed channels:

- `poolside/laguna-xs-2.1:free`
- `cohere/north-mini-code:free`

The one-shot Job initializes a fresh deployment:

```text
sync-openrouter-provider-channels-once
```

The CronJob corrects drift every 15 minutes:

```text
sync-openrouter-provider-channels
```

It updates managed fields such as provider name, public model ID, upstream base
URL, upstream model, secret reference, auth header/prefix, priority, weight,
status, endpoint capabilities, channel type, relay format, and billing class.

It intentionally does not delete unmanaged provider channels.

Run an immediate manual sync:

```sh
kubectl -n aether-relay create job \
  --from=cronjob/sync-openrouter-provider-channels \
  "sync-openrouter-provider-channels-manual-$(date +%Y%m%d%H%M%S)"
```

Check sync logs:

```sh
kubectl -n aether-relay logs job/sync-openrouter-provider-channels-once
kubectl -n aether-relay get cronjob sync-openrouter-provider-channels
```

## 8. Verify Health

Get current public endpoints from Kubernetes:

```sh
kubectl -n aether-relay get svc relay-public account-public
```

Health checks:

```sh
RELAY_URL="http://lb-74kbv10l-5xczlkyth7osdqr8.clb.sh-tencentclb.com"
ACCOUNT_URL="http://lb-l5vk4kbl-i1jarzn7ckm3sf2o.clb.sh-tencentclb.com"

curl --noproxy '*' -fsS "$RELAY_URL/healthz"
curl --noproxy '*' -fsS "$ACCOUNT_URL/healthz"
```

If local DNS returns a fake IP, resolve the CLB domain explicitly:

```sh
curl --noproxy '*' \
  --resolve "lb-74kbv10l-5xczlkyth7osdqr8.clb.sh-tencentclb.com:80:1.15.197.38" \
  -fsS "$RELAY_URL/healthz"
```

Check internal relay status:

```sh
kubectl -n aether-relay port-forward svc/relay 18080:80

curl -fsS \
  -H "Authorization: Bearer $ROUTER_ADMIN_KEY" \
  http://127.0.0.1:18080/internal/status | jq .
```

Expected:

- `in_sync: true`
- provider count `2`
- model count `2`

## 9. Generate A Relay API Key

The account service creates account-scoped relay API keys. It requires
`ACCOUNT_SERVICE_KEY` and an account identity header.

```sh
ACCOUNT_URL="http://lb-l5vk4kbl-i1jarzn7ckm3sf2o.clb.sh-tencentclb.com"

curl --noproxy '*' -fsS \
  -X POST "$ACCOUNT_URL/account/api-keys" \
  -H "Authorization: Bearer $ACCOUNT_SERVICE_KEY" \
  -H "X-Aether-Account-ID: default" \
  -H "Content-Type: application/json" \
  -d '{"name":"smoke-test"}' | jq .
```

The response includes `secret` once. Store it in a local shell variable for
testing, but do not commit it:

```sh
RELAY_API_KEY="<secret-from-response>"
```

Disable or revoke keys when they are no longer needed:

```sh
curl --noproxy '*' -fsS \
  -X POST "$ACCOUNT_URL/account/api-keys/<id>/disable" \
  -H "Authorization: Bearer $ACCOUNT_SERVICE_KEY" \
  -H "X-Aether-Account-ID: default"

curl --noproxy '*' -fsS \
  -X POST "$ACCOUNT_URL/account/api-keys/<id>/revoke" \
  -H "Authorization: Bearer $ACCOUNT_SERVICE_KEY" \
  -H "X-Aether-Account-ID: default"
```

## 10. OpenAI-Compatible Smoke Test

Use the relay public endpoint:

```sh
RELAY_URL="http://lb-74kbv10l-5xczlkyth7osdqr8.clb.sh-tencentclb.com"

curl --noproxy '*' -fsS \
  "$RELAY_URL/v1/chat/completions" \
  -H "Authorization: Bearer $RELAY_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "cohere/north-mini-code:free",
    "messages": [
      {"role": "user", "content": "Output only this word: pong"}
    ],
    "max_tokens": 100
  }' | jq '{id, model, provider, finish_reason: .choices[0].finish_reason, message: .choices[0].message}'
```

Expected:

```json
{
  "finish_reason": "stop",
  "message": {
    "role": "assistant",
    "content": "pong"
  }
}
```

Provider metadata headers should include:

- `X-Aether-Provider-Id`
- `X-Aether-Provider-Name`
- `X-Aether-Provider-Version`
- `X-Aether-Router-Instance`

## 11. Troubleshooting

### Terraform Plan Wants To Replace NAT

Do not re-add `subnet_id` to `tencentcloud_nat_gateway.aether`. The current
state has `subnet_id = null`; adding it forces replacement.

### TKE CNI Cannot Allocate Pod IPs

If pods fail with a message similar to:

```text
zone ap-shanghai-5 does not exist in allocator
```

Confirm that the `ap-shanghai-5` pod ENI subnet is managed and included in the
cluster:

```sh
terraform state show tencentcloud_subnet.aether_pods_az5
terraform state show tencentcloud_kubernetes_cluster.aether
```

Expected subnet:

```text
subnet-5r7klon1
10.0.2.0/24
ap-shanghai-5
```

### Postgres Login Fails

Check `platform/k8s/tke/secrets.env`.

The most common cause is an unescaped password in `sql-dsn`. Keep
`postgres-password` raw, but URL-encode the password part inside `sql-dsn`.

### OpenRouter Returns 401 Missing Authentication Header

Check provider-channel config:

```sh
kubectl -n aether-relay port-forward svc/relay 18080:80

curl -fsS \
  -H "Authorization: Bearer $ROUTER_ADMIN_KEY" \
  http://127.0.0.1:18080/internal/provider-channels | jq .
```

Expected fields:

```json
{
  "upstream_api_key_secret_ref": "env:OPENROUTER_API_KEY",
  "auth_header": "Authorization",
  "auth_prefix": ""
}
```

The empty `auth_prefix` is intentional. Router code supplies the default
`Bearer ` prefix for the `Authorization` header.

Also verify the Kubernetes Secret has the key without printing it:

```sh
kubectl -n aether-relay get secret relay-runtime-secrets \
  -o jsonpath='{.data.openrouter-api-key}' | wc -c
```

### Local kubectl Fails But Cluster Is Running

Use the TAT fallback in the "Get Kubernetes Access" section and run commands
with:

```sh
export KUBECONFIG=/root/.kube/config
```

### Need Logs

```sh
kubectl -n aether-relay logs deployment/relay --tail=100
kubectl -n aether-relay logs deployment/account --tail=100
kubectl -n aether-relay logs statefulset/postgres --tail=100
kubectl -n aether-relay describe pod <pod-name>
```

## 12. Rollout And Rollback

Update image tag in `platform/k8s/tke/kustomization.yaml`, then:

```sh
kubectl apply -k platform/k8s/tke
kubectl -n aether-relay rollout status deployment/relay --timeout=300s
kubectl -n aether-relay rollout status deployment/account --timeout=300s
```

Rollback to the previous Deployment revision:

```sh
kubectl -n aether-relay rollout undo deployment/relay
kubectl -n aether-relay rollout undo deployment/account
```

Or roll back by restoring the previous `newTag` in
`platform/k8s/tke/kustomization.yaml` and applying the overlay again.

## 13. Secret Rotation

Rotate OpenRouter key:

1. Update `openrouter-api-key` in local `platform/k8s/tke/secrets.env`.
2. Run `kubectl apply -k platform/k8s/tke`.
3. Restart relay pods so env vars reload:

```sh
kubectl -n aether-relay rollout restart deployment/relay
kubectl -n aether-relay rollout status deployment/relay --timeout=300s
```

Rotate `ACCOUNT_SERVICE_KEY`, `ROUTER_ADMIN_KEY`, or `API_KEY_HASH_SECRET`
with care:

- `ACCOUNT_SERVICE_KEY` affects account API access.
- `ROUTER_ADMIN_KEY` affects internal provider admin APIs and sync jobs.
- `API_KEY_HASH_SECRET` affects validation of generated relay API keys. Rotating
  it invalidates existing API key hashes unless keys are reissued.

## 14. Parking Or Restoring Capacity

To park the TKE node pool:

```sh
cd /Users/noname/Desktop/Projects/AetherCode/platform/terraform/tencentcloud

TENCENTCLOUD_SHARED_CREDENTIALS_DIR="$HOME/.tccli" \
TENCENTCLOUD_PROFILE=default \
terraform apply -input=false -var='node_pool_max_size=0'
```

To restore capacity:

```sh
TENCENTCLOUD_SHARED_CREDENTIALS_DIR="$HOME/.tccli" \
TENCENTCLOUD_PROFILE=default \
terraform apply -input=false -var='node_pool_max_size=2'
```

Verify:

```sh
kubectl get nodes
kubectl -n aether-relay get pods -o wide
```

## 15. Final Verification Checklist

Before considering the deployment healthy:

- `terraform plan -detailed-exitcode` returns `0`.
- `kubectl -n aether-relay get pods` shows Postgres, relay, and account pods
  running.
- `relay-public` and `account-public` have public CLB hostnames.
- `GET /healthz` succeeds for relay and account public URLs.
- `sync-openrouter-provider-channels-once` completed.
- `sync-openrouter-provider-channels` CronJob exists and is not suspended.
- `/internal/status` reports `in_sync: true`.
- A generated relay API key can call `/v1/chat/completions`.
- Temporary smoke-test API keys are disabled or revoked after use.
