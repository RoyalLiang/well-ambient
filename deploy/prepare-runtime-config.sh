#!/usr/bin/env bash
set -euo pipefail

umask 077

runtime_config=${1:?runtime config path is required}
template_config=${2:?template config path is required}
legacy_snapshot=${3:?legacy SQLite snapshot path is required}
container_legacy_path=/var/lib/well-ambient/legacy/well-ambient.db

if [[ ! -f "$template_config" ]]; then
  echo "production config template not found: $template_config" >&2
  exit 2
fi

mkdir -p "$(dirname "$runtime_config")"
if [[ ! -f "$runtime_config" ]]; then
  cp "$template_config" "$runtime_config"
fi
chmod 600 "$runtime_config"

# Runtime directories created before SQLite onboarding do not contain the
# legacy path. Evolve only when the standard mounted snapshot is present, and
# never overwrite an operator-provided path or migration decision.
if [[ ! -f "$legacy_snapshot" ]]; then
  exit 0
fi

if [[ ! -r "$legacy_snapshot" ]]; then
  echo "legacy SQLite snapshot is not readable by the deploy user: $legacy_snapshot" >&2
  exit 2
fi
if ! LC_ALL=C head -c 15 "$legacy_snapshot" | grep -qx 'SQLite format 3'; then
  echo "legacy migration source is not a valid SQLite snapshot: $legacy_snapshot" >&2
  exit 2
fi

echo "detected legacy SQLite host snapshot: $legacy_snapshot"
echo "configured read-only container migration source: $container_legacy_path"

if awk '
  /^database:[[:space:]]*(#.*)?$/ { in_database=1; next }
  in_database && /^[^[:space:]#]/ { in_database=0 }
  in_database && $1 == "legacy_sqlite_path:" { found=1 }
  END { exit(found ? 0 : 1) }
' "$runtime_config"; then
  exit 0
fi

temporary_config=$(mktemp "${runtime_config}.tmp.XXXXXX")
cleanup() {
  rm -f "$temporary_config"
}
trap cleanup EXIT

if ! awk -v legacy_path="$container_legacy_path" '
  /^database:[[:space:]]*(#.*)?$/ && !inserted {
    print
    print "  legacy_sqlite_path: " legacy_path
    inserted=1
    next
  }
  { print }
  END { if (!inserted) exit 42 }
' "$runtime_config" >"$temporary_config"; then
  echo "cannot add legacy_sqlite_path: database section missing in $runtime_config" >&2
  exit 2
fi

chmod 600 "$temporary_config"
mv "$temporary_config" "$runtime_config"
trap - EXIT
