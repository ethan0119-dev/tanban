#!/usr/bin/env bash
set -Eeuo pipefail

PROJECT_DIR="${PROJECT_DIR:-/root/works/tanban}"
WEBSITE_SERVICE="${WEBSITE_SERVICE:-tanban-website.service}"
WEBSITE_READY_URL="${WEBSITE_READY_URL:-http://127.0.0.1:18100/}"
WEBSITE_PUBLIC_URL="${WEBSITE_PUBLIC_URL:-https://tb.666qwe.cn}"
WEBSITE_READY_TIMEOUT="${WEBSITE_READY_TIMEOUT:-60}"
DEPLOY_LOCK_FILE="${DEPLOY_LOCK_FILE:-/var/lock/tanban-website-deploy.lock}"
NODE_RUNTIME_BIN_DIR="${NODE_RUNTIME_BIN_DIR:-/opt/node-v22/bin}"
WEBSITE_BACKUP_ROOT="${WEBSITE_BACKUP_ROOT:-/var/backups/tanban/website}"

WEBSITE_RELEASE_ID="$(date -u +%Y%m%dT%H%M%SZ)-$$"
WEBSITE_BACKUP_DIR="$WEBSITE_BACKUP_ROOT/$WEBSITE_RELEASE_ID"
WEBSITE_DIST_MUTATED=0
WEBSITE_HAD_PRIOR_DIST=0
WEBSITE_DEPLOY_COMMITTED=0

WEBSITE_CRITICAL_ASSET_PATHS=(
  "/og.png"
  "/website/hero-devices.png"
  "/website/scan-ordering.png"
  "/website/cashier-counter.png"
  "/website/kitchen-printer.png"
  "/website/scene-breakfast.png"
  "/website/scene-coffee-truck.png"
  "/website/scene-bakery.png"
  "/website/scene-night-market.png"
  "/website/scene-cafe.png"
)

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "required command not found: $1" >&2
    exit 1
  fi
}

read_env_value() {
  local key="$1" value
  value="$(sed -n "s/^[[:space:]]*${key}[[:space:]]*=[[:space:]]*//p" .env.production | tail -n 1)"
  value="${value%$'\r'}"
  if [[ "$value" == \"*\" && "$value" == *\" ]]; then
    value="${value:1:${#value}-2}"
  elif [[ "$value" == \'*\' && "$value" == *\' ]]; then
    value="${value:1:${#value}-2}"
  fi
  printf '%s' "$value"
}

check_website_asset() {
  local base_url="$1" asset_path="$2" content_type
  content_type="$(curl --fail --silent --show-error --max-time 10 \
    --output /dev/null --write-out '%{content_type}' "${base_url%/}${asset_path}")"
  if [[ "$content_type" != image/* ]]; then
    echo "website asset returned an unexpected content type: ${base_url%/}${asset_path} ($content_type)" >&2
    return 1
  fi
}

restore_previous_website_dist() {
  if ((WEBSITE_HAD_PRIOR_DIST == 0)); then
    echo "no prior website dist is available for automatic rollback" >&2
    return
  fi
  echo "restoring previous website build from $WEBSITE_BACKUP_DIR" >&2
  install -d -m 0755 "$PROJECT_DIR/dist"
  rsync -a --delete "$WEBSITE_BACKUP_DIR/" "$PROJECT_DIR/dist/"
  systemctl restart "$WEBSITE_SERVICE"
}

on_exit() {
  local status=$?
  set +e
  if ((status != 0)) && ((WEBSITE_DEPLOY_COMMITTED == 0)) && ((WEBSITE_DIST_MUTATED == 1)); then
    restore_previous_website_dist
  fi
  exit "$status"
}
trap on_exit EXIT

if [[ "$NODE_RUNTIME_BIN_DIR" != /* || "$NODE_RUNTIME_BIN_DIR" == "/" || "$NODE_RUNTIME_BIN_DIR" == *".."* ]]; then
  echo "NODE_RUNTIME_BIN_DIR must be a specific absolute path: $NODE_RUNTIME_BIN_DIR" >&2
  exit 1
fi
if [[ ! -x "$NODE_RUNTIME_BIN_DIR/node" || ! -x "$NODE_RUNTIME_BIN_DIR/npm" ]]; then
  echo "missing isolated Node.js runtime in $NODE_RUNTIME_BIN_DIR" >&2
  exit 1
fi
export PATH="$NODE_RUNTIME_BIN_DIR:$PATH"
node_major="$(node --version | sed -n 's/^v\([0-9][0-9]*\).*/\1/p')"
if [[ -z "$node_major" || "$node_major" -lt 22 ]]; then
  echo "Tanban website requires Node.js 22 or newer" >&2
  exit 1
fi

for command_name in curl flock grep install npm rsync sed systemctl; do
  require_command "$command_name"
done
if [[ "$PROJECT_DIR" != /* || "$PROJECT_DIR" == "/" || "$PROJECT_DIR" == *".."* ]]; then
  echo "PROJECT_DIR must be a specific absolute path: $PROJECT_DIR" >&2
  exit 1
fi
if [[ ! "$WEBSITE_READY_TIMEOUT" =~ ^[1-9][0-9]*$ ]]; then
  echo "WEBSITE_READY_TIMEOUT must be a positive integer" >&2
  exit 1
fi
if [[ -n "$WEBSITE_PUBLIC_URL" && "$WEBSITE_PUBLIC_URL" != https://* ]]; then
  echo "WEBSITE_PUBLIC_URL must be empty or an HTTPS URL" >&2
  exit 1
fi
if [[ "$WEBSITE_BACKUP_ROOT" != /* || "$WEBSITE_BACKUP_ROOT" == "/" || "$WEBSITE_BACKUP_ROOT" == *".."* ]]; then
  echo "WEBSITE_BACKUP_ROOT must be a specific absolute directory: $WEBSITE_BACKUP_ROOT" >&2
  exit 1
fi

exec 9>"$DEPLOY_LOCK_FILE"
if ! flock -n 9; then
  echo "another Tanban website deployment is already running" >&2
  exit 1
fi

cd "$PROJECT_DIR"
if [[ ! -f .env.production ]]; then
  echo "missing $PROJECT_DIR/.env.production" >&2
  exit 1
fi
if ! systemctl cat "$WEBSITE_SERVICE" >/dev/null 2>&1; then
  echo "missing systemd service: $WEBSITE_SERVICE" >&2
  exit 1
fi
service_working_directory="$(systemctl show "$WEBSITE_SERVICE" --property=WorkingDirectory --value)"
if [[ "$service_working_directory" != "$PROJECT_DIR" ]]; then
  echo "website service WorkingDirectory must match PROJECT_DIR (service: $service_working_directory, deploy: $PROJECT_DIR)" >&2
  exit 1
fi

website_api_url="$(read_env_value NEXT_PUBLIC_TANBAN_API_URL)"
if [[ "$website_api_url" != https://* ]]; then
  echo "NEXT_PUBLIC_TANBAN_API_URL must be an HTTPS URL" >&2
  exit 1
fi
export NEXT_PUBLIC_TANBAN_API_URL="$website_api_url"

echo "installing deterministic website dependencies"
npm ci
if [[ -d dist ]]; then
  install -d -m 0755 "$WEBSITE_BACKUP_DIR"
  rsync -a dist/ "$WEBSITE_BACKUP_DIR/"
  WEBSITE_HAD_PRIOR_DIST=1
fi
WEBSITE_DIST_MUTATED=1
echo "building official website"
npm run build:prototype

for asset_path in "${WEBSITE_CRITICAL_ASSET_PATHS[@]}"; do
  if [[ ! -s "dist/client${asset_path}" ]]; then
    echo "website build is missing critical asset: dist/client${asset_path}" >&2
    exit 1
  fi
done

systemctl restart "$WEBSITE_SERVICE"
deadline=$((SECONDS + WEBSITE_READY_TIMEOUT))
website_body=""
until website_body="$(curl --fail --silent --show-error --max-time 3 "$WEBSITE_READY_URL")" \
  && grep -q "摊伴" <<<"$website_body"; do
  if ! systemctl is-active --quiet "$WEBSITE_SERVICE"; then
    systemctl status "$WEBSITE_SERVICE" --no-pager >&2 || true
    exit 1
  fi
  if ((SECONDS >= deadline)); then
    echo "website did not become ready before timeout" >&2
    journalctl -u "$WEBSITE_SERVICE" -n 100 --no-pager >&2 || true
    exit 1
  fi
  sleep 2
done

for asset_path in "${WEBSITE_CRITICAL_ASSET_PATHS[@]}"; do
  check_website_asset "$WEBSITE_READY_URL" "$asset_path"
done

if [[ -n "$WEBSITE_PUBLIC_URL" ]]; then
  public_website_body="$(curl --fail --silent --show-error --max-time 10 "${WEBSITE_PUBLIC_URL%/}/")"
  if ! grep -q "摊伴" <<<"$public_website_body"; then
    echo "public website readiness response did not contain the expected content" >&2
    exit 1
  fi
  for asset_path in "${WEBSITE_CRITICAL_ASSET_PATHS[@]}"; do
    check_website_asset "$WEBSITE_PUBLIC_URL" "$asset_path"
  done
fi

WEBSITE_DEPLOY_COMMITTED=1
echo "Tanban official website deployed"
if ((WEBSITE_HAD_PRIOR_DIST == 1)); then
  echo "Website build backup: $WEBSITE_BACKUP_DIR"
fi
