#!/usr/bin/env bash
set -Eeuo pipefail

PROJECT_DIR="${PROJECT_DIR:-/root/works/tanban}"
ENV_FILE="${ENV_FILE:-$PROJECT_DIR/.env.production}"
BACKUP_DIR="${BACKUP_DIR:-/var/backups/tanban/mysql}"
BACKUP_RETENTION_DAYS="${BACKUP_RETENTION_DAYS:-14}"
BACKUP_LOCK_TIMEOUT="${BACKUP_LOCK_TIMEOUT:-0}"
BACKUP_LABEL="${BACKUP_LABEL:-scheduled}"

umask 077

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

escape_option_value() {
  local value="$1"
  value="${value//\\/\\\\}"
  value="${value//\"/\\\"}"
  value="${value//$'\n'/\\n}"
  value="${value//$'\r'/\\r}"
  printf '%s' "$value"
}

for command_name in date find flock grep gzip install mktemp mv mysqldump rm sed stat tail; do
  require_command "$command_name"
done

if [[ ! -f "$ENV_FILE" || -L "$ENV_FILE" ]]; then
  echo "ENV_FILE must be an existing regular file, not a symlink: $ENV_FILE" >&2
  exit 1
fi
env_mode="$(stat -c '%a' "$ENV_FILE")"
if ((8#$env_mode & 8#077)); then
  echo "refusing to read $ENV_FILE because group/other permissions are set (mode $env_mode); run chmod 600" >&2
  exit 1
fi
if [[ "$BACKUP_DIR" != /* || "$BACKUP_DIR" == "/" || "$BACKUP_DIR" == *".."* ]]; then
  echo "BACKUP_DIR must be a specific absolute path: $BACKUP_DIR" >&2
  exit 1
fi
if [[ ! "$BACKUP_RETENTION_DAYS" =~ ^[1-9][0-9]*$ ]]; then
  echo "BACKUP_RETENTION_DAYS must be a positive integer" >&2
  exit 1
fi
if [[ ! "$BACKUP_LOCK_TIMEOUT" =~ ^[0-9]+$ ]]; then
  echo "BACKUP_LOCK_TIMEOUT must be a non-negative integer" >&2
  exit 1
fi
if [[ ! "$BACKUP_LABEL" =~ ^[a-zA-Z0-9][a-zA-Z0-9._-]{0,79}$ ]]; then
  echo "BACKUP_LABEL contains unsupported characters" >&2
  exit 1
fi

dsn="$(read_env_value TB_DATABASE_DSN)"
if [[ ! "$dsn" =~ ^([^:]+):(.*)@tcp\(([^:()]+):([0-9]+)\)/([^?]+)(\?.*)?$ ]]; then
  echo "TB_DATABASE_DSN must use user:password@tcp(host:port)/database form" >&2
  exit 1
fi
db_user="${BASH_REMATCH[1]}"
db_password="${BASH_REMATCH[2]}"
db_host="${BASH_REMATCH[3]}"
db_port="${BASH_REMATCH[4]}"
db_name="${BASH_REMATCH[5]}"
unset dsn

if [[ ! "$db_user" =~ ^[a-zA-Z0-9_.-]+$ || ! "$db_host" =~ ^[a-zA-Z0-9_.-]+$ || ! "$db_name" =~ ^[a-zA-Z0-9_$-]+$ ]]; then
  echo "database DSN contains an unsupported user, host, or database name" >&2
  exit 1
fi

install -d -m 0700 "$BACKUP_DIR"
exec 9>"$BACKUP_DIR/.backup.lock"
if ! flock -w "$BACKUP_LOCK_TIMEOUT" 9; then
  echo "another Tanban database backup is already running" >&2
  exit 1
fi

work_dir="$(mktemp -d "${TMPDIR:-/tmp}/tanban-mysql-backup.XXXXXX")"
defaults_file="$work_dir/mysql-client.cnf"
backup_timestamp="$(date +%Y%m%dT%H%M%S%z)"
partial_file="$BACKUP_DIR/.tanban-${BACKUP_LABEL}-${backup_timestamp}.sql.gz.partial"
final_file="$BACKUP_DIR/tanban-${BACKUP_LABEL}-${backup_timestamp}.sql.gz"

cleanup() {
  local status=$?
  rm -f -- "$partial_file"
  rm -rf -- "$work_dir"
  unset db_password
  exit "$status"
}
trap cleanup EXIT

escaped_user="$(escape_option_value "$db_user")"
escaped_password="$(escape_option_value "$db_password")"
escaped_host="$(escape_option_value "$db_host")"
printf '%s\n' \
  '[client]' \
  "user=\"$escaped_user\"" \
  "password=\"$escaped_password\"" \
  "host=\"$escaped_host\"" \
  "port=$db_port" \
  'protocol=tcp' \
  'default-character-set=utf8mb4' >"$defaults_file"
chmod 0600 "$defaults_file"
unset escaped_password db_password

echo "creating MySQL backup for database $db_name"
mysqldump --defaults-extra-file="$defaults_file" \
  --single-transaction \
  --quick \
  --routines \
  --events \
  --triggers \
  --hex-blob \
  --no-tablespaces \
  --default-character-set=utf8mb4 \
  "$db_name" | gzip -9 >"$partial_file"

if [[ ! -s "$partial_file" ]]; then
  echo "backup output is empty" >&2
  exit 1
fi
gzip -t "$partial_file"
if ! gzip -dc "$partial_file" | tail -n 20 | grep -q 'Dump completed on'; then
  echo "backup completion marker is missing" >&2
  exit 1
fi

chmod 0600 "$partial_file"
mv -- "$partial_file" "$final_file"
find "$BACKUP_DIR" -maxdepth 1 -type f -name 'tanban-*.sql.gz' -mmin "+$((BACKUP_RETENTION_DAYS * 1440))" -delete

echo "MySQL backup completed: $final_file"
