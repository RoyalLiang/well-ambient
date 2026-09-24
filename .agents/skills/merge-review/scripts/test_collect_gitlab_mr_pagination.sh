#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
COLLECTOR="${SCRIPT_DIR}/collect_gitlab_mr.sh"
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
    -H)
      shift 2
      ;;
    -s|-f|-sf|-fs)
      shift
      ;;
    *)
      url="$1"
      shift
      ;;
  esac
done

emit() {
  if [ -n "$output" ]; then
    printf '%s' "$1" >"$output"
  else
    printf '%s' "$1"
  fi
}

case "$url" in
  */merge_requests/123)
    emit '{"title":"fixture","web_url":"https://gitlab.com/group/proj/-/merge_requests/123","state":"opened","author":{"username":"fixture"},"source_branch":"feature","target_branch":"main","sha":"abc","work_in_progress":false,"created_at":"2026-09-24T00:00:00Z","description":"fixture"}'
    ;;
  */commits*page=1)
    json='['
    index=1
    while [ "$index" -le 100 ]; do
      [ "$index" -gt 1 ] && json="${json},"
      json="${json}{\"short_id\":\"c${index}\",\"author_name\":\"A\",\"created_at\":\"2026-09-24T00:00:00Z\",\"title\":\"commit ${index}\"}"
      index=$((index + 1))
    done
    emit "${json}]"
    ;;
  */commits*page=2)
    emit '[{"short_id":"c101","author_name":"B","created_at":"2026-09-24T00:00:00Z","title":"commit 101"}]'
    ;;
  */diffs*page=1)
    json='['
    index=1
    while [ "$index" -le 100 ]; do
      [ "$index" -gt 1 ] && json="${json},"
      json="${json}{\"old_path\":\"f${index}\",\"new_path\":\"f${index}\",\"new_file\":false,\"deleted_file\":false,\"renamed_file\":false,\"diff\":\"@@ fixture ${index}\"}"
      index=$((index + 1))
    done
    emit "${json}]"
    ;;
  */diffs*page=2)
    emit '[{"old_path":"f101","new_path":"f101","new_file":false,"deleted_file":false,"renamed_file":false,"diff":"@@ fixture 101"}]'
    ;;
  *)
    exit 22
    ;;
esac
EOF
chmod +x "${TMP_DIR}/curl"

output=$(PATH="${TMP_DIR}:$PATH" bash "$COLLECTOR" \
  "https://gitlab.com/group/proj/-/merge_requests/123")

printf '%s' "$output" | grep -q 'c101 | B | 2026-09-24 | commit 101'
printf '%s' "$output" | grep -q $'M\tf101'
printf '%s' "$output" | grep -q '共 101 个变更文件；已自动读取全部分页'

printf 'collect_gitlab_mr pagination: PASS\n'
