#!/usr/bin/env python3
"""Collect local review context from immutable Git objects."""
import argparse
from datetime import datetime, timezone
import hashlib
import subprocess

from gitlab_context import Insufficient, emit, raise_for_gaps, require, sha


def collect(args, result):
    def git(*command):
        try:
            p = subprocess.run(["git", "-C", args.repo, "--no-pager", *command],
                               capture_output=True, text=True, encoding="utf-8")
        except (OSError, UnicodeError) as exc:
            raise Insufficient("git 不可用或内容无法无损解码为 UTF-8，需补充独立证据") from exc
        require(p.returncode == 0, "Git 命令失败，不能确认评审对象完整性: " + command[0])
        return p.stdout

    git("rev-parse", "--is-inside-work-tree")
    if args.mode == "merge":
        require(len(args.refs) <= 1, "merge 模式只接受一个提交引用")
        ref = args.refs[0] if args.refs else git("log", "--merges", "-1", "--format=%H").strip()
        require(ref, "仓库中没有 merge 提交")
        head = sha(git("rev-parse", "--verify", "--end-of-options", ref + "^{commit}").strip())
        parents = git("rev-list", "--parents", "-n", "1", head).split()[1:]
        require(len(parents) == 2, "需要恰好两个父提交的 merge；普通提交或多父 merge 不在当前采集范围")
        start, source = parents
        snapshot = {"base_sha": start, "start_sha": start, "head_sha": head, "source_sha": source}
        log_range, diff_range = start + ".." + source, [start, head]
        meta = git("log", "-1", "--format=hash: %H%nsubject: %s%nparents: %P", head)
    else:
        require(len(args.refs) == 2, "range 模式需要 from 和 to")
        start, head = [sha(git("rev-parse", "--verify", "--end-of-options", ref + "^{commit}").strip()) for ref in args.refs]
        base = sha(git("merge-base", start, head).strip())
        snapshot = {"base_sha": base, "start_sha": start, "head_sha": head}
        log_range, diff_range = start + ".." + head, [base, head]
        meta = "range: " + start + ".." + head + "\nmerge_base: " + base + "\n"
    result["snapshot"] = snapshot
    diff_args = ["diff", "--no-ext-diff", "--no-textconv", "--no-color"]
    files = git(*diff_args, "--name-only", "-z", *diff_range, "--").split("\0")
    files = [name for name in files if name]
    numstat = git(*diff_args, "--numstat", *diff_range, "--")
    gaps = []
    for line in numstat.splitlines():
        parts = line.split("\t", 2)
        if len(parts) == 3 and parts[:2] == ["-", "-"]:
            gaps.append({"path": parts[2], "reason": "包含二进制变更，需补充固定 SHA 的独立证据"})
        elif len(parts) == 3 and parts[:2] == ["0", "0"]:
            gaps.append({"path": parts[2], "reason": "空 diff（仅重命名、仅权限变更或空文件），需补充固定 SHA 的独立证据"})
    patch = git(*diff_args, *diff_range, "--")
    stat = git(*diff_args, "--stat", *diff_range, "--")
    commits = git("log", "--format=%H | %an | %ad | %s", "--date=short", log_range, "--")
    count = int(git("rev-list", "--count", log_range, "--").strip())
    summary = meta + "========== COMMITS ==========\n" + commits + "========== DIFFSTAT ==========\n" + stat
    result.update(counts={"commits": count, "files": len(files)}, paths=files,
                  commit_ids=[sha(line.split(" | ", 1)[0]) for line in commits.splitlines()],
                  diff_sha256=hashlib.sha256(patch.encode()).hexdigest(),
                  context_summary=summary,
                  context=summary + "========== FULL DIFF ==========\n" + patch)
    if args.patches_dir:
        result["file_patches"] = [(p, git(*diff_args, *diff_range, "--", p)) for p in files]
    if gaps:
        result["file_gaps"] = gaps
    raise_for_gaps(gaps)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("mode", choices=("merge", "range"))
    parser.add_argument("repo")
    parser.add_argument("refs", nargs="*")
    parser.add_argument("--evidence-json")
    parser.add_argument("--patches-dir")
    parser.add_argument("--summary-only", action="store_true")
    args = parser.parse_args()
    result = {"schema_version": 1, "mode": args.mode, "evidence_complete": False,
              "collected_at": datetime.now(timezone.utc).isoformat(), "snapshot": {},
              "counts": {"commits": 0, "files": 0}, "reasons": []}
    try:
        collect(args, result)
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
