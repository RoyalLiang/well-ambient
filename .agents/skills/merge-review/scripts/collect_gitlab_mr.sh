#!/usr/bin/env bash
# Preserve the original CLI. Dependencies: python3 and curl. Exit 2 = insufficient evidence.
set -euo pipefail
SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
exec python3 -B "$SCRIPT_DIR/gitlab_context.py" mr "$@"
