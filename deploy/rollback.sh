#!/usr/bin/env bash
set -euo pipefail
umask 077

project_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
env_file="$project_root/deploy/.env.production"
state_dir="$project_root/deploy/.state"

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
  echo "deploy/.env.production is required" >&2
  exit 2
fi
chmod 600 "$env_file"
if [[ ! -f "$state_dir/previous-version" ]]; then
  echo "no previous deployed version is recorded" >&2
  exit 2
fi
http_port=$(sed -n 's/^HTTP_PORT=//p' "$env_file" | tail -n 1 | tr -d '[:space:]')
http_port=${http_port:-8080}
if [[ ! "$http_port" =~ ^[0-9]+$ ]] || ((http_port < 1 || http_port > 65535)); then
  echo "HTTP_PORT must be an integer between 1 and 65535" >&2
  exit 2
fi
export APP_UID=${APP_UID:-$(id -u)}
export APP_GID=${APP_GID:-$(id -g)}
export WELL_AMBIENT_VERSION=$(tr -d '[:space:]' <"$state_dir/previous-version")
if [[ ! "$WELL_AMBIENT_VERSION" =~ ^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$ || "$WELL_AMBIENT_VERSION" == "latest" ]]; then
  echo "recorded previous version is not a valid immutable Docker tag" >&2
  exit 2
fi
export WELL_AMBIENT_COMMIT=rollback
export WELL_AMBIENT_BUILD_TIME=$(date -u +%Y-%m-%dT%H:%M:%SZ)

compose=(docker compose --env-file "$env_file" -f "$project_root/compose.yaml" --project-directory "$project_root")
docker image inspect "well-ambient-server:$WELL_AMBIENT_VERSION" >/dev/null
docker image inspect "well-ambient-web:$WELL_AMBIENT_VERSION" >/dev/null
"${compose[@]}" up -d --no-deps server web
curl --retry 20 --retry-delay 2 --retry-connrefused --fail --silent --show-error \
  "http://127.0.0.1:$http_port/ready" >/dev/null

if [[ -f "$state_dir/current-version" ]]; then
  cp "$state_dir/current-version" "$state_dir/rolled-back-from-version"
fi
printf '%s\n' "$WELL_AMBIENT_VERSION" >"$state_dir/current-version"
echo "rolled back application images to $WELL_AMBIENT_VERSION"
echo "database was not restored automatically; forward migrations are expected to remain additive"
