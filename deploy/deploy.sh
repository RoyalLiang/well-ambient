#!/usr/bin/env bash
set -euo pipefail
umask 077

project_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
env_file="$project_root/deploy/.env.production"
runtime_dir="$project_root/deploy/runtime"
state_dir="$project_root/deploy/.state"
backup_dir="$project_root/deploy/backups"
version=${1:-}

if [[ -z "$version" || "$version" == "latest" ]]; then
  echo "usage: deploy/deploy.sh <immutable-version>" >&2
  exit 2
fi
if [[ ! "$version" =~ ^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$ ]]; then
  echo "version must be a valid immutable Docker tag using A-Z a-z 0-9 . _ -" >&2
  exit 2
fi
for command_name in docker curl; do
  if ! command -v "$command_name" >/dev/null 2>&1; then
    echo "$command_name is required" >&2
    exit 2
  fi
done
if ! docker compose version >/dev/null 2>&1; then
  echo "Docker Compose v2 is required" >&2
  exit 2
fi
if [[ ! -f "$env_file" ]]; then
  echo "copy deploy/.env.production.example to deploy/.env.production and set secrets" >&2
  exit 2
fi
chmod 600 "$env_file"
if grep -q 'replace-with-' "$env_file"; then
  echo "deploy/.env.production still contains placeholder secrets" >&2
  exit 2
fi

http_port=$(sed -n 's/^HTTP_PORT=//p' "$env_file" | tail -n 1 | tr -d '[:space:]')
http_port=${http_port:-8080}
if [[ ! "$http_port" =~ ^[0-9]+$ ]] || ((http_port < 1 || http_port > 65535)); then
  echo "HTTP_PORT must be an integer between 1 and 65535" >&2
  exit 2
fi
postgres_password=$(sed -n 's/^POSTGRES_PASSWORD=//p' "$env_file" | tail -n 1 | tr -d '\r')
if [[ -z "$postgres_password" || ! "$postgres_password" =~ ^[A-Za-z0-9._~-]+$ ]]; then
  echo "POSTGRES_PASSWORD must be non-empty and use URL-safe characters: A-Z a-z 0-9 . _ ~ -" >&2
  exit 2
fi
setup_token=$(sed -n 's/^WELL_AMBIENT_SETUP_TOKEN=//p' "$env_file" | tail -n 1 | tr -d '\r')
if (( ${#setup_token} < 32 )); then
  echo "WELL_AMBIENT_SETUP_TOKEN must contain at least 32 characters" >&2
  exit 2
fi
if [[ "$setup_token" == "$postgres_password" ]]; then
  echo "WELL_AMBIENT_SETUP_TOKEN must be different from POSTGRES_PASSWORD" >&2
  exit 2
fi

mkdir -p "$runtime_dir/data/attachments" "$runtime_dir/data/legacy" "$state_dir" "$backup_dir"
if [[ ! -f "$runtime_dir/config.yaml" ]]; then
  cp "$project_root/deploy/config.production.example.yaml" "$runtime_dir/config.yaml"
fi
chmod 600 "$runtime_dir/config.yaml"

database_driver=$(awk '
  /^database:[[:space:]]*$/ { in_database=1; next }
  in_database && /^[^[:space:]]/ { exit }
  in_database && $1 == "driver:" { print $2; exit }
' "$runtime_dir/config.yaml")
database_driver=${database_driver:-sqlite}
database_dsn=$(awk '
  /^database:[[:space:]]*$/ { in_database=1; next }
  in_database && /^[^[:space:]]/ { exit }
  in_database && $1 == "dsn:" {
    sub(/^[[:space:]]*dsn:[[:space:]]*/, "")
    gsub(/^"|"$/, "")
    print
    exit
  }
' "$runtime_dir/config.yaml")
database_endpoint=""
if [[ -n "$database_dsn" && "$database_dsn" == *@* ]]; then
  database_endpoint=${database_dsn#*@}
  database_endpoint=${database_endpoint%%/*}
fi

export APP_UID=${APP_UID:-$(id -u)}
export APP_GID=${APP_GID:-$(id -g)}
export WELL_AMBIENT_VERSION="$version"
export WELL_AMBIENT_COMMIT=${WELL_AMBIENT_COMMIT:-$(git -C "$project_root" rev-parse --short=12 HEAD 2>/dev/null || echo unknown)}
export WELL_AMBIENT_BUILD_TIME=${WELL_AMBIENT_BUILD_TIME:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}

compose=(docker compose --env-file "$env_file" -f "$project_root/compose.yaml" --project-directory "$project_root")

if [[ -f "$state_dir/current-version" ]]; then
  cp "$state_dir/current-version" "$state_dir/previous-version"
fi

"${compose[@]}" build server web
"${compose[@]}" up -d --wait --wait-timeout 120 postgres

if [[ "$database_driver" == "setup" ]]; then
  "${compose[@]}" up -d --no-deps --wait --wait-timeout 240 server web
  curl --fail --silent --show-error "http://127.0.0.1:$http_port/ready" >/dev/null
  printf '%s\n' "$version" >"$state_dir/current-version"
  echo "deployed version $version in first-install mode"
  echo "open http://127.0.0.1:$http_port and use WELL_AMBIENT_SETUP_TOKEN from deploy/.env.production"
  echo "the setup API closes after PostgreSQL is initialized or an existing migrated database is verified"
  exit 0
fi

if [[ "$database_driver" != "postgres" && "$database_driver" != "postgresql" && "$database_driver" != "pg" ]]; then
  echo "production deploy requires database.driver=setup or postgres, got: $database_driver" >&2
  exit 2
fi

backup_path=""
if [[ -z "$database_endpoint" || "$database_endpoint" == "postgres:5432" ]]; then
  backup_path="$backup_dir/well-ambient-$(date -u +%Y%m%dT%H%M%SZ)-before-$version.dump"
  "${compose[@]}" exec -T postgres sh -ceu \
    'pg_dump --username "$POSTGRES_USER" --dbname "$POSTGRES_TARGET_DB" --format custom' \
    >"$backup_path"
  if [[ ! -s "$backup_path" ]]; then
    echo "database backup is empty" >&2
    exit 1
  fi
  "${compose[@]}" exec -T postgres pg_restore --list <"$backup_path" >/dev/null
else
  external_backup_reference=${WELL_AMBIENT_EXTERNAL_BACKUP_REFERENCE:-}
  if [[ -z "$external_backup_reference" ]]; then
    echo "external PostgreSQL detected; create and verify an upstream snapshot first" >&2
    echo "rerun with WELL_AMBIENT_EXTERNAL_BACKUP_REFERENCE=<snapshot-or-backup-id>" >&2
    exit 2
  fi
  if [[ ! "$external_backup_reference" =~ ^[A-Za-z0-9][A-Za-z0-9._:/-]{0,255}$ ]]; then
    echo "WELL_AMBIENT_EXTERNAL_BACKUP_REFERENCE contains unsupported characters" >&2
    exit 2
  fi
  backup_path="$backup_dir/external-$(date -u +%Y%m%dT%H%M%SZ)-before-$version.txt"
  printf 'external_database_endpoint=%s\nbackup_reference=%s\n' \
    "$database_endpoint" "$external_backup_reference" >"$backup_path"
fi

"${compose[@]}" run --rm --no-deps migrate
"${compose[@]}" up -d --no-deps --wait --wait-timeout 240 server web
curl --fail --silent --show-error "http://127.0.0.1:$http_port/ready" >/dev/null
printf '%s\n' "$version" >"$state_dir/current-version"

echo "deployed version $version"
echo "database backup: $backup_path"
