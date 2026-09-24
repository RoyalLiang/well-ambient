#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
REPO_ROOT=$(CDPATH= cd -- "${SCRIPT_DIR}/../../../.." && pwd)
TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

cat >"${TMP_DIR}/curl" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
output=""
url=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    -o)
      output="$2"
      shift 2
      ;;
    -H|--data-urlencode)
      shift 2
      ;;
    -s|-f|-sf|-fs|--get)
      shift
      ;;
    *)
      url="$1"
      shift
      ;;
  esac
done
case "$url" in
  */merge_requests/123)
    body='{"title":"fixture","web_url":"https://gitlab.com/group/proj/-/merge_requests/123","state":"opened","author":{"username":"fixture"},"source_branch":"feature","target_branch":"main","sha":"abc","work_in_progress":false,"created_at":"2026-09-24T00:00:00Z","description":"fixture"}'
    ;;
  */commits*)
    body='[]'
    ;;
  */diffs*)
    body='[{"old_path":"big.bin","new_path":"big.bin","too_large":true,"diff":""}]'
    ;;
  */repository/compare)
    body='{"compare_timeout":true,"compare_same_ref":false,"commit":{"id":"abc","committed_date":"2026-09-24"},"commits":[],"diffs":[]}'
    ;;
  *)
    exit 22
    ;;
esac
if [ -n "$output" ]; then
  printf '%s' "$body" >"$output"
else
  printf '%s' "$body"
fi
EOF
chmod +x "${TMP_DIR}/curl"

if PATH="${TMP_DIR}:$PATH" bash "${SCRIPT_DIR}/collect_gitlab_mr.sh" \
  "https://gitlab.com/group/proj/-/merge_requests/123" >/dev/null 2>&1; then
  echo "incomplete MR diff was accepted" >&2
  exit 1
fi

if PATH="${TMP_DIR}:$PATH" bash "${SCRIPT_DIR}/collect_gitlab_compare.sh" \
  "group/proj" "main" "feature" >/dev/null 2>&1; then
  echo "compare timeout was accepted" >&2
  exit 1
fi

if bash "${SCRIPT_DIR}/collect_merge_context.sh" "$REPO_ROOT" >/dev/null 2>&1; then
  echo "missing merge_ref was accepted" >&2
  exit 1
fi

printf 'incomplete evidence guards: PASS\n'
