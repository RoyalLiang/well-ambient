#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
COLLECTOR="${SCRIPT_DIR}/collect_gitlab_mr.sh"

assert_parse() {
  local input="$1"
  local expected_host="$2"
  local expected_project="$3"
  local expected_iid="$4"
  local actual
  actual=$(MERGE_REVIEW_PARSE_ONLY=1 bash "$COLLECTOR" "$input")
  [ "$actual" = "host=${expected_host}
project=${expected_project}
iid=${expected_iid}" ] || {
    printf 'unexpected parse for %s\n%s\n' "$input" "$actual" >&2
    exit 1
  }
}

assert_parse \
  "https://gitlab.com/group/proj/-/merge_requests/123" \
  "https://gitlab.com" \
  "group/proj" \
  "123"

assert_parse \
  "https://gitlab.example.test/platform/fms/dispatch/-/merge_requests/42/" \
  "https://gitlab.example.test" \
  "platform/fms/dispatch" \
  "42"

assert_parse \
  "http://gitlab.local:8080/team/proj/-/merge_requests/9" \
  "http://gitlab.local:8080" \
  "team/proj" \
  "9"

actual=$(GITLAB_HOST="http://gitlab.local:8080" MERGE_REVIEW_PARSE_ONLY=1 bash "$COLLECTOR" "team/proj" "7")
[ "$actual" = "host=http://gitlab.local:8080
project=team/proj
iid=7" ] || {
  printf 'unexpected project+iid parse\n%s\n' "$actual" >&2
  exit 1
}

if GITLAB_TOKEN="secret" MERGE_REVIEW_PARSE_ONLY=1 bash "$COLLECTOR" \
  "https://attacker.example/team/proj/-/merge_requests/1" >/dev/null 2>&1; then
  echo "mismatched token host was accepted" >&2
  exit 1
fi

actual=$(GITLAB_HOST="https://gitlab.example.test" GITLAB_TOKEN="secret" MERGE_REVIEW_PARSE_ONLY=1 \
  bash "$COLLECTOR" "https://gitlab.example.test/team/proj/-/merge_requests/11")
[ "$actual" = "host=https://gitlab.example.test
project=team/proj
iid=11" ] || {
  printf 'unexpected authorized host parse\n%s\n' "$actual" >&2
  exit 1
}

printf 'collect_gitlab_mr parser: PASS\n'
