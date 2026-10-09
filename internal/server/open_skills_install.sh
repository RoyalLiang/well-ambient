#!/usr/bin/env bash
# Served by /open/v1/install.sh; the server replaces only the quoted base URL.
set -euo pipefail

BASE_URL=__WELL_AMBIENT_BASE_URL__
INSTALL_ROOT=''
ASSUME_YES=false
MCP_MODE=auto
MCP_ONLY=false
SKILLS=()

usage() {
  cat <<'HELP'
用法: bash install.sh [--root 安装根目录] [--skill 技能名] [--yes]
  --root DIR   将 Skill 安装到 DIR/skills，例如 ~/.codex 或 ~/.claude
  --skill NAME 仅安装指定技能（默认安装全部三个官方技能）
  --yes        不询问，使用首个检测目录；未检测到时使用 ~/.codex
  --help       显示帮助
在线示例: curl -fsSL '<服务端>/open/v1/install.sh' | bash -s -- --root "$HOME/.claude"
MCP 选项:
  --mcp         同时配置 Codex MCP，写入 DIR/config.toml
  --mcp-only    仅配置 Codex MCP，不重复下载 Skill
  --skip-mcp    仅安装 Skill，不配置 MCP

Codex 根目录会交互询问是否配置 MCP；无终端时需显式添加 --mcp。
自动配置需要 Python 3.11+（或 Python 3 + tomli）。先在页面“开放能力与 MCP”签发 API Key。
密钥通过终端隐藏输入，也可从环境变量 WELL_AMBIENT_API_KEY 读取；不要将密钥放进命令参数或 URL。
脚本验证 initialize / tools/list 后写入配置，备份原文件并保留其他服务；会替换 well-ambient 条目。
HELP
}

fail() { printf '错误: %s\n' "$*" >&2; exit 1; }

while [ "$#" -gt 0 ]; do
  case "$1" in
    --root)
      [ "$#" -ge 2 ] && [ -n "$2" ] || fail '--root 需要安装根目录'
      INSTALL_ROOT="$2"; shift 2 ;;
    --skill)
      [ "$#" -ge 2 ] || fail '--skill 需要技能名'
      case "$2" in
        jira-analysis|board-decision|code-review-reader) SKILLS+=("$2") ;;
        *) fail "未知技能: $2" ;;
      esac
      shift 2 ;;
    --yes|-y) ASSUME_YES=true; shift ;;
    --mcp) MCP_MODE=configure; shift ;;
    --mcp-only) MCP_MODE=configure; MCP_ONLY=true; shift ;;
    --skip-mcp) MCP_MODE=skip; shift ;;
    --help|-h) usage; exit 0 ;;
    *) usage >&2; fail "未知参数: $1" ;;
  esac
done

[ "$MCP_ONLY" = false ] || [ "$MCP_MODE" = configure ] || fail '--mcp-only 不能与 --skip-mcp 同时使用'
[ "${#SKILLS[@]}" -gt 0 ] || SKILLS=(jira-analysis board-decision code-review-reader)
if [ "$MCP_ONLY" = false ]; then
  for dependency in curl gzip tar mktemp; do
    command -v "$dependency" >/dev/null 2>&1 || fail "缺少命令: $dependency"
  done
fi

printf '\n=== well-ambient Skill 安装器 ===\n服务端地址: %s\n' "$BASE_URL"
printf '选择的是客户端安装根目录，Skill 文件将写入其 skills 子目录。\n'

ROOTS=()
LABELS=()
add_root() {
  local root="$1" label="$2" existing
  [ -d "$root" ] || return 0
  root=$(cd "$root" && pwd -P)
  for existing in "${ROOTS[@]:-}"; do
    [ "$existing" != "$root" ] || return 0
  done
  ROOTS+=("$root")
  LABELS+=("$label")
}

# Prefer Codex/Claude Code locations over a generic .agents directory.
add_root "$PWD/.codex" '项目 Codex'
add_root "$PWD/.claude" '项目 Claude Code'
add_root "${CODEX_HOME:-$HOME/.codex}" '用户 Codex'
add_root "${CLAUDE_CONFIG_DIR:-$HOME/.claude}" '用户 Claude Code'
add_root "$PWD/.agents" '项目通用 Agent'
add_root "$HOME/.agents" '用户通用 Agent'

if [ -z "$INSTALL_ROOT" ]; then
  if [ "${#ROOTS[@]}" -eq 0 ]; then
    ROOTS=("${CODEX_HOME:-$HOME/.codex}" "${CLAUDE_CONFIG_DIR:-$HOME/.claude}")
    LABELS=('用户 Codex（新建）' '用户 Claude Code（新建）')
  fi
  printf '\n可选安装根目录:\n'
  for index in "${!ROOTS[@]}"; do
    printf '  %s) %s: %s\n' "$((index + 1))" "${LABELS[$index]}" "${ROOTS[$index]}"
  done
  if [ "$ASSUME_YES" = true ]; then
    INSTALL_ROOT="${ROOTS[0]}"
    printf '已按 --yes 选择首个目录。\n'
  elif { [ -t 1 ] || [ -t 2 ]; } && { exec 3<>/dev/tty; } 2>/dev/null; then
    printf '  0) 输入其他安装根目录\n  q) 取消\n'
    while [ -z "$INSTALL_ROOT" ]; do
      printf '选择目录 [1]: ' >&3
      IFS= read -r choice <&3 || fail '已取消安装'
      choice="${choice:-1}"
      case "$choice" in
        q|Q) printf '已取消安装。\n'; exit 0 ;;
        0)
          printf '输入安装根目录（例如 ~/.claude 或 /path/to/project/.codex）: ' >&3
          IFS= read -r INSTALL_ROOT <&3 || fail '已取消安装' ;;
        *)
          for index in "${!ROOTS[@]}"; do
            if [ "$choice" = "$((index + 1))" ]; then INSTALL_ROOT="${ROOTS[$index]}"; break; fi
          done ;;
      esac
      [ -n "$INSTALL_ROOT" ] || printf '请输入列表中的编号，或选择 0 输入目录。\n' >&3
    done
    exec 3>&-
  elif [ "${#ROOTS[@]}" -eq 1 ]; then
    INSTALL_ROOT="${ROOTS[0]}"
    printf '仅检测到一个安装根目录，自动选择。\n'
  else
    fail '当前没有可交互终端。请添加 --root "$HOME/.codex" 或 --root "$HOME/.claude" 明确指定；使用 --yes 可接受首个目录。'
  fi
fi

case "$INSTALL_ROOT" in
  '~') INSTALL_ROOT="$HOME" ;;
  '~/'*) INSTALL_ROOT="$HOME/${INSTALL_ROOT#\~/}" ;;
  '~'*) fail '仅支持 ~ 或 ~/ 开头的当前用户路径，请使用绝对路径' ;;
esac
mkdir -p -- "$INSTALL_ROOT" || fail "无法创建安装根目录: $INSTALL_ROOT"
INSTALL_ROOT=$(cd "$INSTALL_ROOT" && pwd -P)
if [ "$MCP_ONLY" = false ]; then
  mkdir -p -- "$INSTALL_ROOT/skills" || fail "无法创建安装目录: $INSTALL_ROOT/skills"
  SKILLS_DIR=$(cd "$INSTALL_ROOT/skills" && pwd -P)
  printf '\n目标目录: %s\n' "$SKILLS_DIR"

  printf '正在探测服务端健康状态... '
  if curl --connect-timeout 5 --max-time 10 -fsSL "${BASE_URL}/health" >/dev/null 2>&1; then
    printf '正常\n'
  else
    printf '未通过；将尝试下载技能包，请留意后续下载结果。\n'
  fi

  INSTALLED_SKILLS=()
  FAILED_SKILLS=()
  TMP_FILE=''
  trap 'rm -f -- "${TMP_FILE:-}"' EXIT
  for skill in "${SKILLS[@]}"; do
    printf '正在下载并安装 Skill [%s]... ' "$skill"
    TMP_FILE=$(mktemp)
    if ! curl --connect-timeout 10 --max-time 120 -fsSL -H 'Cache-Control: no-cache' "${BASE_URL}/open/v1/skills/${skill}/archive" -o "$TMP_FILE" 2>/dev/null; then
      FAILED_SKILLS+=("$skill")
      printf '下载失败（检查网络或服务端状态）\n'
    elif ! gzip -t "$TMP_FILE" >/dev/null 2>&1 || ! tar -tzf "$TMP_FILE" >/dev/null 2>&1; then
      FAILED_SKILLS+=("$skill")
      printf '技能包校验失败（下载内容损坏；未写入安装目录，请重试或联系管理员）\n'
    elif tar -xzf "$TMP_FILE" -C "$SKILLS_DIR"; then
      INSTALLED_SKILLS+=("$skill")
      printf '完成\n'
    else
      FAILED_SKILLS+=("$skill")
      printf '写入失败（检查目标目录权限和磁盘空间）\n'
    fi
    rm -f -- "$TMP_FILE"
  done
  if [ "${#FAILED_SKILLS[@]}" -gt 0 ]; then
    printf '\n已成功安装: %s\n安装失败: %s\n目标目录: %s\n' "${INSTALLED_SKILLS[*]:-无}" "${FAILED_SKILLS[*]}" "$SKILLS_DIR"
    printf '安装未全部完成。可以使用 --skill <技能名> 重试失败项。\n' >&2
    exit 1
  fi

  printf '\n✓ Skill 文件安装完成\n目标目录: %s\n已成功安装: %s\n' "$SKILLS_DIR" "${INSTALLED_SKILLS[*]}"
  printf '在对应客户端重新加载技能；若使用项目目录，请打开该项目。\n'
fi

if [ "$MCP_MODE" = auto ]; then
  case "$INSTALL_ROOT" in
    */.codex|"${CODEX_HOME:-$HOME/.codex}")
      if [ "$ASSUME_YES" = false ] && { [ -t 1 ] || [ -t 2 ]; } && { exec 3<>/dev/tty; } 2>/dev/null; then
        printf '\n现在配置 Codex MCP？需先在页面签发 API Key [Y/n]: ' >&3
        IFS= read -r answer <&3 || answer=n
        exec 3>&-
        case "$answer" in ''|y|Y|yes|YES) MCP_MODE=configure ;; esac
      fi
      ;;
  esac
fi

if [ "$MCP_MODE" = configure ]; then
  command -v python3 >/dev/null 2>&1 || fail 'MCP 自动配置需要 Python 3.11+（或 Python 3 + tomli）；Skill 安装结果不受影响。'
  MCP_KEY="${WELL_AMBIENT_API_KEY:-}"
  if [ -z "$MCP_KEY" ]; then
    if { [ -t 1 ] || [ -t 2 ]; } && { exec 3<>/dev/tty; } 2>/dev/null; then
      printf '粘贴页面签发的完整 API Key（输入隐藏，回车确认）: ' >&3
      IFS= read -r -s MCP_KEY <&3 || MCP_KEY=''
      printf '\n' >&3
      exec 3>&-
    else
      fail 'MCP 未配置：无交互终端且未设置 WELL_AMBIENT_API_KEY。请在终端运行 --mcp-only 并隐藏输入凭证。'
    fi
  fi
  [ -n "$MCP_KEY" ] || fail 'MCP 未配置：凭证为空。可用 --mcp-only 重试。'
  printf '\n正在验证凭证与 MCP 连接，验证通过后写入 Codex 配置...\n'
  if WELL_AMBIENT_API_KEY="$MCP_KEY" python3 - "$INSTALL_ROOT" "${BASE_URL}/mcp" <<'WELL_AMBIENT_CODEX_PY'
__WELL_AMBIENT_CODEX_SETUP__
WELL_AMBIENT_CODEX_PY
  then
    unset MCP_KEY WELL_AMBIENT_API_KEY
    exit 0
  else
    unset MCP_KEY WELL_AMBIENT_API_KEY
    fail 'MCP 自动配置未完成；已安装的 Skill 保留。排除上述原因后可使用 --mcp-only 重试。'
  fi
fi

printf '\n=== MCP 接入待办（MCP 尚未配置）===\n'
printf '1. 在页面“AI 治理 → 开放能力与 MCP → 签发 API Key”签发集成凭证，来源可填写 codex-dev。\n'
printf '   完整密钥只显示一次；不要使用网页登录令牌或 Key ID。无写权限时请联系管理员。\n'
printf '2. Codex 自动配置：重新运行本脚本，添加 --root "安装根目录" --mcp-only，然后隐藏输入 API Key。\n'
printf '   手动配置：将下列 TOML 合并到所选 Codex 根目录的 config.toml（默认 ~/.codex/config.toml）：\n'
cat <<EOF
[mcp_servers."well-ambient"]
url = "${BASE_URL}/mcp"
http_headers = { Authorization = "Bearer <YOUR_API_KEY>" }
EOF
printf '   将 <YOUR_API_KEY> 替换为完整集成凭证，并将 config.toml 权限设为 600。\n'
printf '3. 重启 Codex 或重新加载会话，在 /mcp 查看连接与工具。codex mcp list 仅确认配置存在。\n'
printf '   HTTP 401：检查凭证是否过期或撤销；HTTP 403：请管理员检查开放能力开关、当前生效策略及来源权限。\n'
printf 'MCP 地址: %s/mcp\n' "$BASE_URL"
printf '在客户端本地配置 Authorization，将 <YOUR_API_KEY> 替换为真实凭证。\n'
printf '不要将 API Key 放入下载地址、命令 URL 或客户端安装深链。\n'
printf '\nCursor 的 .cursor/mcp.json 配置示例:\n'
cat <<EOF
{
  "mcpServers": {
    "well-ambient": {
      "url": "${BASE_URL}/mcp",
      "headers": {
        "Authorization": "Bearer <YOUR_API_KEY>"
      }
    }
  }
}
EOF
printf '\n其他客户端请按其 MCP 配置格式填写以上地址和 Authorization。\n'
printf '保存配置后，在客户端检查 MCP 连接与工具列表。\n'
