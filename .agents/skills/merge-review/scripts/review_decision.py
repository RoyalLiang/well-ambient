#!/usr/bin/env python3
"""Apply P0/P1/P2 release review policy to reviewed findings; does not deploy."""
import argparse
import json
import re
import sys


def is_sha(value):
    return isinstance(value, str) and bool(re.fullmatch(r"[0-9a-fA-F]{40}|[0-9a-fA-F]{64}", value))


def valid_snapshot(snap):
    return (isinstance(snap, dict) and is_sha(snap.get("base_sha")) and is_sha(snap.get("head_sha"))
            and all(is_sha(v) for k, v in snap.items() if k.endswith("_sha")))


def check_evidence_item(ev, data, problems, snapshots, known_shas):
    if not isinstance(ev, dict) or not valid_snapshot(ev.get("snapshot")):
        problems.append("证据文件缺少合法的固定 SHA 快照")
        return
    snap = {k: v.lower() if isinstance(v, str) else v for k, v in ev["snapshot"].items()}
    snapshots.append(snap)
    for k, v in snap.items():
        if k.endswith("_sha"):
            known_shas.add(v)
    supplemented = {}
    for item in (data.get("supplemented_files") or []) + (ev.get("supplemented_files") or []):
        if isinstance(item, dict) and isinstance(item.get("path"), str) and item["path"].strip():
            supplemented[item["path"]] = item
    if ev.get("evidence_complete") is True:
        pass
    elif ("files" in ev or "paths" in ev) and isinstance(ev.get("file_gaps"), list) and ev["file_gaps"]:
        for gap in ev["file_gaps"]:
            gpath = (gap.get("new_path") or gap.get("path", "")) if isinstance(gap, dict) else ""
            sup = supplemented.get(gpath)
            ref_sha = (sup.get("sha") or sup.get("head_sha") or "").lower() if isinstance(sup, dict) else ""
            if not (isinstance(sup, dict) and sup.get("verified") is True
                    and isinstance(sup.get("evidence"), str) and sup["evidence"].strip()
                    and ref_sha in (snap["head_sha"], snap["base_sha"])):
                problems.append(f"文件证据缺口未按固定 SHA 补齐: {gpath or 'unknown'}")
    else:
        problems.append("采集证据不完整或未确认")
    if "reviewed_files" in data:
        reviewed = data.get("reviewed_files")
        expected = [f.get("new_path") for f in ev.get("files", []) if isinstance(f, dict)] if "files" in ev else list(ev.get("paths") or [])
        if not isinstance(reviewed, list) or any(p not in reviewed for p in expected):
            problems.append("存在未完成评审覆盖的文件")


def decide(data, evidence_items=None):
    counts = {p: {"total": 0, "unresolved": 0} for p in ("P0", "P1", "P2")}
    counts["P1"].update(explained=0, unexplained=0)
    problems, snapshots, known_shas = [], [], set()
    if not isinstance(data, dict):
        data = {}
        problems.append("输入必须是对象")
    attached = list(evidence_items or [])
    if "evidence" in data:
        raw_ev = data["evidence"]
        attached.extend(raw_ev if isinstance(raw_ev, list) else [raw_ev])
    if attached:
        for ev in attached:
            check_evidence_item(ev, data, problems, snapshots, known_shas)
        if data.get("evidence_complete") is False and not any("缺口" in p or "不完整" in p for p in problems):
            problems.append("采集证据不完整或未确认")
    elif data.get("evidence_complete") is not True:
        problems.append("采集证据不完整或未确认")
    if data.get("review_complete") is not True:
        problems.append("评审覆盖尚未完成或未确认")
    if "snapshot" in data:
        if not valid_snapshot(data["snapshot"]):
            problems.append("评审输入的固定 SHA 快照无效")
        else:
            input_snap = {k: v.lower() if isinstance(v, str) else v for k, v in data["snapshot"].items()}
            for k, v in input_snap.items():
                if k.endswith("_sha"):
                    known_shas.add(v)
            if len(snapshots) == 1 and any(
                input_snap.get(k) != snapshots[0].get(k) for k in ("base_sha", "head_sha")
            ):
                problems.append("评审输入与证据文件的固定 SHA 不一致")
            if not snapshots:
                snapshots.append(input_snap)
    if "targets" in data:
        targets = data.get("targets")
        if not isinstance(targets, list) or not targets:
            problems.append("批量评审 targets 必须是非空数组")
        else:
            for t in targets:
                if not isinstance(t, dict) or t.get("evidence_complete") is not True or t.get("review_complete") is not True:
                    problems.append("批量评审存在证据不足或未完成覆盖的对象")
                    break
                if "head_sha" in t:
                    if not is_sha(t["head_sha"]):
                        problems.append("批量评审对象缺少完整 head_sha")
                        break
                    known_shas.add(t["head_sha"].lower())
    findings = data.get("findings")
    if not isinstance(findings, list):
        problems.append("findings 必须是数组")
        findings = []
    seen_ids = set()
    for f in findings:
        if not isinstance(f, dict) or not isinstance(f.get("severity"), str) or f["severity"] not in counts or f.get("status") not in ("open", "fixed"):
            problems.append("问题缺少合法 P0/P1/P2 级别或 open/fixed 状态")
            continue
        if "id" in f:
            fid = f.get("id")
            if not isinstance(fid, str) or not fid.strip() or fid in seen_ids:
                problems.append("发现项 ID 缺失或重复")
                continue
            seen_ids.add(fid)
        for sha_key in ("sha", "head_sha"):
            if sha_key in f:
                val = f.get(sha_key)
                if not is_sha(val) or (known_shas and val.lower() not in known_shas):
                    problems.append("发现项引用了非当前固定快照的 SHA")
        level = f["severity"]
        counts[level]["total"] += 1
        if f["status"] == "fixed" and f.get("verified") is True:
            continue
        counts[level]["unresolved"] += 1
        if level == "P1":
            j = f.get("justification")
            explained = isinstance(j, dict) and j.get("reasonable") is True and all(
                isinstance(j.get(key), str) and j[key].strip() for key in ("reason", "risk", "mitigation"))
            counts[level]["explained" if explained else "unexplained"] += 1
    if problems:
        conclusion = "证据不足"
    elif counts["P0"]["unresolved"] or counts["P1"]["unexplained"]:
        conclusion = "不通过"
    elif counts["P1"]["explained"]:
        conclusion = "有条件通过"
    else:
        conclusion = "通过"
    out = {"conclusion": conclusion, "release_passed": conclusion in ("通过", "有条件通过"),
           "counts": counts, "counts_complete": not problems, "evidence_gaps": problems,
           "blocking_p0": counts["P0"]["unresolved"], "blocking_p1": counts["P1"]["unexplained"]}
    if len(snapshots) == 1:
        out["snapshot"] = snapshots[0]
    elif len(snapshots) > 1:
        out["snapshots"] = snapshots
    return out


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("input", help="判定输入 JSON 文件，或 - 从 stdin 读取")
    parser.add_argument("--evidence", action="append", default=[], help="绑定采集生成的证据 JSON 文件（批量可重复指定）")
    args = parser.parse_args()
    try:
        if args.input == "-":
            data = json.load(sys.stdin)
        else:
            with open(args.input, encoding="utf-8") as f:
                data = json.load(f)
        evidence_items = []
        for path in args.evidence:
            with open(path, encoding="utf-8") as f:
                evidence_items.append(json.load(f))
    except (OSError, ValueError):
        print(json.dumps({"conclusion": "证据不足", "release_passed": False,
                          "evidence_gaps": ["判定输入或证据文件读取失败或不是合法 JSON"]}, ensure_ascii=False))
        return 2
    result = decide(data, evidence_items)
    print(json.dumps(result, ensure_ascii=False, indent=2))
    return 2 if result["conclusion"] == "证据不足" else (0 if result["release_passed"] else 1)


if __name__ == "__main__":
    raise SystemExit(main())
