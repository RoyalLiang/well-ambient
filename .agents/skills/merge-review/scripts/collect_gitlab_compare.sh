#!/usr/bin/env bash
# collect_gitlab_compare.sh — 通过 GitLab REST API v4 收集任意两个引用（分支/tag/commit）之间的评审上下文
# 用法: collect_gitlab_compare.sh <project_path> <from_ref> <to_ref>
#   from_ref : 基线（旧），to_ref : 目标（新）。等价于 git diff from...to 的引入改动视角
# 环境变量: GITLAB_TOKEN（私有项目必填，api scope）、GITLAB_HOST（默认 https://gitlab.com）
# 输出: 结构化文本到 stdout（compare 元信息、提交列表、变更文件、逐文件 diff）

set -euo pipefail

HOST="${GITLAB_HOST:-https://gitlab.com}"
TOKEN="${GITLAB_TOKEN:-}"
PROJECT="${1:?用法: collect_gitlab_compare.sh <project_path> <from_ref> <to_ref>}"
FROM="${2:?缺少 from_ref（基线分支/tag/commit）}"
TO="${3:?缺少 to_ref（目标分支/tag/commit）}"

command -v curl >/dev/null && command -v jq >/dev/null || { echo "ERROR: 需要 curl 和 jq" >&2; exit 1; }

PROJECT_ENC=$(printf '%s' "$PROJECT" | jq -sRr @uri)
API="${HOST}/api/v4/projects/${PROJECT_ENC}/repository/compare"

AUTH=()
[ -n "$TOKEN" ] && AUTH=(-H "PRIVATE-TOKEN: $TOKEN")

CMP_JSON=$(curl -sf ${AUTH[@]+"${AUTH[@]}"} --get "$API" --data-urlencode "from=$FROM" --data-urlencode "to=$TO") || {
  echo "ERROR: compare 失败（$API from=$FROM to=$TO）。检查: 1) GITLAB_TOKEN 权限 2) 项目路径 3) 两个 ref 是否存在（分支名含 / 时注意拼写）" >&2
  exit 1
}
if [ "$(echo "$CMP_JSON" | jq -r '.compare_timeout // false')" = "true" ]; then
  echo "ERROR: GitLab compare 超时，结果可能不完整；拒绝继续评审" >&2
  exit 1
fi
if echo "$CMP_JSON" | jq -e 'any(.diffs[]?; (.too_large // false) or (.collapsed // false) or ((.diff // "") == ""))' >/dev/null; then
  echo "ERROR: GitLab compare 返回 collapsed/too_large/空 diff；拒绝继续评审" >&2
  exit 1
fi

echo "========== COMPARE META =========="
echo "project: $PROJECT"
echo "range:   $FROM ... $TO"
echo "$CMP_JSON" | jq -r '"commit:  \(.commit.id[0:8]) (\(.commit.committed_date // ""))\ncommits: \(.commits | length) 个\ndiffs:   \(.diffs | length) 个文件"'
echo "$CMP_JSON" | jq -r 'if .compare_same_ref then "WARN: 两个 ref 相同，无差异" else empty end'

echo
echo "========== COMMITS (from..to) =========="
echo "$CMP_JSON" | jq -r '.commits[] | "\(.short_id) | \(.author_name) | \(.created_at[0:10]) | \(.title)"'

echo
echo "========== CHANGED FILES =========="
echo "$CMP_JSON" | jq -r '.diffs[] | "\(if .new_file then "A" elif .deleted_file then "D" elif .renamed_file then "R" else "M" end)\t\(.new_path)"'

echo
echo "========== FULL DIFF =========="
echo "$CMP_JSON" | jq -r '.diffs[] | "diff --git a/\(.old_path) b/\(.new_path)\n\(.diff)\n"'

echo "========== END =========="
echo "提示: 大 diff 可按文件分批: 同上 API 结果中按 new_path 过滤"
