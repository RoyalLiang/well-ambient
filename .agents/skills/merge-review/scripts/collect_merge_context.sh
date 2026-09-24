#!/usr/bin/env bash
# collect_merge_context.sh — 收集一个 merge 提交的完整 review 上下文
# 用法: collect_merge_context.sh <repo_dir> <merge_ref|--latest>
#   repo_dir   : git 仓库路径
#   merge_ref  : merge commit 的 hash/引用；--latest 表示显式选择最近一次 merge
# 输出: 结构化文本到 stdout，供 review 时读取

set -euo pipefail

REPO="${1:?用法: collect_merge_context.sh <repo_dir> <merge_ref|--latest>}"
REF="${2:-}"

cd "$REPO" || { echo "ERROR: 无法进入仓库目录 $REPO" >&2; exit 1; }
git rev-parse --is-inside-work-tree >/dev/null 2>&1 || { echo "ERROR: $REPO 不是 git 仓库" >&2; exit 1; }

if [ -z "$REF" ]; then
  echo "ERROR: 缺少 merge_ref；请显式传入提交，或使用 --latest" >&2
  exit 1
fi
if [ "$REF" = "--latest" ]; then
  REF=$(git log --merges -1 --format=%H) || true
  [ -n "$REF" ] || { echo "ERROR: 仓库中没有 merge 提交，请显式传入 merge_ref" >&2; exit 1; }
fi

FULL=$(git rev-parse "$REF" 2>/dev/null) || { echo "ERROR: 找不到提交 $REF" >&2; exit 1; }

# 校验是否为 merge 提交（至少两个父提交）
PARENTS=$(git rev-list --parents -n 1 "$FULL" | wc -w | tr -d ' ')
if [ "$PARENTS" -lt 3 ]; then
  echo "ERROR: $REF 不是 merge 提交（父提交数 $((PARENTS-1))）" >&2
  echo "提示: 可用 git log --merges --oneline -20 查看最近的 merge 提交" >&2
  exit 1
fi

P1=$(git rev-parse "${FULL}^1")
P2=$(git rev-parse "${FULL}^2")

echo "========== MERGE COMMIT =========="
git log -1 --format='hash:    %H%nsubject: %s%nauthor:  %an <%ae>%ndate:    %ad%nparents: %P' "$FULL"
echo
echo "first_parent (目标分支侧): $P1"
echo "second_parent (被合并分支): $P2"
echo "分支信息: $(git log -1 --format=%s "$FULL" | head -1)"

echo
echo "========== MERGED COMMITS (P1..P2) =========="
# 被合并分支带进来的全部提交
git log --format='%h | %an | %ad | %s' --date=short "$P1..$P2"
COUNT=$(git rev-list --count "$P1..$P2")
echo "(共 $COUNT 个提交)"

echo
echo "========== DIFFSTAT (P1 -> merge) =========="
git diff --stat "$P1" "$FULL"

echo
echo "========== CHANGED FILES =========="
git diff --name-status "$P1" "$FULL"

echo
echo "========== FULL DIFF (P1 -> merge) =========="
# 相对第一父提交的 diff = 该 merge 实际引入的改动（忽略目标分支自身的演进）
git diff "$P1" "$FULL"

echo
echo "========== END =========="
echo "提示: 大 diff 建议按文件分批查看: git diff $P1 $FULL -- <path>"
