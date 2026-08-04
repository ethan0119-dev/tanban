#!/usr/bin/env bash
set -Eeuo pipefail

PROJECT_DIR="${PROJECT_DIR:-/srv/tanban/current}"
ENV_FILE="${ENV_FILE:-/etc/tanban/env/production.env}"
WEBSITE_SERVICE="${WEBSITE_SERVICE:-tanban-website.service}"
WEBSITE_READY_URL="${WEBSITE_READY_URL:-http://127.0.0.1:18100/}"
WEBSITE_PUBLIC_URL="${WEBSITE_PUBLIC_URL:-https://tanban.com.cn}"
WEBSITE_READY_TIMEOUT="${WEBSITE_READY_TIMEOUT:-60}"
DEPLOY_LOCK_FILE="${DEPLOY_LOCK_FILE:-/var/lock/tanban-website-deploy.lock}"
NODE_RUNTIME_BIN_DIR="${NODE_RUNTIME_BIN_DIR:-/opt/node-v22/bin}"
WEBSITE_BACKUP_ROOT="${WEBSITE_BACKUP_ROOT:-/var/backups/tanban/website}"
WEBSITE_BUILD_ROOT="${WEBSITE_BUILD_ROOT:-/var/cache/tanban/website-build}"
WEBSITE_RELEASE_ROOT="${WEBSITE_RELEASE_ROOT:-/var/lib/tanban/website-releases}"

WEBSITE_RELEASE_ID="$(date -u +%Y%m%dT%H%M%SZ)-$$"
WEBSITE_BACKUP_DIR="$WEBSITE_BACKUP_ROOT/$WEBSITE_RELEASE_ID"
WEBSITE_RELEASE_DIR="$WEBSITE_RELEASE_ROOT/$WEBSITE_RELEASE_ID"
WEBSITE_DIST_MUTATED=0
WEBSITE_HAD_PRIOR_DIST=0
WEBSITE_DEPLOY_COMMITTED=0
WEBSITE_PRIOR_DIST_TARGET=""

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
  value="$(sed -n "s/^[[:space:]]*${key}[[:space:]]*=[[:space:]]*//p" "$ENV_FILE" | tail -n 1)"
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
  local rollback_link="$PROJECT_DIR/.dist-rollback-$WEBSITE_RELEASE_ID"
  echo "restoring previous website build from $WEBSITE_PRIOR_DIST_TARGET" >&2
  ln -s "$WEBSITE_PRIOR_DIST_TARGET" "$rollback_link"
  mv -Tf "$rollback_link" "$PROJECT_DIR/dist"
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

for command_name in curl flock grep install ln mv npm readlink rsync sed systemctl; do
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
if [[ "$WEBSITE_BUILD_ROOT" != /* || "$WEBSITE_BUILD_ROOT" == "/" || "$WEBSITE_BUILD_ROOT" == *".."* ]]; then
  echo "WEBSITE_BUILD_ROOT must be a specific absolute directory: $WEBSITE_BUILD_ROOT" >&2
  exit 1
fi
if [[ "$WEBSITE_RELEASE_ROOT" != /* || "$WEBSITE_RELEASE_ROOT" == "/" || "$WEBSITE_RELEASE_ROOT" == *".."* ]]; then
  echo "WEBSITE_RELEASE_ROOT must be a specific absolute directory: $WEBSITE_RELEASE_ROOT" >&2
  exit 1
fi
if [[ "$WEBSITE_BUILD_ROOT" == "$PROJECT_DIR" || "$WEBSITE_BUILD_ROOT" == "$PROJECT_DIR/"* ]]; then
  echo "WEBSITE_BUILD_ROOT must be outside PROJECT_DIR" >&2
  exit 1
fi

exec 9>"$DEPLOY_LOCK_FILE"
if ! flock -n 9; then
  echo "another Tanban website deployment is already running" >&2
  exit 1
fi

cd "$PROJECT_DIR"
if [[ ! -f "$ENV_FILE" ]]; then
  echo "missing $ENV_FILE" >&2
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

echo "preparing isolated website build"
install -d -m 0755 "$WEBSITE_BUILD_ROOT"
rsync -a --delete \
  --exclude='.dist-*' \
  --exclude='.env.production' \
  --exclude='.vinext/' \
  --exclude='.wrangler/' \
  --exclude='dist/' \
  --exclude='node_modules/' \
  "$PROJECT_DIR/" "$WEBSITE_BUILD_ROOT/"
if [[ -e "$WEBSITE_BUILD_ROOT/node_modules" && ! -L "$WEBSITE_BUILD_ROOT/node_modules" ]]; then
  echo "website build node_modules must be a symlink" >&2
  exit 1
fi
if [[ ! -L "$WEBSITE_BUILD_ROOT/node_modules" ]]; then
  ln -s "$PROJECT_DIR/node_modules" "$WEBSITE_BUILD_ROOT/node_modules"
elif [[ "$(readlink "$WEBSITE_BUILD_ROOT/node_modules")" != "$PROJECT_DIR/node_modules" ]]; then
  echo "website build node_modules points to an unexpected location" >&2
  exit 1
fi
if [[ -e "$WEBSITE_BUILD_ROOT/dist" || -L "$WEBSITE_BUILD_ROOT/dist" ]]; then
  install -d -m 0755 "$WEBSITE_BACKUP_ROOT"
  mv "$WEBSITE_BUILD_ROOT/dist" "$WEBSITE_BACKUP_ROOT/unfinished-$WEBSITE_RELEASE_ID"
fi

echo "building official website"
(cd "$WEBSITE_BUILD_ROOT" && npm run build:prototype)

for asset_path in "${WEBSITE_CRITICAL_ASSET_PATHS[@]}"; do
  if [[ ! -s "$WEBSITE_BUILD_ROOT/dist/client${asset_path}" ]]; then
    echo "website build is missing critical asset: $WEBSITE_BUILD_ROOT/dist/client${asset_path}" >&2
    exit 1
  fi
done

install -d -m 0755 "$WEBSITE_RELEASE_ROOT"
if [[ -e "$WEBSITE_RELEASE_DIR" || -L "$WEBSITE_RELEASE_DIR" ]]; then
  echo "website release path already exists: $WEBSITE_RELEASE_DIR" >&2
  exit 1
fi
mv "$WEBSITE_BUILD_ROOT/dist" "$WEBSITE_RELEASE_DIR"

next_dist_link="$PROJECT_DIR/.dist-next-$WEBSITE_RELEASE_ID"
if [[ -e "$next_dist_link" || -L "$next_dist_link" ]]; then
  echo "website activation link already exists: $next_dist_link" >&2
  exit 1
fi
ln -s "$WEBSITE_RELEASE_DIR" "$next_dist_link"
if [[ -L "$PROJECT_DIR/dist" ]]; then
  WEBSITE_PRIOR_DIST_TARGET="$(readlink -f "$PROJECT_DIR/dist")"
  if [[ ! -d "$WEBSITE_PRIOR_DIST_TARGET" ]]; then
    echo "current website dist symlink target is missing: $WEBSITE_PRIOR_DIST_TARGET" >&2
    exit 1
  fi
  WEBSITE_HAD_PRIOR_DIST=1
elif [[ -d "$PROJECT_DIR/dist" ]]; then
  install -d -m 0755 "$WEBSITE_BACKUP_ROOT"
  if [[ -e "$WEBSITE_BACKUP_DIR" || -L "$WEBSITE_BACKUP_DIR" ]]; then
    echo "website backup path already exists: $WEBSITE_BACKUP_DIR" >&2
    exit 1
  fi
  mv "$PROJECT_DIR/dist" "$WEBSITE_BACKUP_DIR"
  WEBSITE_PRIOR_DIST_TARGET="$WEBSITE_BACKUP_DIR"
  WEBSITE_HAD_PRIOR_DIST=1
  WEBSITE_DIST_MUTATED=1
elif [[ -e "$PROJECT_DIR/dist" ]]; then
  echo "website dist must be a directory, symlink, or missing" >&2
  exit 1
fi

WEBSITE_DIST_MUTATED=1
mv -Tf "$next_dist_link" "$PROJECT_DIR/dist"
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
  echo "Previous website build: $WEBSITE_PRIOR_DIST_TARGET"
fi
