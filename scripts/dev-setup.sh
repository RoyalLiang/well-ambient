#!/usr/bin/env bash
set -euo pipefail

umask 077

project_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
api_host="${DEV_SETUP_API_HOST:-127.0.0.1}"
api_port="${DEV_SETUP_API_PORT:-18197}"
web_host="${DEV_SETUP_WEB_HOST:-127.0.0.1}"
web_port="${DEV_SETUP_WEB_PORT:-5175}"

for port_name in api_port web_port; do
  port_value=${!port_name}
  if [[ ! "$port_value" =~ ^[0-9]+$ ]] || ((port_value < 1 || port_value > 65535)); then
    echo "$port_name must be an integer between 1 and 65535" >&2
    exit 2
  fi
done

for command_name in go pnpm curl; do
  if ! command -v "$command_name" >/dev/null 2>&1; then
    echo "$command_name is required for the local setup environment" >&2
    exit 2
  fi
done

runtime_dir=$(mktemp -d "${TMPDIR:-/tmp}/well-ambient-dev-setup.XXXXXX")
config_path="$runtime_dir/config.yaml"
server_binary="$runtime_dir/well-ambient-server"
backend_pid=""

cleanup() {
  exit_code=$?
  trap - EXIT INT TERM
  if [[ -n "$backend_pid" ]] && kill -0 "$backend_pid" >/dev/null 2>&1; then
    kill -TERM "$backend_pid" >/dev/null 2>&1 || true
    wait "$backend_pid" >/dev/null 2>&1 || true
  fi
  rm -f "$config_path" "$server_binary"
  rmdir "$runtime_dir" >/dev/null 2>&1 || true
  exit "$exit_code"
}
trap cleanup EXIT INT TERM

legacy_sqlite_path="$project_root/well-ambient.db"
legacy_sqlite_escaped=$(printf '%s' "$legacy_sqlite_path" | sed "s/'/''/g")
{
  printf '%s\n' \
    'database:' \
    '  driver: setup'
  if [[ -f "$legacy_sqlite_path" ]]; then
    printf "  legacy_sqlite_path: '%s'\n" "$legacy_sqlite_escaped"
  fi
  printf '%s\n' \
    '  auto_migrate: false' \
    '  max_open_connections: 20' \
    '  max_idle_connections: 10' \
    '  connection_max_lifetime_minutes: 30' \
    '  connection_max_idle_time_minutes: 5' \
    '' \
    'server:' \
    "  host: $api_host" \
    "  port: $api_port" \
    "  public_url: http://$api_host:$api_port"
} >"$config_path"
chmod 600 "$config_path"

(
  cd "$project_root"
  go build -o "$server_binary" ./cmd/server
)

run_backend() {
  child_pid=""
  stop_child() {
    if [[ -n "$child_pid" ]] && kill -0 "$child_pid" >/dev/null 2>&1; then
      kill -TERM "$child_pid" >/dev/null 2>&1 || true
      wait "$child_pid" >/dev/null 2>&1 || true
    fi
    exit 0
  }
  trap stop_child INT TERM

  while true; do
		env -u WELL_AMBIENT_SETUP_TOKEN "$server_binary" \
			--config "$config_path" \
			--http-host "$api_host" \
			--http-port "$api_port" &
    child_pid=$!
    set +e
    wait "$child_pid"
    child_status=$?
    set -e
    child_pid=""
    if ((child_status != 0)); then
      return "$child_status"
    fi

    database_driver=$(awk '
      /^database:[[:space:]]*$/ { in_database=1; next }
      in_database && /^[^[:space:]]/ { exit }
      in_database && $1 == "driver:" { print $2; exit }
    ' "$config_path")
    if [[ -z "$database_driver" || "$database_driver" == "setup" ]]; then
      return 0
    fi
    echo "Database setup completed; restarting the local backend in $database_driver mode."
  done
}

run_backend &
backend_pid=$!

setup_ready=false
for _ in {1..150}; do
  if ! kill -0 "$backend_pid" >/dev/null 2>&1; then
    wait "$backend_pid"
    exit $?
  fi
  setup_status=$(curl --fail --silent --show-error "http://$api_host:$api_port/api/setup/status" 2>/dev/null || true)
  if printf '%s' "$setup_status" | grep -q '"setup_required"[[:space:]]*:[[:space:]]*true'; then
    setup_ready=true
    break
  fi
  sleep 0.1
done

if [[ "$setup_ready" != true ]]; then
  echo "local setup backend did not become ready at http://$api_host:$api_port" >&2
  exit 1
fi

echo "Local PostgreSQL setup page: http://$web_host:$web_port/"
echo "The generated setup token and its temporary file path are printed by the backend above."

cd "$project_root"
VITE_API_PROXY_TARGET="http://$api_host:$api_port" pnpm -C web dev --host "$web_host" --port "$web_port"
