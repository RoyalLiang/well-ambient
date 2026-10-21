#!/usr/bin/env bash
# Preserve the original CLI. Dependencies: python3 and git. Exit 2 = insufficient evidence.
set -euo pipefail
SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
exec python3 -B "$SCRIPT_DIR/local_context.py" range "$@"
