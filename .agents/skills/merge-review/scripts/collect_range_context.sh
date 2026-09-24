#!/usr/bin/env bash
# collect_range_context.sh — 收集本地仓库任意两个引用（分支/tag/commit）之间的评审上下文
# 用法: collect_range_context.sh <repo_dir> <from_ref> <to_ref>
#   等价于 git log/diff from..to（to 相对 from 引入的改动）
# 输出: 结构化文本到 stdout

set -euo pipefail

REPO="${1:?用法: collect_range_context.sh <repo_dir> <from_ref> <to_ref>}"
FROM="${2:?缺少 from_ref（基线）}"
TO="${3:?缺少 to_ref（目标）}"

cd "$REPO" || { echo "ERROR: 无法进入仓库目录 $REPO" >&2; exit 1; }
git rev-parse --is-inside-work-tree >/dev/null 2>&1 || { echo "ERROR: $REPO 不是 git 仓库" >&2; exit 1; }
git rev-parse --verify "$FROM^{commit}" >/dev/null 2>&1 || { echo "ERROR: 找不到引用 $FROM" >&2; exit 1; }
git rev-parse --verify "$TO^{commit}" >/dev/null 2>&1 || { echo "ERROR: 找不到引用 $TO" >&2; exit 1; }

echo "========== RANGE META =========="
echo "repo:  $REPO"
echo "range: $FROM..$TO"
echo "merge_base: $(git merge-base "$FROM" "$TO")"
COUNT=$(git rev-list --count "$FROM..$TO")
echo "commits: $COUNT 个"
[ "$COUNT" -eq 0 ] && echo "WARN: 范围内无提交（检查 from/to 顺序或两 ref 是否相同）"

echo
echo "========== COMMITS (from..to) =========="
git log --format='%h | %an | %ad | %s' --date=short "$FROM..$TO"

echo
echo "========== DIFFSTAT =========="
git diff --stat "$FROM...$TO"

echo
echo "========== CHANGED FILES =========="
git diff --name-status "$FROM...$TO"

echo
echo "========== FULL DIFF (merge-base...to) =========="
# 三点 diff：只含 to 侧引入的改动，不含 from 侧此后的演进
git diff "$FROM...$TO"

echo
echo "========== END =========="
echo "提示: 大 diff 建议按文件分批查看: git diff $FROM...$TO -- <path>"
