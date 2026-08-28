#!/usr/bin/env bash
set -euo pipefail

project_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)

# Keep one lifecycle owner for the local backend, setup restart, Vite proxy,
# readiness checks, and signal cleanup.
exec "$project_root/scripts/dev-setup.sh" "$@"
