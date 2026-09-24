#!/usr/bin/env bash
# collect_gitlab_mr.sh — 通过 GitLab REST API v4 收集一个 MR 的完整 review 上下文
# 用法: collect_gitlab_mr.sh <mr_url_or_project> <mr_iid>
#   mr_url_or_project : MR 完整 URL（如 https://gitlab.com/group/proj/-/merge_requests/123）
#                       或项目路径（如 group/proj，此时 mr_iid 必填）
#   mr_iid            : MR 的 IID（URL 形式时省略）
# 环境变量:
#   GITLAB_TOKEN : Personal Access Token，私有项目必填（read_api scope 即可）
#   GITLAB_HOST  : 自建实例地址，默认 https://gitlab.com
# 输出: 结构化文本到 stdout（MR 元信息、提交列表、变更文件、逐文件 diff）

set -euo pipefail

CONFIGURED_HOST="${GITLAB_HOST:-}"
HOST="${CONFIGURED_HOST:-https://gitlab.com}"
TOKEN="${GITLAB_TOKEN:-}"
INPUT="${1:?用法: collect_gitlab_mr.sh <mr_url|project_path> [mr_iid]}"
IID="${2:-}"
URL_HOST=""

# 解析输入为 project_path + iid，保留自建实例的 scheme、端口和嵌套 group。
if [[ "$INPUT" =~ ^(https?://[^/]+)/(.+)/-/merge_requests/([0-9]+)/?$ ]]; then
  URL_HOST="${BASH_REMATCH[1]}"
  PROJECT="${BASH_REMATCH[2]}"
  IID="${BASH_REMATCH[3]}"
elif [[ "$INPUT" =~ ^(https?://[^/]+)/(.+)/merge_requests/([0-9]+)/?$ ]]; then
  URL_HOST="${BASH_REMATCH[1]}"
  PROJECT="${BASH_REMATCH[2]}"
  IID="${BASH_REMATCH[3]}"
elif [ -n "$IID" ]; then
  PROJECT="$INPUT"
else
  echo "ERROR: 无法解析输入。传 MR 完整 URL，或 <project_path> <mr_iid>" >&2
  exit 1
fi

HOST="${HOST%/}"
URL_HOST="${URL_HOST%/}"
if [ -n "$URL_HOST" ]; then
  if [ -n "$CONFIGURED_HOST" ] && [ "$URL_HOST" != "$HOST" ]; then
    echo "ERROR: MR URL host ($URL_HOST) 与 GITLAB_HOST ($HOST) 不一致；拒绝发送凭证" >&2
    exit 1
  fi
  if [ -n "$TOKEN" ] && [ -z "$CONFIGURED_HOST" ] && [ "$URL_HOST" != "https://gitlab.com" ]; then
    echo "ERROR: 带 GITLAB_TOKEN 使用自建 MR URL 时必须显式设置匹配的 GITLAB_HOST；拒绝向未授权 host 发送凭证" >&2
    exit 1
  fi
  HOST="$URL_HOST"
fi

if [ "${MERGE_REVIEW_PARSE_ONLY:-}" = "1" ]; then
  printf 'host=%s\nproject=%s\niid=%s\n' "$HOST" "$PROJECT" "$IID"
  exit 0
fi

command -v curl >/dev/null && command -v jq >/dev/null || { echo "ERROR: 需要 curl 和 jq" >&2; exit 1; }

PROJECT_ENC=$(printf '%s' "$PROJECT" | jq -sRr @uri)
API="${HOST}/api/v4/projects/${PROJECT_ENC}/merge_requests/${IID}"

AUTH=()
[ -n "$TOKEN" ] && AUTH=(-H "PRIVATE-TOKEN: $TOKEN")
# bash 3.2 兼容：空数组需用 ${AUTH[@]+...} 形式展开

PAGINATION_DIR=$(mktemp -d)
trap 'rm -rf "$PAGINATION_DIR"' EXIT

fetch_paginated_array() {
  local endpoint="$1"
  local label="$2"
  local page=1
  local count=0
  local combined="${PAGINATION_DIR}/${label}-combined.json"
  local current="${PAGINATION_DIR}/${label}-page.json"
  local next="${PAGINATION_DIR}/${label}-next.json"
  printf '[]' >"$combined"
  while :; do
    curl -sf ${AUTH[@]+"${AUTH[@]}"} "${endpoint}?per_page=100&page=${page}" -o "$current" || return 1
    count=$(jq -er 'if type == "array" then length else error("expected array") end' "$current") || return 1
    jq -s '.[0] + .[1]' "$combined" "$current" >"$next" || return 1
    mv "$next" "$combined"
    [ "$count" -lt 100 ] && break
    page=$((page + 1))
    if [ "$page" -gt 100 ]; then
      echo "ERROR: ${label} 分页超过 100 页，拒绝输出不完整证据" >&2
      return 1
    fi
  done
  cat "$combined"
}

# 拉取 MR 元信息
MR_JSON=$(curl -sf ${AUTH[@]+"${AUTH[@]}"} "$API") || {
  echo "ERROR: 拉取 MR 失败（$API）。检查: 1) GITLAB_TOKEN 是否设置且有 read_api scope 2) 项目路径和 MR IID 是否正确 3) GITLAB_HOST 是否指向你的实例" >&2
  exit 1
}

echo "========== MR META =========="
echo "$MR_JSON" | jq -r '"title:  \(.title)\nurl:    \(.web_url)\nstate:  \(.state)\nauthor: \(.author.username)\nsource: \(.source_branch) -> \(.target_branch)\nsha:    \(.sha)\ndraft:  \(.work_in_progress)\ncreated: \(.created_at)\n---\ndescription:\n\(.description // "(空)")"'

echo
echo "========== MR COMMITS =========="
COMMITS_JSON=$(fetch_paginated_array "${API}/commits" "commits") || {
  echo "ERROR: MR commits 分页读取失败；拒绝在证据不完整时继续评审" >&2
  exit 1
}
echo "$COMMITS_JSON" | jq -r '.[] | "\(.short_id) | \(.author_name) | \(.created_at[0:10]) | \(.title)"'

echo
echo "========== CHANGED FILES & DIFFS =========="
# /diffs 接口返回逐文件 diff（v15.7+；旧版本用 /changes 的 changes 字段）
DIFFS_JSON=$(fetch_paginated_array "${API}/diffs" "diffs") || {
  echo "WARN: /diffs 接口不可用，回退到 /changes（旧版 GitLab）" >&2
  CHANGES_JSON=$(curl -sf ${AUTH[@]+"${AUTH[@]}"} "${API}/changes") || {
    echo "ERROR: /changes 回退接口读取失败" >&2
    exit 1
  }
  if [ "$(echo "$CHANGES_JSON" | jq -r '.overflow // false')" = "true" ]; then
    echo "ERROR: GitLab /changes 报告 overflow；拒绝输出不完整 diff" >&2
    exit 1
  fi
  DIFFS_JSON=$(echo "$CHANGES_JSON" | jq '[.changes[] | {old_path, new_path, new_file, deleted_file, renamed_file, diff}]')
}
if echo "$DIFFS_JSON" | jq -e 'any(.[]; (.too_large // false) or (.collapsed // false) or ((.diff // "") == ""))' >/dev/null; then
  echo "ERROR: GitLab 返回 collapsed/too_large/空 diff；拒绝在证据不完整时继续评审" >&2
  exit 1
fi

echo "$DIFFS_JSON" | jq -r '.[] | "\(if .new_file then "A" elif .deleted_file then "D" elif .renamed_file then "R" else "M" end)\t\(.new_path)"'

echo
echo "$DIFFS_JSON" | jq -r '.[] | "diff --git a/\(.old_path) b/\(.new_path)\n\(.diff)\n"'

TOTAL=$(echo "$DIFFS_JSON" | jq 'length')
echo "========== END =========="
echo "(共 $TOTAL 个变更文件；已自动读取全部分页)"
