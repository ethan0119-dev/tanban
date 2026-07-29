#!/usr/bin/env bash
set -Eeuo pipefail

PROJECT_DIR="${PROJECT_DIR:-/root/works/tanban}"
PLATFORM_ROOT="${PLATFORM_ROOT:-/www/wwwroot/tanban-platform}"
MERCHANT_ROOT="${MERCHANT_ROOT:-/www/wwwroot/tanban-merchant}"
NGINX_VHOST_DIR="${NGINX_VHOST_DIR:-/www/server/panel/vhost/nginx}"
NGINX_BACKUP_ROOT="${NGINX_BACKUP_ROOT:-/var/backups/tanban/nginx}"
NGINX_BIN="${NGINX_BIN:-nginx}"
API_READY_URL="http://127.0.0.1:18090/readyz"
API_READY_TIMEOUT="${API_READY_TIMEOUT:-180}"
RELEASE_RETENTION_COUNT="${RELEASE_RETENTION_COUNT:-5}"
MYSQL_BACKUP_DIR="${MYSQL_BACKUP_DIR:-/var/backups/tanban/mysql}"
MYSQL_BACKUP_RETENTION_DAYS="${MYSQL_BACKUP_RETENTION_DAYS:-14}"
DEPLOY_LOCK_FILE="${DEPLOY_LOCK_FILE:-/var/lock/tanban-server-deploy.lock}"
DEPLOY_LOCK_TIMEOUT="${DEPLOY_LOCK_TIMEOUT:-600}"
COMPOSE_FILE="infra/deploy/docker-compose.prod.yml"

if ! command -v flock >/dev/null 2>&1; then
  echo "required command not found: flock" >&2
  exit 1
fi
if [[ ! "$DEPLOY_LOCK_TIMEOUT" =~ ^[1-9][0-9]*$ ]]; then
  echo "DEPLOY_LOCK_TIMEOUT must be a positive integer" >&2
  exit 1
fi
if [[ "$DEPLOY_LOCK_FILE" != /* || "$DEPLOY_LOCK_FILE" == "/" || "$DEPLOY_LOCK_FILE" == *".."* ]]; then
  echo "DEPLOY_LOCK_FILE must be a specific absolute path: $DEPLOY_LOCK_FILE" >&2
  exit 1
fi
exec 9>"$DEPLOY_LOCK_FILE"
if ! flock -w "$DEPLOY_LOCK_TIMEOUT" 9; then
  echo "another Tanban deployment is still running after ${DEPLOY_LOCK_TIMEOUT}s" >&2
  exit 1
fi

RELEASE_ID="$(date -u +%Y%m%dT%H%M%SZ)-$$"
DEPLOY_TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/tanban-deploy.XXXXXX")"
NGINX_BACKUP_DIR="$NGINX_BACKUP_ROOT/$RELEASE_ID"
NGINX_MUTATED=0
DEPLOY_COMMITTED=0
API_MUTATED=0
API_HAD_PRIOR_IMAGE=0
API_ROLLBACK_ALLOWED=1
API_ROLLBACK_IMAGE="tanban-api:rollback-$RELEASE_ID"

declare -A STATIC_RELEASE_DIR=()
declare -A STATIC_PRIOR_TYPE=()
declare -A STATIC_PRIOR_VALUE=()
declare -a STATIC_ACTIVATED_KEYS=()

compose() {
  docker compose -f "$COMPOSE_FILE" "$@"
}

restore_static_releases() {
  local index key root prior_type prior_value rollback_link

  for ((index=${#STATIC_ACTIVATED_KEYS[@]} - 1; index >= 0; index--)); do
    key="${STATIC_ACTIVATED_KEYS[$index]}"
    if [[ "$key" == "platform" ]]; then
      root="$PLATFORM_ROOT"
    else
      root="$MERCHANT_ROOT"
    fi
    prior_type="${STATIC_PRIOR_TYPE[$key]}"
    prior_value="${STATIC_PRIOR_VALUE[$key]:-}"
    rollback_link="${root}.rollback-${RELEASE_ID}"

    case "$prior_type" in
      symlink)
        ln -s "$prior_value" "$rollback_link"
        mv -Tf "$rollback_link" "$root"
        ;;
      path)
        if [[ -L "$root" ]]; then
          rm -f -- "$root"
          mv -- "$prior_value" "$root"
        else
          echo "cannot restore $root: deployed path is no longer a symlink" >&2
        fi
        ;;
      missing)
        if [[ -L "$root" ]]; then
          rm -f -- "$root"
        fi
        ;;
    esac
  done
}

restore_nginx_configs() {
  local name target backup missing_marker restore_path

  for name in tbapi.666qwe.cn.conf tbadmin.666qwe.cn.conf mysales.666qwe.cn.conf tanban-acme-bootstrap.conf; do
    target="$NGINX_VHOST_DIR/$name"
    backup="$NGINX_BACKUP_DIR/$name"
    missing_marker="$NGINX_BACKUP_DIR/.missing-$name"

    if [[ -e "$backup" || -L "$backup" ]]; then
      restore_path="$NGINX_VHOST_DIR/.${name}.restore-${RELEASE_ID}"
      cp -a -- "$backup" "$restore_path"
      mv -f -- "$restore_path" "$target"
    elif [[ -f "$missing_marker" ]]; then
      rm -f -- "$target"
    fi
  done
}

on_exit() {
  local status=$?
  set +e

  if ((status != 0)) && ((DEPLOY_COMMITTED == 0)); then
    echo "deployment failed; restoring the previous static and Nginx configuration" >&2
    if ((NGINX_MUTATED == 1)); then
      restore_nginx_configs
      "$NGINX_BIN" -t && "$NGINX_BIN" -s reload
    fi
    restore_static_releases
    if ((API_MUTATED == 1)); then
      if ((API_ROLLBACK_ALLOWED == 0)); then
        echo "keeping the new API because it passed readiness with a forward-only schema; restore static/Nginx only and fix forward" >&2
      elif ((API_HAD_PRIOR_IMAGE == 1)); then
        echo "restoring previous API image" >&2
        docker tag "$API_ROLLBACK_IMAGE" tanban-api:local
        compose up -d --no-build api
        if ! wait_for_api; then
          echo "automatic rollback started the previous API image, but it did not become ready; manual recovery is required" >&2
        fi
      else
        compose rm -sf api >/dev/null 2>&1 || true
      fi
    fi
  fi

  rm -rf -- "$DEPLOY_TMP_DIR"
  exit "$status"
}
trap on_exit EXIT

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "required command not found: $1" >&2
    exit 1
  fi
}

validate_static_root() {
  local root="$1"
  if [[ "$root" != /www/wwwroot/* || "$root" == "/www/wwwroot/" || "$root" == *".."* ]]; then
    echo "static root must be a specific child of /www/wwwroot: $root" >&2
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

wait_for_api() {
  local deadline=$((SECONDS + API_READY_TIMEOUT)) container_id state health

  echo "waiting up to ${API_READY_TIMEOUT}s for $API_READY_URL"
  until curl --fail --silent --show-error --max-time 3 "$API_READY_URL" >/dev/null; do
    container_id="$(compose ps -q api)"
    if [[ -z "$container_id" ]]; then
      echo "API container is not running" >&2
      compose ps >&2 || true
      return 1
    fi

    state="$(docker inspect --format '{{.State.Status}}' "$container_id" 2>/dev/null || true)"
    health="$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{end}}' "$container_id" 2>/dev/null || true)"
    if [[ "$state" == "exited" || "$state" == "dead" || "$health" == "unhealthy" ]]; then
      echo "API failed during startup (state=$state health=$health)" >&2
      compose logs --tail=200 api >&2 || true
      return 1
    fi
    if ((SECONDS >= deadline)); then
      echo "API did not become ready before timeout" >&2
      compose ps >&2 || true
      compose logs --tail=200 api >&2 || true
      return 1
    fi
    sleep 2
  done
}

prune_release_directories() {
  local releases_root="$1" active_path="$2" index=0 name path
  local -a names=()

  [[ -d "$releases_root" ]] || return 0
  mapfile -t names < <(find "$releases_root" -mindepth 1 -maxdepth 1 -type d -printf '%f\n' | sort -r)
  for name in "${names[@]}"; do
    [[ "$name" =~ ^[0-9]{8}T[0-9]{6}Z-[0-9]+$ ]] || continue
    index=$((index + 1))
    path="$releases_root/$name"
    if ((index <= RELEASE_RETENTION_COUNT)) || [[ "$(readlink -f "$path")" == "$active_path" ]]; then
      continue
    fi
    rm -rf -- "$path"
  done
}

prune_nginx_backups() {
  local index=0 name path
  local -a names=()

  [[ -d "$NGINX_BACKUP_ROOT" ]] || return 0
  mapfile -t names < <(find "$NGINX_BACKUP_ROOT" -mindepth 1 -maxdepth 1 -type d -printf '%f\n' | sort -r)
  for name in "${names[@]}"; do
    [[ "$name" =~ ^[0-9]{8}T[0-9]{6}Z-[0-9]+$ ]] || continue
    index=$((index + 1))
    if ((index <= RELEASE_RETENTION_COUNT)); then
      continue
    fi
    path="$NGINX_BACKUP_ROOT/$name"
    rm -rf -- "$path"
  done
}

prune_api_rollback_images() {
  local index=0 tag
  local -a tags=()

  mapfile -t tags < <(docker image ls tanban-api --format '{{.Tag}}' | sed -n '/^rollback-[0-9]\{8\}T[0-9]\{6\}Z-[0-9][0-9]*$/p' | sort -r)
  for tag in "${tags[@]}"; do
    index=$((index + 1))
    if ((index <= RELEASE_RETENTION_COUNT)); then
      continue
    fi
    docker image rm "tanban-api:$tag" >/dev/null 2>&1 || echo "warning: could not remove old API rollback image tanban-api:$tag" >&2
  done
}

cleanup_old_release_artifacts() {
  local platform_active merchant_active
  platform_active="$(readlink -f "$PLATFORM_ROOT")"
  merchant_active="$(readlink -f "$MERCHANT_ROOT")"

  prune_release_directories "${PLATFORM_ROOT}.releases" "$platform_active"
  prune_release_directories "${MERCHANT_ROOT}.releases" "$merchant_active"
  prune_nginx_backups
  prune_api_rollback_images
}

prepare_static_release() {
  local key="$1" source_dir="$2" root="$3" releases_root release_dir

  if [[ ! -f "$source_dir/index.html" ]]; then
    echo "static build is missing $source_dir/index.html" >&2
    exit 1
  fi

  releases_root="${root}.releases"
  release_dir="$releases_root/$RELEASE_ID"
  install -d -m 0755 "$releases_root" "$release_dir"
  rsync -a --delete "$source_dir/" "$release_dir/"
  STATIC_RELEASE_DIR[$key]="$release_dir"
}

activate_static_release() {
  local key="$1" root="$2" release_dir next_link legacy_backup
  release_dir="${STATIC_RELEASE_DIR[$key]}"
  next_link="${root}.next-${RELEASE_ID}"

  ln -s "$release_dir" "$next_link"
  if [[ -L "$root" ]]; then
    STATIC_PRIOR_TYPE[$key]="symlink"
    STATIC_PRIOR_VALUE[$key]="$(readlink "$root")"
    mv -Tf "$next_link" "$root"
  elif [[ -e "$root" ]]; then
    legacy_backup="${root}.pre-tanban-${RELEASE_ID}"
    if [[ -e "$legacy_backup" || -L "$legacy_backup" ]]; then
      echo "static backup path already exists: $legacy_backup" >&2
      exit 1
    fi
    STATIC_PRIOR_TYPE[$key]="path"
    STATIC_PRIOR_VALUE[$key]="$legacy_backup"
    mv -- "$root" "$legacy_backup"
    if ! mv -- "$next_link" "$root"; then
      mv -- "$legacy_backup" "$root"
      exit 1
    fi
  else
    STATIC_PRIOR_TYPE[$key]="missing"
    STATIC_PRIOR_VALUE[$key]=""
    mv -- "$next_link" "$root"
  fi

  STATIC_ACTIVATED_KEYS+=("$key")
}

preflight_nginx_configs() {
  local include_dir="$DEPLOY_TMP_DIR/nginx-includes" test_config="$DEPLOY_TMP_DIR/nginx.conf"
  install -d -m 0700 "$include_dir"
  install -m 0644 infra/nginx/tbapi.666qwe.cn.conf "$include_dir/tbapi.666qwe.cn.conf"
  install -m 0644 infra/nginx/tbadmin.666qwe.cn.conf "$include_dir/tbadmin.666qwe.cn.conf"
  install -m 0644 infra/nginx/mysales.666qwe.cn.conf "$include_dir/mysales.666qwe.cn.conf"

  printf '%s\n' \
    'worker_processes 1;' \
    "pid $DEPLOY_TMP_DIR/nginx.pid;" \
    'error_log stderr notice;' \
    'events { worker_connections 16; }' \
    'http {' \
    '    default_type application/octet-stream;' \
    '    access_log off;' \
    "    include $include_dir/*.conf;" \
    '}' >"$test_config"

  "$NGINX_BIN" -t -q -e stderr -p / -c "$test_config"
}

backup_nginx_configs() {
  local name target
  install -d -m 0700 "$NGINX_BACKUP_DIR"

  for name in tbapi.666qwe.cn.conf tbadmin.666qwe.cn.conf mysales.666qwe.cn.conf tanban-acme-bootstrap.conf; do
    target="$NGINX_VHOST_DIR/$name"
    if [[ -e "$target" || -L "$target" ]]; then
      cp -a -- "$target" "$NGINX_BACKUP_DIR/$name"
    else
      : >"$NGINX_BACKUP_DIR/.missing-$name"
    fi
  done
}

install_nginx_configs() {
  local name staged
  NGINX_MUTATED=1

  for name in tbapi.666qwe.cn.conf tbadmin.666qwe.cn.conf mysales.666qwe.cn.conf; do
    staged="$NGINX_VHOST_DIR/.${name}.new-${RELEASE_ID}"
    install -m 0644 "infra/nginx/$name" "$staged"
    mv -f -- "$staged" "$NGINX_VHOST_DIR/$name"
  done
  rm -f -- "$NGINX_VHOST_DIR/tanban-acme-bootstrap.conf"
}

for command_name in curl docker find install npm readlink rm rsync sed sort "$NGINX_BIN"; do
  require_command "$command_name"
done

cd "$PROJECT_DIR"

if [[ ! -f .env.production ]]; then
  echo "missing $PROJECT_DIR/.env.production" >&2
  exit 1
fi
if [[ ! "$API_READY_TIMEOUT" =~ ^[1-9][0-9]*$ ]]; then
  echo "API_READY_TIMEOUT must be a positive integer" >&2
  exit 1
fi
if [[ ! "$RELEASE_RETENTION_COUNT" =~ ^[1-9][0-9]*$ ]]; then
  echo "RELEASE_RETENTION_COUNT must be a positive integer" >&2
  exit 1
fi
if [[ ! "$MYSQL_BACKUP_RETENTION_DAYS" =~ ^[1-9][0-9]*$ ]]; then
  echo "MYSQL_BACKUP_RETENTION_DAYS must be a positive integer" >&2
  exit 1
fi
validate_static_root "$PLATFORM_ROOT"
validate_static_root "$MERCHANT_ROOT"
if [[ "$PLATFORM_ROOT" == "$MERCHANT_ROOT" ]]; then
  echo "PLATFORM_ROOT and MERCHANT_ROOT must be different" >&2
  exit 1
fi
if [[ "$NGINX_VHOST_DIR" != /* || ! -d "$NGINX_VHOST_DIR" ]]; then
  echo "NGINX_VHOST_DIR must be an existing absolute directory: $NGINX_VHOST_DIR" >&2
  exit 1
fi
if [[ "$NGINX_BACKUP_ROOT" != /* || "$NGINX_BACKUP_ROOT" == "/" || "$NGINX_BACKUP_ROOT" == *".."* ]]; then
  echo "NGINX_BACKUP_ROOT must be a specific absolute directory: $NGINX_BACKUP_ROOT" >&2
  exit 1
fi

http_addr="$(read_env_value TB_HTTP_ADDR)"
if [[ "$http_addr" != "127.0.0.1:18090" ]]; then
  echo "TB_HTTP_ADDR must be exactly 127.0.0.1:18090; refusing to expose or misroute the API (found: ${http_addr:-unset})" >&2
  exit 1
fi

seed_demo="$(read_env_value TB_SEED_DEMO | tr '[:upper:]' '[:lower:]')"
case "$seed_demo" in
  ""|0|false|no|off) ;;
  *)
    echo "TB_SEED_DEMO must be disabled for production deployment; demo seeding can create an unintended default store" >&2
    exit 1
    ;;
esac

compose config --quiet

echo "creating a verified pre-deploy MySQL backup"
PROJECT_DIR="$PROJECT_DIR" \
  BACKUP_DIR="$MYSQL_BACKUP_DIR" \
  BACKUP_RETENTION_DAYS="$MYSQL_BACKUP_RETENTION_DAYS" \
  BACKUP_LOCK_TIMEOUT=300 \
  BACKUP_LABEL="predeploy-$RELEASE_ID" \
  bash scripts/mysql-backup.sh

if docker image inspect tanban-api:local >/dev/null 2>&1; then
  docker tag tanban-api:local "$API_ROLLBACK_IMAGE"
  API_HAD_PRIOR_IMAGE=1
fi
API_MUTATED=1
echo "building API image"
compose build api
echo "starting API"
compose up -d --no-build api
wait_for_api
# Migration 011 expands one legacy template scope into multiple copy roles and
# changes its unique key. Once the new API has reached readiness, a later
# frontend/Nginx failure must not replace it with the old API. The exit trap
# will still restore static files and Nginx, leaving this healthy API in place.
API_ROLLBACK_ALLOWED=0

echo "building static frontends"
# npm's filtered workspace install can omit transitive packages needed by
# Vite/Rollup on a clean production host. Install the lockfile as a whole so
# both static builds see the same deterministic dependency tree.
npm ci
npm run build:platform
npm run build:merchant

prepare_static_release platform apps/platform-web/dist "$PLATFORM_ROOT"
prepare_static_release merchant apps/merchant-web/dist "$MERCHANT_ROOT"
install -d -m 0755 /www/wwwroot/tanban-api-acme
install -m 0644 infra/wechat-domain-verification/YYMFacfbfJ.txt \
  /www/wwwroot/tanban-api-acme/YYMFacfbfJ.txt

echo "preflighting Nginx vhosts in an isolated include directory"
preflight_nginx_configs
backup_nginx_configs

activate_static_release platform "$PLATFORM_ROOT"
activate_static_release merchant "$MERCHANT_ROOT"
install_nginx_configs

"$NGINX_BIN" -t
"$NGINX_BIN" -s reload
curl --fail --silent --show-error --max-time 5 "$API_READY_URL" >/dev/null

DEPLOY_COMMITTED=1
cleanup_old_release_artifacts || echo "warning: release cleanup did not complete; deployment remains active" >&2
echo "tanban deployed (release: $RELEASE_ID)"
echo "Nginx configuration backup: $NGINX_BACKUP_DIR"
