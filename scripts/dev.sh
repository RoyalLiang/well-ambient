#!/usr/bin/env bash
set -euo pipefail

project_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)

for command_name in go pnpm curl; do
  if ! command -v "$command_name" >/dev/null 2>&1; then
    echo "$command_name 是运行本地开发环境所必需的" >&2
    exit 2
  fi
done

config_path="$project_root/config.yaml"
if [[ ! -f "$config_path" ]]; then
  if [[ -f "$project_root/config.example.yaml" ]]; then
    echo "未找到 config.yaml，将使用 config.example.yaml"
    config_path="$project_root/config.example.yaml"
  else
    echo "未找到配置文件 config.yaml 或 config.example.yaml" >&2
    exit 1
  fi
fi

api_host="${DEV_API_HOST:-127.0.0.1}"
api_port="${DEV_API_PORT:-8080}"

# Read configured port from config.yaml if not explicitly set
if [[ -z "${DEV_API_PORT:-}" && -f "$config_path" ]]; then
  parsed_port=$(awk '
    /^server:[[:space:]]*$/ { in_server=1; next }
    in_server && /^[^[:space:]]/ { exit }
    in_server && $1 == "port:" { print $2; exit }
  ' "$config_path")
  if [[ -n "$parsed_port" && "$parsed_port" =~ ^[0-9]+$ ]]; then
    api_port="$parsed_port"
  fi
fi

web_port="${DEV_WEB_PORT:-5173}"

runtime_dir=$(mktemp -d "${TMPDIR:-/tmp}/well-ambient-dev.XXXXXX")
server_binary="$runtime_dir/well-ambient-server"
backend_pid=""
frontend_pid=""

cleanup() {
  exit_code=$?
  trap - EXIT INT TERM
  echo ""
  echo "==> 正在停止前端与后端服务..."
  if [[ -n "$frontend_pid" ]] && kill -0 "$frontend_pid" >/dev/null 2>&1; then
    pkill -TERM -P "$frontend_pid" >/dev/null 2>&1 || true
    kill -TERM "$frontend_pid" >/dev/null 2>&1 || true
    for _ in {1..20}; do
      if ! kill -0 "$frontend_pid" >/dev/null 2>&1; then
        break
      fi
      sleep 0.05
    done
    if kill -0 "$frontend_pid" >/dev/null 2>&1; then
      pkill -KILL -P "$frontend_pid" >/dev/null 2>&1 || true
      kill -KILL "$frontend_pid" >/dev/null 2>&1 || true
    fi
    wait "$frontend_pid" >/dev/null 2>&1 || true
  fi
  if [[ -n "$backend_pid" ]] && kill -0 "$backend_pid" >/dev/null 2>&1; then
    pkill -TERM -P "$backend_pid" >/dev/null 2>&1 || true
    kill -TERM "$backend_pid" >/dev/null 2>&1 || true
    for _ in {1..30}; do
      if ! kill -0 "$backend_pid" >/dev/null 2>&1; then
        break
      fi
      sleep 0.05
    done
    if kill -0 "$backend_pid" >/dev/null 2>&1; then
      pkill -KILL -P "$backend_pid" >/dev/null 2>&1 || true
      kill -KILL "$backend_pid" >/dev/null 2>&1 || true
    fi
    wait "$backend_pid" >/dev/null 2>&1 || true
  fi
  rm -f "$server_binary"
  rmdir "$runtime_dir" >/dev/null 2>&1 || true
  echo "==> 本地开发环境已完全停止"
  exit "$exit_code"
}
trap cleanup EXIT INT TERM

echo "==> 编译后端服务..."
(
  cd "$project_root"
  go build -o "$server_binary" ./cmd/server
)

echo "==> 启动后端服务 (配置: $config_path)..."
"$server_binary" --config "$config_path" &
backend_pid=$!

# Wait for backend readiness
backend_ready=false
for _ in {1..100}; do
  if ! kill -0 "$backend_pid" >/dev/null 2>&1; then
    wait "$backend_pid"
    exit $?
  fi
  if curl --silent --fail "http://127.0.0.1:$api_port/live" >/dev/null 2>&1; then
    backend_ready=true
    break
  fi
  sleep 0.1
done

if [[ "$backend_ready" != true ]]; then
  echo "后端服务启动失败，未在 http://127.0.0.1:$api_port/live 响应" >&2
  exit 1
fi

echo "==> 后端服务已就绪: http://127.0.0.1:$api_port"
echo "==> 启动前端开发服务器 (Vite --host, 端口: $web_port)..."
echo "==> 提示: 按 Ctrl+C 可停止前后端服务 (保留缓冲期排空写请求，连按两次立即强退)"
echo ""

cd "$project_root"
VITE_API_PROXY_TARGET="http://127.0.0.1:$api_port" pnpm -C web dev --host --port "$web_port" &
frontend_pid=$!
set +e
wait "$frontend_pid"
frontend_status=$?
set -e
frontend_pid=""
exit "$frontend_status"
