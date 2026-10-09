"""Embedded in install.sh; verify MCP before atomically updating local Codex TOML."""
import copy
import json
import os
from pathlib import Path
import re
import sys
import tempfile
import urllib.error
import urllib.parse
import urllib.request
import uuid

try:
    import tomllib
except ImportError:
    try:
        import tomli as tomllib
    except ImportError:
        tomllib = None


class SetupError(Exception):
    pass


def table_headers(text):
    """Find real table boundaries, ignoring comments and multiline string contents."""
    quote = None
    for number, line in enumerate(text.splitlines(keepends=True)):
        stripped = line.strip()
        if quote is None and stripped.startswith("["):
            try:
                node = tomllib.loads(stripped + "\n__well_ambient_marker__ = true\n")
                path = []
                while isinstance(node, dict) and "__well_ambient_marker__" not in node:
                    key, node = next(iter(node.items()))
                    path.append(key)
                    if isinstance(node, list):
                        node = node[0]
                if isinstance(node, dict) and node.get("__well_ambient_marker__") is True:
                    yield number, path
            except (ValueError, StopIteration):
                pass
        index = 0
        while index < len(line):
            if quote:
                if quote.startswith('"') and line[index] == "\\":
                    index += 2
                    continue
                if line.startswith(quote, index):
                    index += len(quote)
                    quote = None
                    continue
            else:
                if line[index] == "#":
                    break
                if line[index] in ("'", '"'):
                    quote = line[index] * (3 if line.startswith(line[index] * 3, index) else 1)
                    index += len(quote)
                    continue
            index += 1


def merge_config(text, endpoint, token):
    if tomllib is None:
        raise SetupError("自动配置需要 Python 3.11+，或安装 tomli。请按页面的 Codex 模板手动配置。")
    try:
        original = tomllib.loads(text)
        servers = original.get("mcp_servers", {})
        if not isinstance(servers, dict):
            raise ValueError()
        lines = text.splitlines(keepends=True)
        boundaries = list(table_headers(text)) + [(len(lines), [])]
        removed = set()
        for index, (start, path) in enumerate(boundaries[:-1]):
            if path[:2] == ["mcp_servers", "well-ambient"]:
                removed.update(range(start, boundaries[index + 1][0]))
        if "well-ambient" in servers and not removed:
            raise SetupError("现有 well-ambient 使用内联或点号 TOML 配置；为保留原配置，请手动替换该服务条目。")
        block = ('[mcp_servers."well-ambient"]\nurl = ' + json.dumps(endpoint) +
                 '\nhttp_headers = { Authorization = ' + json.dumps("Bearer " + token) + ' }\n')
        merged = "".join(line for index, line in enumerate(lines) if index not in removed).rstrip() + "\n\n" + block
        expected = copy.deepcopy(original)
        expected.setdefault("mcp_servers", {})["well-ambient"] = {
            "url": endpoint, "http_headers": {"Authorization": "Bearer " + token}}
        if tomllib.loads(merged) != expected:
            raise SetupError("无法在保留其他配置的同时合并 TOML，请按页面模板手动配置。")
        return merged
    except ValueError:
        raise SetupError("现有 config.toml 格式无效或无法安全合并；原文件未修改，请先检查 TOML 配置。") from None


def verify_mcp(endpoint, token):
    parsed = urllib.parse.urlsplit(endpoint)
    if parsed.scheme not in ("http", "https") or not parsed.netloc or parsed.username or parsed.password or parsed.query or parsed.fragment:
        raise SetupError("MCP 地址无效，不能包含凭证、查询参数或片段。")
    headers = {"Authorization": "Bearer " + token, "Content-Type": "application/json",
               "Accept": "application/json, text/event-stream"}

    class NoRedirect(urllib.request.HTTPRedirectHandler):
        def redirect_request(self, req, fp, code, msg, response_headers, newurl):
            return None  # Never forward the credential to another URL.

    opener = urllib.request.build_opener(NoRedirect)

    def rpc(request_id, method, params=None):
        payload = {"jsonrpc": "2.0", "method": method}
        if request_id is not None:
            payload["id"] = request_id
        if params is not None:
            payload["params"] = params
        request = urllib.request.Request(endpoint, json.dumps(payload).encode(), headers, method="POST")
        try:
            with opener.open(request, timeout=20) as response:
                body = response.read(2 * 1024 * 1024)
                session = response.headers.get("Mcp-Session-Id")
                if request_id is None:
                    return {}
                result = json.loads(body)
        except urllib.error.HTTPError as error:
            if error.code == 401:
                raise SetupError("凭证无效、过期或已撤销（HTTP 401），请重新签发 API Key。") from None
            if error.code == 403:
                raise SetupError("服务端拒绝接入（HTTP 403）。请管理员检查开放能力开关、当前生效策略及来源权限；安装 Skill 不会解除限制。") from None
            raise SetupError("MCP 请求失败（HTTP %s），请检查服务端或代理配置。" % error.code) from None
        except (urllib.error.URLError, TimeoutError, OSError):
            raise SetupError("无法连接 MCP，请检查地址、网络及代理配置。") from None
        except ValueError:
            raise SetupError("MCP 响应不是有效 JSON，请检查 MCP 地址及代理配置。") from None
        if not isinstance(result, dict) or result.get("id") != request_id or "error" in result or not isinstance(result.get("result"), dict):
            raise SetupError("MCP 协议验证失败，服务端未返回成功结果。")
        if session:
            headers["Mcp-Session-Id"] = session
        return result["result"]

    initialized = rpc(1, "initialize", {"protocolVersion": "2025-03-26", "capabilities": {},
                      "clientInfo": {"name": "well-ambient-installer", "version": "1.0"}})
    protocol = initialized.get("protocolVersion")
    if not isinstance(protocol, str) or not isinstance(initialized.get("capabilities"), dict):
        raise SetupError("MCP 初始化响应不完整。")
    headers["MCP-Protocol-Version"] = protocol
    rpc(None, "notifications/initialized", {})
    tools = rpc(2, "tools/list").get("tools")
    if not isinstance(tools, list):
        raise SetupError("MCP 工具列表响应不完整。")
    return len(tools)


def configure(root, endpoint, token):
    if not re.fullmatch(r"wa_live_[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+", token):
        raise SetupError("请输入本页面签发的完整 wa_live_ API Key，不能使用登录令牌、Key ID 或占位符。")
    root = Path(root)
    target = root / "config.toml"
    lock = root / ".well-ambient-mcp.lock"
    if target.is_symlink():
        raise SetupError("config.toml 是符号链接；为保留原配置，请按页面模板手动配置。")
    try:
        descriptor = os.open(lock, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    except FileExistsError:
        raise SetupError("检测到另一个 MCP 配置进程。请等待其完成；确认无进程运行后可移除 .well-ambient-mcp.lock 重试。") from None
    os.close(descriptor)
    temporary = None
    try:
        before = target.read_bytes() if target.exists() else None
        if before is not None and len(before) > 2 * 1024 * 1024:
            raise SetupError("config.toml 超过 2 MB，请手动配置。")
        try:
            text = before.decode("utf-8") if before is not None else ""
        except UnicodeError:
            raise SetupError("config.toml 不是 UTF-8 文件，请手动配置。") from None
        merged = merge_config(text, endpoint, token)
        count = verify_mcp(endpoint, token)
        print("MCP 验证通过：initialize / tools/list（%s 个工具）。" % count)
        if target.is_symlink() or (target.read_bytes() if target.exists() else None) != before:
            raise SetupError("验证期间 config.toml 已被其他进程修改，请重试。")
        if before is not None:
            backup = root / ("config.toml.well-ambient-" + uuid.uuid4().hex + ".bak")
            with os.fdopen(os.open(backup, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600), "wb") as handle:
                handle.write(before)
            print("原配置备份: %s（权限 600，含凭证，请妥善保管）" % backup)
        with tempfile.NamedTemporaryFile(mode="w", encoding="utf-8", dir=root, delete=False) as handle:
            temporary = Path(handle.name)
            os.chmod(temporary, 0o600)
            handle.write(merged)
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(temporary, target)
        print("Codex MCP 已写入: %s（权限 600）" % target)
        print("已更新 well-ambient 服务条目，其他配置保持不变。重启 Codex 或重新加载会话后，在 /mcp 检查连接与工具列表。")
    finally:
        if temporary and temporary.exists():
            temporary.unlink()
        lock.unlink()


if __name__ == "__main__":
    try:
        configure(sys.argv[1], sys.argv[2], os.environ.pop("WELL_AMBIENT_API_KEY", "").strip())
    except SetupError as error:
        print("MCP 未配置: %s" % error, file=sys.stderr)
        sys.exit(1)
    except OSError:
        print("MCP 未配置: 无法读取或写入本地配置，请检查目录权限。", file=sys.stderr)
        sys.exit(1)
