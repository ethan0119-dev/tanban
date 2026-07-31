#!/usr/bin/env bash
set -Eeuo pipefail

PROJECT_DIR="${PROJECT_DIR:-/root/works/tanban}"
WEBSITE_SERVICE="${WEBSITE_SERVICE:-tanban-website.service}"
WEBSITE_READY_URL="${WEBSITE_READY_URL:-http://127.0.0.1:18100/}"
WEBSITE_READY_TIMEOUT="${WEBSITE_READY_TIMEOUT:-60}"
DEPLOY_LOCK_FILE="${DEPLOY_LOCK_FILE:-/var/lock/tanban-website-deploy.lock}"

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

for command_name in curl flock grep npm sed systemctl; do
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

website_api_url="$(read_env_value NEXT_PUBLIC_TANBAN_API_URL)"
if [[ "$website_api_url" != https://* ]]; then
  echo "NEXT_PUBLIC_TANBAN_API_URL must be an HTTPS URL" >&2
  exit 1
fi
export NEXT_PUBLIC_TANBAN_API_URL="$website_api_url"

echo "installing deterministic website dependencies"
npm ci
echo "building official website"
npm run build:prototype

systemctl restart "$WEBSITE_SERVICE"
deadline=$((SECONDS + WEBSITE_READY_TIMEOUT))
until curl --fail --silent --show-error --max-time 3 "$WEBSITE_READY_URL" | grep -q "摊伴"; do
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

echo "Tanban official website deployed"
