#!/usr/bin/env bash
#
# coscli-auth.sh - run coscli using temporary credentials read from tccli.
#
# Usage:
#   ./coscli-auth.sh ls
#   ./coscli-auth.sh ls cos://qibaotu-1313190940
#   COS_REGION=ap-guangzhou ./coscli-auth.sh ls
#   ./coscli-auth.sh cp ./local.txt cos://bucket-appid/path/
#
# Env: COS_REGION (default ap-shanghai),
#      TCCLI_VENV (default ~/.tccli/venv),
#      COSCLI_BIN (default $TCCLI_VENV/bin/coscli),
#      TCCLI_CMD (default $TCCLI_VENV/bin/tccli),
#      PYTHON_BIN (default $TCCLI_VENV/bin/python),
#      TCCLI_PROFILE (default default),
#      TCCLI_CRED (default ~/.tccli/<profile>.credential)
#
set -euo pipefail

TCCLI_VENV="${TCCLI_VENV:-$HOME/.tccli/venv}"
PROFILE="${TCCLI_PROFILE:-default}"
CRED="${TCCLI_CRED:-$HOME/.tccli/${PROFILE}.credential}"
REGION="${COS_REGION:-ap-shanghai}"
COSCLI_BIN="${COSCLI_BIN:-$TCCLI_VENV/bin/coscli}"
TCCLI_CMD="${TCCLI_CMD:-$TCCLI_VENV/bin/tccli}"
PYTHON_BIN="${PYTHON_BIN:-$TCCLI_VENV/bin/python}"
read -r -a TCCLI_WORDS <<< "$TCCLI_CMD"

die() { echo "[coscli-auth] $*" >&2; exit 1; }

for c in "$COSCLI_BIN" "${TCCLI_WORDS[0]}" "$PYTHON_BIN"; do
  command -v "$c" >/dev/null 2>&1 || die "$c not found; run: source $TCCLI_VENV/bin/activate, or see references/environment-setup.md"
done

[ "$#" -gt 0 ] || die "usage: $0 <coscli-subcommand> [args...]"

# tccli temp keys refresh lazily: a tccli call renews and rewrites the cred file.
"${TCCLI_WORDS[@]}" sts GetCallerIdentity >/dev/null 2>&1 || die "tccli auth failed, re-login required"
[ -f "$CRED" ] || die "credential file not found: $CRED"

# Read creds and validate expiry in one python pass; emit shell-safe assignments.
eval "$("$PYTHON_BIN" - "$CRED" <<'PY'
import json, sys, time, shlex
d = json.load(open(sys.argv[1]))
sid, skey, tok = d.get("secretId",""), d.get("secretKey",""), d.get("token","")
if not sid or not skey:
    print("echo '[coscli-auth] invalid credential: missing secretId/secretKey' >&2; exit 1")
    sys.exit(0)
exp = d.get("expiresAt")
if exp and float(exp) < time.time():
    print("echo '[coscli-auth] warning: temporary credential expired, re-login tccli' >&2")
print(f"SID={shlex.quote(sid)}; SKEY={shlex.quote(skey)}; TOKEN={shlex.quote(tok)}")
PY
)"

ARGS=(--init-skip -e "cos.${REGION}.myqcloud.com" -i "$SID" -k "$SKEY")
[ -n "$TOKEN" ] && ARGS+=(--token "$TOKEN")

exec "$COSCLI_BIN" "${ARGS[@]}" "$@"
