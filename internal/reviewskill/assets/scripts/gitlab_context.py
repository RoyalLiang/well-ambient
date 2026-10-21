#!/usr/bin/env python3
"""Collect fixed GitLab review evidence. Exit 2 means insufficient evidence."""
import argparse
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
from urllib.parse import parse_qs, quote, unquote, urlencode, urlsplit


class Insufficient(Exception):
    pass


class HTTPFailure(Insufficient):
    def __init__(self, status):
        self.status = status
        super().__init__(f"GitLab HTTP {status}; 未确认完整证据")


def require(condition, message):
    if not condition:
        raise Insufficient(message)


def sha(value):
    require(isinstance(value, str) and re.fullmatch(r"[0-9a-fA-F]{40}|[0-9a-fA-F]{64}", value), "缺少完整 SHA")
    return value.lower()


def host_url(value):
    p = urlsplit(value)
    require(p.scheme in ("http", "https") and p.netloc and not p.username and not p.password
            and not p.query and not p.fragment, "GitLab host 无效")
    return value.rstrip("/")


def valid_project(project):
    require(isinstance(project, str) and project and all(
        x and x not in (".", "..", "-") for x in project.split("/")
    ), "项目路径无效")
    return project


def parse_mr(value, iid, configured_host, token=""):
    configured_base = host_url(configured_host)
    configured = urlsplit(configured_base)
    if value.startswith(("http://", "https://")):
        p = urlsplit(value)
        require(p.netloc and not p.username and not p.password, "MR URL 不允许携带账号或密码")
        if token:
            require((p.scheme, p.netloc) == (configured.scheme, configured.netloc),
                    "携带 GITLAB_TOKEN 时 MR URL 主机与端口必须与 GITLAB_HOST 一致")
        else:
            require(p.hostname == configured.hostname, "MR URL 域名与 GITLAB_HOST 不一致")
        prefix = configured.path.rstrip("/") if (p.scheme, p.netloc) == (configured.scheme, configured.netloc) else ""
        require(not prefix or p.path.startswith(prefix + "/"), "MR URL 不在配置的 GitLab 路径下")
        path = p.path[len(prefix):]
        # Match /-/ first; the old optional delimiter was swallowed by the project group.
        match = re.fullmatch(r"/(.+)/-/merge_requests/([1-9][0-9]*)(?:/(?:diffs|commits|pipelines))?/?", path)
        if not match:
            match = re.fullmatch(r"/(.+)/merge_requests/([1-9][0-9]*)(?:/(?:diffs|commits|pipelines))?/?", path)
        require(match is not None, "无法解析 MR URL")
        project, parsed_iid = unquote(match[1]), match[2]
        require(not iid or iid == parsed_iid, "URL 和参数中的 MR IID 不一致")
        host, iid = f"{p.scheme}://{p.netloc}" + prefix, parsed_iid
    else:
        host, project = configured_base, value
    require(isinstance(iid, str) and re.fullmatch(r"[1-9][0-9]*", iid), "MR IID 必须是正整数")
    return host, valid_project(project), iid


class Client:
    def __init__(self, host, token):
        self.host, self.token = host_url(host), token or ""
        if self.token:
            require(self.host.startswith("https://"), "携带 GITLAB_TOKEN 时必须使用 https")
            require(not re.search(r"[\r\n\0]", self.token), "GITLAB_TOKEN 包含非法控制字符")

    def get(self, path, params=None):
        url = self.host + path + ("?" + urlencode(params, doseq=True) if params else "")
        with tempfile.TemporaryDirectory(prefix="merge-review-http-") as d:
            hp, bp = Path(d, "headers"), Path(d, "body")
            command = ["curl", "-q", "-sS", "--connect-timeout", "10", "--max-time", "60",
                       "--dump-header", str(hp), "--output", str(bp), "--write-out", "%{http_code}"]
            if self.token:
                cp = Path(d, "curl.cfg")
                escaped = self.token.replace("\\", "\\\\").replace('"', '\\"')
                cp.write_text(f'header = "PRIVATE-TOKEN: {escaped}"\n', encoding="utf-8")
                cp.chmod(0o600)
                command += ["--config", str(cp)]
            # No -L: do not forward authentication to a redirect destination.
            try:
                response = subprocess.run(command + [url], text=True, capture_output=True, timeout=70)
            except (OSError, subprocess.TimeoutExpired) as exc:
                raise Insufficient("curl 不可用或请求超时") from exc
            require(response.returncode == 0, "GitLab 传输失败或超时")
            require(re.fullmatch(r"[0-9]{3}", response.stdout.strip()), "缺少 HTTP 状态码")
            status = int(response.stdout.strip())
            if not 200 <= status < 300:
                raise HTTPFailure(status)
            try:
                body, raw_headers = json.loads(bp.read_text()), hp.read_text()
            except (OSError, UnicodeError, ValueError) as exc:
                raise Insufficient("GitLab 响应不是完整 JSON/HTTP 头") from exc
        headers = {}
        for line in raw_headers.splitlines():
            if line.startswith("HTTP/"):
                headers = {}  # Retain only final headers, ignoring proxy/100 responses.
            elif ":" in line:
                key, val = line.split(":", 1)
                key = key.lower().strip()
                headers[key] = (headers[key] + "," if key in headers else "") + val.strip()
        return body, headers

    def pages(self, path, identity):
        rows, seen, expected_total, expected_pages, page = [], set(), None, None, 1
        while page <= 1000:
            body, headers = self.get(path, {"per_page": 100, "page": page})
            require(isinstance(body, list) and len(body) <= 100, "分页响应不是合法数组")
            if "x-page" in headers:
                require(headers["x-page"] == str(page), "分页页码与请求不一致")
            if "x-total" in headers:
                require(re.fullmatch(r"[0-9]+", headers["x-total"]), "分页总数无效")
                total = int(headers["x-total"])
                require(expected_total is None or expected_total == total, "分页总数在采集中变化")
                expected_total = total
            if "x-total-pages" in headers:
                require(re.fullmatch(r"[0-9]+", headers["x-total-pages"]), "分页总页数无效")
                total_pages = int(headers["x-total-pages"])
                require(expected_pages is None or expected_pages == total_pages, "分页总页数在采集中变化")
                expected_pages = total_pages
            require(not body or expected_pages is None or page <= expected_pages, "实际页码超过声明总页数")
            for row in body:
                require(isinstance(row, dict), "分页元素不是对象")
                key = identity(row)
                require(key not in seen, "分页出现重复对象，可能重复页或版本漂移")
                seen.add(key)
                rows.append(row)
            next_page = None
            links = re.findall(r'<([^>]+)>\s*;\s*rel=["\']?next["\']?', headers.get("link", ""), re.I)
            require(len(links) <= 1, "分页返回多个 next 链接")
            if links:
                nxt, current = urlsplit(links[0]), urlsplit(self.host + path)
                require((nxt.scheme, nxt.netloc, nxt.path) == (current.scheme, current.netloc, current.path)
                        and not nxt.username and not nxt.password, "分页 next 链接越出当前接口")
                values = parse_qs(nxt.query).get("page", [])
                require(len(values) == 1 and re.fullmatch(r"[0-9]+", values[0]), "分页 next 链接缺少页码")
                next_page = int(values[0])
            if headers.get("x-next-page"):
                value = headers["x-next-page"]
                require(re.fullmatch(r"[0-9]+", value), "X-Next-Page 无效")
                require(next_page is None or next_page == int(value), "分页头互相矛盾")
                next_page = int(value)
            if next_page is None and len(body) == 100:
                next_page = page + 1  # Headers may be omitted; probe the page boundary.
            if next_page is None and expected_pages is not None and page < expected_pages:
                next_page = page + 1
            if next_page is None:
                require(expected_total is None or expected_total == len(rows), "分页返回数量少于声明总数")
                return rows
            require(body and next_page == page + 1, "分页回环、跳页或空页仍声明下一页")
            page = next_page
        raise Insufficient("分页超过安全上限，未完成采集")


def file_key(row):
    old, new = row.get("old_path"), row.get("new_path")
    require(isinstance(old, str) and old and isinstance(new, str) and new, "变更文件缺少路径")
    return old, new


def format_diff_entry(f):
    lines = [f'diff --git a/{f["old_path"]} b/{f["new_path"]}']
    if f.get("new_file") is True:
        lines.append("new file" + (f' mode {f["b_mode"]}' if f.get("b_mode") else ""))
    if f.get("deleted_file") is True:
        lines.append("deleted file" + (f' mode {f["a_mode"]}' if f.get("a_mode") else ""))
    if f.get("renamed_file") is True or f["old_path"] != f["new_path"]:
        lines.append(f'rename from {f["old_path"]} to {f["new_path"]}')
    if (f.get("a_mode") and f.get("b_mode") and f.get("a_mode") != f.get("b_mode")
            and not f.get("new_file") and not f.get("deleted_file")):
        lines.append(f'mode change {f["a_mode"]} -> {f["b_mode"]}')
    raw = f.get("diff") if isinstance(f.get("diff"), str) and f["diff"].strip() else None
    lines.append(raw if raw is not None else "[FILE_GAP: diff 缺失、为空或被省略，需按固定 SHA 补充文件证据]")
    return "\n".join(lines)


def inspect_files(files):
    require(isinstance(files, list), "缺少变更文件数组")
    seen, gaps = set(), []
    for item in files:
        require(isinstance(item, dict), "变更文件不是对象")
        key = file_key(item)
        require(key not in seen, "变更文件重复")
        seen.add(key)
        reasons = []
        for flag in ("collapsed", "too_large"):
            val = item.get(flag)
            require(val in (None, False, True), f"{key[1]}: {flag} 标记无效")
            if val is True:
                reasons.append(f"{flag}，diff 被省略")
        raw = item.get("diff")
        if not (isinstance(raw, str) and raw.strip()):
            if item.get("renamed_file") is True or key[0] != key[1]:
                reasons.append("仅重命名或 diff 为空，需补取固定 SHA 的文件证据")
            elif item.get("a_mode") != item.get("b_mode"):
                reasons.append("权限变更或 diff 为空，需补取固定 SHA 的文件证据")
            else:
                reasons.append("diff 缺失或为空，需补取固定 SHA 的文件证据")
        if reasons:
            gaps.append({"old_path": key[0], "new_path": key[1], "reason": "；".join(reasons)})
    return gaps


def raise_for_gaps(gaps):
    if gaps:
        first = gaps[0].get("new_path") or gaps[0].get("path", "")
        msg = f'{first}: {gaps[0]["reason"]}'
        if len(gaps) > 1:
            msg += f"（共 {len(gaps)} 个文件存在证据缺口，详见 file_gaps）"
        raise Insufficient(msg)


def mr_snapshot(meta):
    require(isinstance(meta, dict) and isinstance(meta.get("diff_refs"), dict), "MR diff_refs 尚未生成")
    snapshot = {key: sha(meta["diff_refs"].get(key)) for key in ("base_sha", "start_sha", "head_sha")}
    require(sha(meta.get("sha")) == snapshot["head_sha"], "MR sha 与 diff_refs.head_sha 不一致")
    count = meta.get("changes_count")
    require(isinstance(count, (str, int)) and not isinstance(count, bool)
            and re.fullmatch(r"[0-9]+", str(count)), "MR 文件总数未知或已被截断（例如 1000+）")
    return snapshot, int(count)


def latest_version(client, path, snapshot, expected):
    versions, _ = client.get(path + "/versions", {"per_page": 1, "page": 1})
    require(isinstance(versions, list) and len(versions) == 1 and isinstance(versions[0], dict), "缺少最新 diff 版本")
    v = versions[0]
    require(type(v.get("id")) is int and v["id"] > 0 and v.get("state") == "collected", "diff 版本未完整生成或已 overflow")
    for name in ("base", "start", "head"):
        require(sha(v.get(name + "_commit_sha")) == snapshot[name + "_sha"], "diff 版本与 MR SHA 不一致")
    if "real_size" in v:
        require(str(v["real_size"]) == str(expected), "diff 版本文件数量与 MR 不一致")
    return v["id"]


def collect_mr(args, result):
    token = os.environ.get("GITLAB_TOKEN", "")
    host, project, iid = parse_mr(args.project, args.iid, os.environ.get("GITLAB_HOST", "https://gitlab.com"), token)
    client = Client(host, token)
    path = "/api/v4/projects/" + quote(project, safe="") + "/merge_requests/" + iid
    meta, _ = client.get(path)
    snapshot, expected = mr_snapshot(meta)
    result.update(project=project, host=host, snapshot=snapshot, expected_files=expected)
    version = latest_version(client, path, snapshot, expected)
    snapshot["diff_version_id"] = version
    commits = client.pages(path + "/commits", lambda row: sha(row.get("id")))
    result["counts"]["commits"] = len(commits)
    require(expected == 0 or snapshot["head_sha"] in [sha(c["id"]) for c in commits], "MR 提交列表缺少固定 head SHA")
    try:
        files = client.pages(path + "/diffs", file_key)
    except HTTPFailure as exc:
        if exc.status not in (404, 405):
            raise
        fallback, _ = client.get(path + "/changes")
        require(isinstance(fallback, dict) and fallback.get("overflow") is False, "旧版 /changes 未明确确认 overflow=false")
        require(fallback.get("diff_refs") == meta.get("diff_refs"), "旧版 /changes 与固定 SHA 不一致")
        files = fallback.get("changes")
        require(isinstance(files, list), "旧版 /changes 缺少文件数组")
    result["counts"]["files"] = len(files)
    require(len(files) == expected, "实际 diff 文件数与 MR changes_count 不一致")
    gaps = inspect_files(files)
    final_meta, _ = client.get(path)
    final_snapshot, final_count = mr_snapshot(final_meta)
    require(all(final_snapshot[key] == snapshot[key] for key in final_snapshot) and final_count == expected,
            "采集期间 MR SHA/文件数发生变化，必须重新采集")
    require(latest_version(client, path, final_snapshot, final_count) == version, "采集期间 diff 版本发生变化")
    require(all(meta.get(k) == final_meta.get(k) for k in ("title", "description", "state")), "采集期间 MR 声明或状态发生变化")
    result.update(meta=meta, commits=commits, diffs=files)
    if gaps:
        result["file_gaps"] = gaps
    raise_for_gaps(gaps)


def collect_compare(args, result):
    host = host_url(os.environ.get("GITLAB_HOST", "https://gitlab.com"))
    project = valid_project(args.project)
    client = Client(host, os.environ.get("GITLAB_TOKEN", ""))
    path = "/api/v4/projects/" + quote(project, safe="") + "/repository"
    resolved = []
    for ref in (args.from_ref, args.to_ref):
        data, _ = client.get(path + "/commits/" + quote(ref, safe=""))
        require(isinstance(data, dict), "引用解析失败")
        resolved.append(sha(data.get("id")))
    start, head = resolved
    base, _ = client.get(path + "/merge_base", {"refs[]": [start, head]})
    require(isinstance(base, dict), "缺少 merge base")
    result.update(project=project, host=host,
                  snapshot={"start_sha": start, "head_sha": head, "base_sha": sha(base.get("id"))})
    data, _ = client.get(path + "/compare", {"from": start, "to": head, "straight": "false"})
    require(isinstance(data, dict) and data.get("compare_timeout") is False, "compare 超时、超过大小限制或缺少完整性标记")
    require(isinstance(data.get("commit"), dict) and sha(data["commit"].get("id")) == head, "compare 结果不属于固定目标 SHA")
    require(data.get("overflow") is None or data["overflow"] is False, "compare 结果被截断")
    commits, files = data.get("commits"), data.get("diffs")
    require(isinstance(commits, list), "compare 缺少提交数组")
    ids = [sha(row.get("id")) for row in commits if isinstance(row, dict)]
    require(len(ids) == len(commits) and len(set(ids)) == len(ids), "compare 提交记录缺失或重复")
    require(not files or head in ids, "compare 提交列表缺少固定 head SHA")
    gaps = inspect_files(files)
    result.update(commits=commits, diffs=files, counts={"commits": len(commits), "files": len(files)})
    if gaps:
        result["file_gaps"] = gaps
    raise_for_gaps(gaps)


def evidence(result):
    payload = {k: v for k, v in result.items() if k not in ("meta", "commits", "diffs", "context", "context_summary", "file_patches")}
    if "diffs" in result:
        files = []
        for f in result.get("diffs", []):
            raw = f.get("diff") if isinstance(f.get("diff"), str) and f["diff"].strip() else None
            entry = {"old_path": f["old_path"], "new_path": f["new_path"],
                     "patch_sha256": hashlib.sha256(raw.encode()).hexdigest() if raw else None}
            for flag in ("new_file", "deleted_file", "renamed_file"):
                if f.get(flag) is True:
                    entry[flag] = True
            if (f.get("a_mode") or f.get("b_mode")) and (
                f.get("a_mode") != f.get("b_mode") or f.get("new_file") is True or f.get("deleted_file") is True
            ):
                entry["a_mode"] = f.get("a_mode")
                entry["b_mode"] = f.get("b_mode")
            files.append(entry)
        payload["files"] = files
    if "commits" in result:
        payload["commit_ids"] = [c["id"] for c in result.get("commits", [])]
    return payload


def write_atomic_json(destination, payload):
    with tempfile.NamedTemporaryFile(mode="w", dir=destination.parent, delete=False, encoding="utf-8") as f:
        temp = Path(f.name)
        json.dump(payload, f, ensure_ascii=False, indent=2)
        f.write("\n")
    try:
        os.replace(temp, destination)
    finally:
        temp.unlink(missing_ok=True)


def write_patches_dir(result, patches_dir):
    out_dir = Path(patches_dir)
    out_dir.mkdir(parents=True, exist_ok=True)
    entries = []
    if "diffs" in result:
        items = [(f["new_path"], format_diff_entry(f)) for f in result.get("diffs", [])]
    else:
        items = list(result.get("file_patches", []))
    for idx, (path, content) in enumerate(items, 1):
        fname = f"{idx:04d}.diff"
        target = out_dir / fname
        target.write_text(content + ("\n" if not content.endswith("\n") else ""), encoding="utf-8")
        entries.append({"index": idx, "path": path, "patch_file": str(target), "lines": len(content.splitlines())})
    write_atomic_json(out_dir / "index.json", {"snapshot": result.get("snapshot", {}), "files": entries})


def emit(result, output_path, patches_dir=None, summary_only=False):
    payload = evidence(result)
    if output_path:
        write_atomic_json(Path(output_path), payload)
    if patches_dir and ("diffs" in result or "file_patches" in result):
        write_patches_dir(result, patches_dir)
    print("========== COLLECTION STATUS ==========")
    print("evidence_status: " + ("complete" if result["evidence_complete"] else "insufficient（证据不足，不得给出上线通过结论）"))
    print("snapshot: " + json.dumps(result.get("snapshot", {}), ensure_ascii=False))
    for reason in result["reasons"]:
        print("reason: " + reason)
    if "context" in result or "diffs" in result:
        if "context" in result:
            print(result["context_summary"] if summary_only and "context_summary" in result else result["context"])
        if "meta" in result:
            print("========== MR META ==========")
            meta = result["meta"]
            print(json.dumps({k: meta.get(k) for k in ("title", "web_url", "state", "author", "source_branch",
                              "target_branch", "sha", "draft", "created_at", "description")}, ensure_ascii=False, indent=2))
        if "commits" in result:
            print("========== COMMITS ==========")
            for c in result.get("commits", []):
                print(f'{c["id"]} | {c.get("author_name", "")} | {c.get("title", "")}')
        if "diffs" in result:
            print("========== CHANGED FILES & DIFFS ==========")
            if summary_only:
                for f in result.get("diffs", []):
                    print(f'file: {f["old_path"]} -> {f["new_path"]}')
            else:
                for f in result.get("diffs", []):
                    print(format_diff_entry(f))
        if result.get("file_gaps"):
            print("========== FILE EVIDENCE GAPS ==========")
            for g in result["file_gaps"]:
                target = g.get("new_path") or g.get("path", "")
                print(f'gap: {target} -> {g["reason"]}')
    print("EVIDENCE_JSON: " + json.dumps(payload, ensure_ascii=False, separators=(",", ":")))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("mode", choices=("mr", "compare"))
    parser.add_argument("project")
    parser.add_argument("refs", nargs="*")
    parser.add_argument("--evidence-json", help="将机器可读证据写入指定 JSON 文件")
    parser.add_argument("--patches-dir", help="按固定快照将单文件 diff 拆分写入指定目录，供大 MR 分批读取")
    parser.add_argument("--summary-only", action="store_true", help="stdout 仅输出摘要与清单，抑制完整 diff 正文以控制上下文体积")
    args = parser.parse_args()
    result = {"schema_version": 1, "mode": args.mode, "evidence_complete": False,
              "collected_at": datetime.now(timezone.utc).isoformat(), "snapshot": {},
              "counts": {"commits": 0, "files": 0}, "reasons": []}
    try:
        if args.mode == "mr":
            require(len(args.refs) <= 1, "MR 模式只接受 URL 或项目路径加 IID")
            args.iid = args.refs[0] if args.refs else None
            collect_mr(args, result)
        else:
            require(len(args.refs) == 2, "compare 模式需要 from 和 to")
            args.from_ref, args.to_ref = args.refs
            collect_compare(args, result)
        result["evidence_complete"] = True
    except (Insufficient, ValueError) as exc:
        result["reasons"].append(str(exc))
    try:
        emit(result, args.evidence_json, args.patches_dir, args.summary_only)
    except OSError:
        result["evidence_complete"] = False
        result["reasons"].append("证据文件写入失败")
        emit(result, None, None, args.summary_only)
    return 0 if result["evidence_complete"] else 2


if __name__ == "__main__":
    raise SystemExit(main())
