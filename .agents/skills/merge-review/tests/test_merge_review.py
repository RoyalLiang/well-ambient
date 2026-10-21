"""Exercise the public CLIs with offline HTTP responses and temporary Git repos."""
import copy
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch


ROOT = Path(__file__).resolve().parents[1]
BASE, START, HEAD = "a" * 40, "b" * 40, "c" * 40
API = "/api/v4/projects/group%2Fproj"
MR = API + "/merge_requests/7"

FAKE_CURL = r'''
import json, os, sys
from pathlib import Path
from urllib.parse import urlsplit, parse_qs
a = sys.argv[1:]
url = next((x for x in a if x.startswith(('https://', 'http://'))), '')
parts = urlsplit(url)
query = {k:v[-1] for k,v in parse_qs(parts.query).items()}
for i,x in enumerate(a):
    if x == '--data-urlencode':
        k,v = a[i+1].split('=',1); query[k] = v
fixture = json.loads(Path(os.environ['FAKE_HTTP']).read_text())
state_path = Path(os.environ['FAKE_STATE'])
state = json.loads(state_path.read_text()) if state_path.exists() else {}
response = {'status': 599, 'body': {'error':'unmatched offline route'}}
for i,route in enumerate(fixture):
    if route['path'] == parts.path and all(query.get(k) == str(v) for k,v in route.get('query',{}).items()):
        n = state.get(str(i), 0); state[str(i)] = n+1
        options = route['responses']; response = options[min(n,len(options)-1)]; break
state_path.write_text(json.dumps(state))
cfg_text = Path(a[a.index('--config')+1]).read_text() if '--config' in a else ''
with open(os.environ['FAKE_LOG'],'a') as f: f.write(json.dumps({'url':url,'query':query,'argv':a,'cfg':cfg_text})+'\n')
status = response.get('status',200)
body = response.get('raw',json.dumps(response.get('body')))
for flag in ('--dump-header','-D'):
    if flag in a:
        h = 'HTTP/1.1 %s offline\r\n' % status
        h += ''.join('%s: %s\r\n' % (k,v) for k,v in response.get('headers',{}).items())+'\r\n'
        Path(a[a.index(flag)+1]).write_text(h)
if '--output' in a: Path(a[a.index('--output')+1]).write_text(body)
else: sys.stdout.write(body)
if '--write-out' in a: sys.stdout.write(str(status))
sys.exit(response.get('exit',22 if status>=400 and any(x in ('-sf','-f','--fail') for x in a) else 0))
'''


def diff(i=0, **overrides):
    result = {"old_path": f"file{i}.py", "new_path": f"file{i}.py",
              "diff": "@@ -1 +1 @@\n-old\n+new\n", "new_file": False,
              "deleted_file": False, "renamed_file": False,
              "collapsed": False, "too_large": False}
    result.update(overrides)
    return result


def commit(i=0):
    return {"id": HEAD if i == 0 else f"{i:040x}", "short_id": f"{i:08x}",
            "author_name": "fixture", "created_at": "2026-10-21T00:00:00Z", "title": f"change {i}"}


def route(path, body, query=None, headers=None, status=200):
    return {"path": path, "query": query or {},
            "responses": [{"body": body, "headers": headers or {}, "status": status}]}


def mr_routes(files=None, commits=None, pagination=True):
    files = [diff()] if files is None else files
    commits = [commit()] if commits is None else commits
    meta = {"title": "fixture", "description": "fixture change", "sha": HEAD,
            "diff_refs": {"base_sha": BASE, "start_sha": START, "head_sha": HEAD},
            "changes_count": str(len(files)), "state": "opened"}
    version = {"id": 42, "base_commit_sha": BASE, "start_commit_sha": START,
               "head_commit_sha": HEAD, "state": "collected", "real_size": str(len(files))}
    result = [route(MR, meta), route(MR + "/versions", [version])]
    for suffix, rows in (("commits", commits), ("diffs", files)):
        for page in range(1, max(2, (len(rows) + 99) // 100 + 1)):
            chunk = rows[(page-1)*100:page*100]
            headers = {"X-Total": str(len(rows)), "X-Next-Page": str(page+1) if page*100 < len(rows) else ""} if pagination else {}
            result.append(route(MR + "/" + suffix, chunk, {"page": page}, headers))
        result.append(route(MR + "/" + suffix, []))
    return result


class OfflineCLI(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="merge-review-test-")
        self.addCleanup(self.temp.cleanup)
        self.dir = Path(self.temp.name)
        bindir = self.dir / "bin"; bindir.mkdir()
        mock = bindir / "curl"
        mock.write_text(f"#!{sys.executable}\n" + FAKE_CURL)
        mock.chmod(0o755)
        self.env = dict(os.environ, PATH=str(bindir) + os.pathsep + os.environ['PATH'],
                        GITLAB_TOKEN="", GITLAB_HOST="https://gitlab.example.test",
                        PYTHONDONTWRITEBYTECODE="1", FAKE_HTTP=str(self.dir / "http.json"),
                        FAKE_STATE=str(self.dir / "state.json"), FAKE_LOG=str(self.dir / "log.jsonl"))

    def run_cli(self, script, args, routes=None):
        (self.dir / "http.json").write_text(json.dumps(routes or []))
        for name in ("state.json", "log.jsonl"):
            (self.dir / name).unlink(missing_ok=True)
        return subprocess.run(["bash", str(ROOT / "scripts" / script), *args],
                              env=self.env, text=True, errors="replace", capture_output=True, timeout=20)

    def evidence(self, result, expected_code=0):
        self.assertEqual(result.returncode, expected_code, result.stdout + result.stderr)
        rows = [x[len("EVIDENCE_JSON: "):] for x in result.stdout.splitlines() if x.startswith("EVIDENCE_JSON: ")]
        self.assertEqual(len(rows), 1, result.stdout + result.stderr)
        return json.loads(rows[0])

    def run_mr(self, routes=None, url=None):
        return self.run_cli("collect_gitlab_mr.sh", [url or "https://gitlab.example.test/group/proj/-/merge_requests/7"],
                            mr_routes() if routes is None else routes)

    def test_standard_url_and_fixed_snapshot(self):
        e = self.evidence(self.run_mr())
        self.assertTrue(e["evidence_complete"])
        self.assertEqual(e["snapshot"]["head_sha"], HEAD)
        self.assertEqual(e["snapshot"]["base_sha"], BASE)
        self.assertEqual(e["snapshot"]["start_sha"], START)
        self.assertEqual(e["snapshot"]["diff_version_id"], 42)

    def test_nested_url_query_and_fragment(self):
        routes = json.loads(json.dumps(mr_routes()).replace(API, "/api/v4/projects/group%2Fsub%2Fproj"))
        self.evidence(self.run_mr(routes, "http://gitlab.example.test:8080/group/sub/proj/-/merge_requests/7/diffs?foo=1#note"))
        self.assertIn("http://gitlab.example.test:8080/", (self.dir / "log.jsonl").read_text())

    def test_legacy_url(self):
        self.evidence(self.run_mr(url="https://gitlab.example.test/group/proj/merge_requests/7"))

    def test_project_and_iid(self):
        self.evidence(self.run_cli("collect_gitlab_mr.sh", ["group/proj", "7"], mr_routes()))

    def test_invalid_url_never_requests_http(self):
        for url in ("https://gitlab.example.test/group/proj/-/merge_requests/7bad", "https://user:secret@gitlab.example.test/group/proj/-/merge_requests/7"):
            with self.subTest(url=url):
                e = self.evidence(self.run_mr(url=url), 2)
                self.assertFalse(e["evidence_complete"])
        self.assertFalse((self.dir / "log.jsonl").exists())

    def test_paginate_both_commits_and_diffs(self):
        e = self.evidence(self.run_mr(mr_routes([diff(i) for i in range(101)], [commit(i) for i in range(101)])))
        self.assertEqual(e["counts"], {"commits": 101, "files": 101})

    def test_missing_headers_at_exact_page_boundary(self):
        e = self.evidence(self.run_mr(mr_routes([diff(i) for i in range(100)], pagination=False)))
        self.assertEqual(e["counts"]["files"], 100)
        self.assertIn('"page": "2"', (self.dir / "log.jsonl").read_text())

    def test_link_header_continues_short_page(self):
        routes = mr_routes([diff(0), diff(1)])
        rows = next(r for r in routes if r["path"] == MR + "/diffs" and r["query"])
        rows["responses"][0] = {"body": [diff(0)], "headers": {"Link": f'<https://gitlab.example.test{MR}/diffs?page=2&per_page=100>; rel="next"', "X-Total": "2"}}
        routes.insert(0, route(MR + "/diffs", [diff(1)], {"page": 2}, {"X-Total": "2"}))
        self.assertEqual(self.evidence(self.run_mr(routes))["counts"]["files"], 2)

    def test_total_pages_header_continues_without_next_header(self):
        routes = mr_routes([diff(0), diff(1)])
        page1 = next(r for r in routes if r["path"] == MR + "/diffs" and r["query"])
        page1["responses"][0] = {"body": [diff(0)], "headers": {"X-Total-Pages": "2"}}
        routes.insert(0, route(MR + "/diffs", [diff(1)], {"page": 2}, {"X-Total-Pages": "2"}))
        self.assertEqual(self.evidence(self.run_mr(routes))["counts"]["files"], 2)

    def test_bad_pagination_is_insufficient(self):
        for headers in ({"X-Next-Page": "1"}, {"X-Total": "2"},
                        {"Link": '<https://untrusted.test/x?page=2>; rel="next"'}):
            with self.subTest(headers=headers):
                routes = mr_routes()
                next(r for r in routes if r["path"] == MR + "/diffs" and r["query"])["responses"][0]["headers"] = headers
                e = self.evidence(self.run_mr(routes), 2)
                self.assertFalse(e["evidence_complete"])

    def test_duplicate_page_is_insufficient(self):
        routes = mr_routes([diff(i) for i in range(101)])
        next(r for r in routes if r["path"] == MR + "/diffs" and r["query"] == {"page": 2})["responses"][0]["body"] = [diff(0)]
        self.assertFalse(self.evidence(self.run_mr(routes), 2)["evidence_complete"])

    def test_omitted_content_is_insufficient(self):
        for overrides in ({"too_large": True}, {"collapsed": True}, {"diff": ""}, {"diff": None}):
            with self.subTest(overrides=overrides):
                e = self.evidence(self.run_mr(mr_routes([diff(**overrides)])), 2)
                self.assertFalse(e["evidence_complete"])

    def test_count_and_metadata_must_be_verifiable(self):
        for field, value in (("changes_count", "1000+"), ("changes_count", "2"), ("diff_refs", {})):
            with self.subTest(field=field):
                routes = mr_routes(); routes[0]["responses"][0]["body"][field] = value
                self.assertFalse(self.evidence(self.run_mr(routes), 2)["evidence_complete"])

    def test_version_overflow_is_insufficient(self):
        routes = mr_routes(); routes[1]["responses"][0]["body"][0]["state"] = "overflow"
        self.assertFalse(self.evidence(self.run_mr(routes), 2)["evidence_complete"])

    def test_moving_head_or_version_is_insufficient(self):
        for change in ("head", "version"):
            with self.subTest(change=change):
                routes = mr_routes()
                entry = routes[0 if change == "head" else 1]
                updated = copy.deepcopy(entry["responses"][0])
                if change == "head":
                    updated["body"]["sha"] = "d" * 40
                    updated["body"]["diff_refs"]["head_sha"] = "d" * 40
                else:
                    updated["body"][0]["id"] = 43
                entry["responses"].append(updated)
                self.assertFalse(self.evidence(self.run_mr(routes), 2)["evidence_complete"])

    def test_legacy_fallback_checks_overflow(self):
        for overflow in (False, True):
            with self.subTest(overflow=overflow):
                routes = mr_routes()
                next(r for r in routes if r["path"] == MR + "/diffs" and r["query"])["responses"][0]["status"] = 404
                routes.append(route(MR + "/changes", {"changes": [diff()], "overflow": overflow, "diff_refs": routes[0]["responses"][0]["body"]["diff_refs"]}))
                self.evidence(self.run_mr(routes), 2 if overflow else 0)

    def test_http_failure_and_malformed_json_are_insufficient(self):
        for status, raw in ((403, '{}'), (500, '{}'), (200, '<html>bad</html>')):
            with self.subTest(status=status, raw=raw):
                routes = mr_routes(); routes[0]["responses"][0].update(status=status, raw=raw)
                self.assertFalse(self.evidence(self.run_mr(routes), 2)["evidence_complete"])

    def test_evidence_json_file_matches_stdout(self):
        path = self.dir / "evidence.json"
        r = self.run_cli("collect_gitlab_mr.sh", ["group/proj", "7", "--evidence-json", str(path)], mr_routes())
        self.assertEqual(self.evidence(r), json.loads(path.read_text()))

    def test_failed_evidence_output_cannot_report_success(self):
        r = self.run_cli("collect_gitlab_mr.sh", ["group/proj", "7", "--evidence-json", str(self.dir / "missing" / "e.json")], mr_routes())
        self.assertFalse(self.evidence(r, 2)["evidence_complete"])

    def test_second_page_failure_is_not_a_partial_success(self):
        routes = mr_routes([diff(i) for i in range(101)])
        next(r for r in routes if r["path"] == MR + "/diffs" and r["query"] == {"page": 2})["responses"][0]["status"] = 500
        self.assertFalse(self.evidence(self.run_mr(routes), 2)["evidence_complete"])

    def test_missing_head_commit_is_insufficient(self):
        self.assertFalse(self.evidence(self.run_mr(mr_routes(commits=[])), 2)["evidence_complete"])

    def test_no_fallback_for_permissions_or_transport_failure(self):
        for failure in ({"status": 403}, {"status": 200, "exit": 28}, {"status": 302}):
            with self.subTest(failure=failure):
                routes = mr_routes()
                next(r for r in routes if r["path"] == MR + "/diffs" and r["query"])["responses"][0].update(failure)
                self.evidence(self.run_mr(routes), 2)
                self.assertNotIn("/changes", (self.dir / "log.jsonl").read_text())

    def test_legacy_fallback_requires_matching_sha(self):
        routes = mr_routes()
        next(r for r in routes if r["path"] == MR + "/diffs" and r["query"])["responses"][0]["status"] = 404
        routes.append(route(MR + "/changes", {"changes": [diff()], "overflow": False, "diff_refs": {"head_sha": BASE}}))
        self.assertFalse(self.evidence(self.run_mr(routes), 2)["evidence_complete"])

    def test_empty_change_is_a_complete_collection_not_a_release_verdict(self):
        r = self.run_mr(mr_routes(files=[], commits=[]))
        e = self.evidence(r)
        self.assertTrue(e["evidence_complete"])
        self.assertNotIn("release_passed", e)

    def compare_routes(self, **extra):
        body = {"commit": commit(), "commits": [commit()], "diffs": [diff()], "compare_timeout": False}
        body.update(extra)
        return [route(API + "/repository/commits/main", {"id": START}),
                route(API + "/repository/commits/feature%2Fx", {"id": HEAD}),
                route(API + "/repository/merge_base", {"id": BASE}),
                route(API + "/repository/compare", body)]

    def test_compare_uses_only_resolved_shas(self):
        e = self.evidence(self.run_cli("collect_gitlab_compare.sh", ["group/proj", "main", "feature/x"], self.compare_routes()))
        self.assertEqual(e["snapshot"]["head_sha"], HEAD)
        logs = [json.loads(s) for s in (self.dir / "log.jsonl").read_text().splitlines()]
        cmp = next(x for x in logs if "/repository/compare" in x["url"])
        self.assertEqual(cmp["query"]["from"], START)
        self.assertEqual(cmp["query"]["to"], HEAD)

    def test_compare_limits_are_insufficient(self):
        for extra in ({"compare_timeout": True}, {"compare_timeout": None}, {"diffs": [diff(too_large=True)]}, {"commit": {"id": BASE}}):
            with self.subTest(extra=extra):
                e = self.evidence(self.run_cli("collect_gitlab_compare.sh", ["group/proj", "main", "feature/x"], self.compare_routes(**extra)), 2)
                self.assertFalse(e["evidence_complete"])

    def test_token_never_sent_to_mismatched_host_or_http(self):
        self.env["GITLAB_TOKEN"] = "SECRET123"
        for url in ("https://evil.test/group/proj/-/merge_requests/7",
                    "http://gitlab.example.test/group/proj/-/merge_requests/7",
                    "https://gitlab.example.test:8443/group/proj/-/merge_requests/7"):
            with self.subTest(url=url):
                e = self.evidence(self.run_mr(url=url), 2)
                self.assertFalse(e["evidence_complete"])
        self.assertFalse((self.dir / "log.jsonl").exists())

    def test_token_passed_via_config_not_argv(self):
        self.env["GITLAB_TOKEN"] = "SECRET123"
        e = self.evidence(self.run_mr())
        self.assertTrue(e["evidence_complete"])
        logs = [json.loads(s) for s in (self.dir / "log.jsonl").read_text().splitlines()]
        self.assertTrue(logs)
        for row in logs:
            self.assertNotIn("SECRET123", " ".join(row["argv"]))
            self.assertIn("PRIVATE-TOKEN: SECRET123", row["cfg"])

    def test_partial_file_gap_preserves_other_diffs_and_metadata(self):
        files = [diff(0, new_file=True, a_mode="0", b_mode="100755"),
                 diff(1, new_path="img.bin", old_path="img.bin", diff="")]
        r = self.run_mr(mr_routes(files))
        e = self.evidence(r, 2)
        self.assertFalse(e["evidence_complete"])
        self.assertEqual(e["counts"]["files"], 2)
        self.assertEqual(len(e["file_gaps"]), 1)
        self.assertEqual(e["file_gaps"][0]["new_path"], "img.bin")
        self.assertTrue(e["files"][0]["new_file"])
        self.assertEqual(e["files"][0]["b_mode"], "100755")
        self.assertIn("new file mode 100755", r.stdout)
        self.assertIn("+new", r.stdout)
        self.assertIn("========== FILE EVIDENCE GAPS ==========", r.stdout)

    def test_summary_only_and_patches_dir_prevent_stdout_bloat(self):
        pdir = self.dir / "patches"
        r = self.run_cli("collect_gitlab_mr.sh",
                         ["group/proj", "7", "--summary-only", "--patches-dir", str(pdir)],
                         mr_routes([diff(0), diff(1)]))
        e = self.evidence(r, 0)
        self.assertEqual(e["counts"]["files"], 2)
        self.assertNotIn("@@ -1 +1 @@", r.stdout)
        index = json.loads((pdir / "index.json").read_text())
        self.assertEqual(index["snapshot"]["head_sha"], HEAD)
        self.assertEqual(len(index["files"]), 2)
        self.assertIn("+new", (pdir / "0001.diff").read_text())


class DecisionCLI(unittest.TestCase):
    def decide(self, findings=(), **extra):
        payload = {"evidence_complete": True, "review_complete": True, "findings": list(findings)}
        payload.update(extra)
        r = subprocess.run([sys.executable, str(ROOT / "scripts" / "review_decision.py"), "-"],
                           input=json.dumps(payload), text=True, capture_output=True)
        self.assertTrue(r.stdout, r.stderr)
        result = json.loads(r.stdout)
        expected = 2 if result["conclusion"] == "证据不足" else (0 if result["release_passed"] else 1)
        self.assertEqual(r.returncode, expected, r.stderr)
        return result

    def test_no_p0_p1_passes(self):
        self.assertEqual(self.decide()["conclusion"], "通过")

    def test_p0_must_be_fixed_even_with_explanation(self):
        f = {"severity": "P0", "status": "open", "justification": {"reason": "defer", "reasonable": True}}
        self.assertEqual(self.decide([f])["conclusion"], "不通过")

    def test_p1_without_reason_blocks(self):
        self.assertEqual(self.decide([{"severity": "P1", "status": "open"}])["conclusion"], "不通过")

    def test_reasonable_p1_is_conditional(self):
        f = {"severity": "P1", "status": "open", "justification": {"reason": "only affects disabled feature", "risk": "feature unavailable", "mitigation": "keep disabled", "reasonable": True}}
        result = self.decide([f])
        self.assertEqual(result["conclusion"], "有条件通过")
        self.assertEqual(result["counts"]["P1"]["explained"], 1)

    def test_unreviewed_or_empty_p1_reason_blocks(self):
        for j in ({"reason": "later"}, {"reason": " ", "risk": "r", "mitigation": "m", "reasonable": True},
                  {"reason": "r", "risk": "r", "mitigation": "m", "reasonable": "true"}):
            with self.subTest(j=j):
                self.assertEqual(self.decide([{"severity": "P1", "status": "open", "justification": j}])["conclusion"], "不通过")

    def test_p2_is_risk_reference(self):
        r = self.decide([{"severity": "P2", "status": "open"}])
        self.assertEqual(r["conclusion"], "通过")
        self.assertEqual(r["counts"]["P2"]["unresolved"], 1)

    def test_fixed_requires_verification(self):
        for verified, expected in ((False, "不通过"), (True, "通过")):
            with self.subTest(verified=verified):
                self.assertEqual(self.decide([{"severity": "P0", "status": "fixed", "verified": verified}])["conclusion"], expected)

    def test_insufficient_evidence_or_review_cannot_pass(self):
        for extra in ({"evidence_complete": False}, {"review_complete": False}, {"evidence_complete": "true"}):
            with self.subTest(extra=extra):
                r = self.decide(**extra)
                self.assertEqual(r["conclusion"], "证据不足")
                self.assertFalse(r["release_passed"])

    def test_invalid_finding_cannot_disappear(self):
        for severity in ("P9", [], None):
            with self.subTest(severity=severity):
                r = self.decide([{"severity": severity, "status": "open"}])
                self.assertEqual(r["conclusion"], "证据不足")
                self.assertFalse(r["counts_complete"])

    def test_mixed_counts_use_unresolved_findings(self):
        r = self.decide([{"severity": "P0", "status": "fixed", "verified": True},
                         {"severity": "P1", "status": "open"},
                         {"severity": "P2", "status": "open"}])
        self.assertEqual(r["counts"]["P0"], {"total": 1, "unresolved": 0})
        self.assertEqual(r["counts"]["P1"]["unexplained"], 1)
        self.assertEqual(r["conclusion"], "不通过")

    def test_malformed_input_fails_closed(self):
        r = subprocess.run([sys.executable, str(ROOT / "scripts/review_decision.py"), "-"],
                           input="broken json", text=True, capture_output=True)
        self.assertEqual(r.returncode, 2)
        self.assertFalse(json.loads(r.stdout)["release_passed"])

    def test_evidence_binding_and_file_gap_supplementation(self):
        ev = {"evidence_complete": False, "snapshot": {"base_sha": BASE, "head_sha": HEAD},
              "files": [{"old_path": "a.py", "new_path": "a.py"}, {"old_path": "b.bin", "new_path": "b.bin"}],
              "file_gaps": [{"old_path": "b.bin", "new_path": "b.bin", "reason": "binary"}]}
        self.assertEqual(self.decide(evidence=ev)["conclusion"], "证据不足")
        bad_sup = [{"path": "b.bin", "sha": "d" * 40, "evidence": "hash", "verified": True}]
        self.assertEqual(self.decide(evidence=ev, supplemented_files=bad_sup)["conclusion"], "证据不足")
        good_sup = [{"path": "b.bin", "sha": HEAD, "evidence": "verified blob sha256", "verified": True}]
        res = self.decide(evidence=ev, supplemented_files=good_sup, reviewed_files=["a.py", "b.bin"])
        self.assertEqual(res["conclusion"], "通过")
        self.assertEqual(res["snapshot"]["head_sha"], HEAD)

    def test_coverage_duplicate_id_sha_mismatch_and_batch_guards(self):
        ev = {"evidence_complete": True, "snapshot": {"base_sha": BASE, "head_sha": HEAD},
              "files": [{"old_path": "a.py", "new_path": "a.py"}, {"old_path": "b.py", "new_path": "b.py"}]}
        self.assertEqual(self.decide(evidence=ev, reviewed_files=["a.py"])["conclusion"], "证据不足")
        self.assertEqual(self.decide(evidence=ev, snapshot={"base_sha": BASE, "head_sha": "d" * 40})["conclusion"], "证据不足")
        dup = [{"id": "F-1", "severity": "P2", "status": "open"}, {"id": "F-1", "severity": "P2", "status": "open"}]
        self.assertEqual(self.decide(dup, evidence=ev)["conclusion"], "证据不足")
        wrong_sha = [{"id": "F-1", "severity": "P2", "status": "open", "sha": "d" * 40}]
        self.assertEqual(self.decide(wrong_sha, evidence=ev)["conclusion"], "证据不足")
        batch = [{"evidence_complete": True, "review_complete": True, "head_sha": HEAD},
                 {"evidence_complete": False, "review_complete": True, "head_sha": BASE}]
        self.assertEqual(self.decide(targets=batch)["conclusion"], "证据不足")


class LocalCLI(unittest.TestCase):
    def setUp(self):
        isolation = patch.dict(os.environ, {"GIT_CONFIG_NOSYSTEM": "1", "GIT_CONFIG_GLOBAL": os.devnull,
                                           "GIT_CONFIG_COUNT": "0"})
        isolation.start()
        self.addCleanup(isolation.stop)

    def test_moving_branch_does_not_change_the_resolved_snapshot(self):
        with tempfile.TemporaryDirectory(prefix="merge-review-moving-") as d:
            real_git = shutil.which("git")
            def git(*args):
                return subprocess.check_output([real_git, "-C", d, "-c", "user.name=fixture", "-c", "user.email=fixture@example.test", *args], text=True).strip()
            git("init", "-q", "-b", "main")
            Path(d, "base").write_text("base\n"); git("add", "."); git("commit", "-qm", "base")
            git("checkout", "-qb", "feature")
            Path(d, "first").write_text("first\n"); git("add", "."); git("commit", "-qm", "first")
            first = git("rev-parse", "HEAD")
            Path(d, "future-only").write_text("future\n"); git("add", "."); git("commit", "-qm", "future")
            future = git("rev-parse", "HEAD")
            git("update-ref", "refs/heads/feature", first)
            bindir = Path(d, "mock-bin"); bindir.mkdir()
            mock = bindir / "git"
            mock.write_text(f'''#!{sys.executable}
import subprocess, sys
real = {real_git!r}
p = subprocess.run([real, *sys.argv[1:]], capture_output=True)
if p.returncode == 0 and 'rev-parse' in sys.argv and 'feature^{{commit}}' in sys.argv:
    subprocess.check_call([real, '-C', {d!r}, 'update-ref', 'refs/heads/feature', {future!r}])
sys.stdout.buffer.write(p.stdout); sys.stderr.buffer.write(p.stderr)
sys.exit(p.returncode)
''')
            mock.chmod(0o755)
            env = dict(os.environ, PATH=str(bindir) + os.pathsep + os.environ['PATH'], PYTHONDONTWRITEBYTECODE="1")
            r = subprocess.run(["bash", str(ROOT / "scripts/collect_range_context.sh"), d, "main", "feature"], env=env, text=True, capture_output=True)
            self.assertEqual(r.returncode, 0, r.stderr)
            e = json.loads(next(x[len("EVIDENCE_JSON: "):] for x in r.stdout.splitlines() if x.startswith("EVIDENCE_JSON: ")))
            self.assertEqual(e["snapshot"]["head_sha"], first)
            self.assertEqual(git("rev-parse", "feature"), future)
            self.assertNotIn("future-only", r.stdout)

    def test_binary_changes_need_additional_evidence(self):
        with tempfile.TemporaryDirectory(prefix="merge-review-binary-") as d:
            def git(*args):
                return subprocess.check_output(["git", "-C", d, "-c", "user.name=fixture", "-c", "user.email=fixture@example.test", *args], text=True).strip()
            git("init", "-q", "-b", "main")
            Path(d, "base").write_text("base\n")
            Path(d, "script.sh").write_text("#!/bin/sh\necho hi\n")
            git("add", "."); git("commit", "-qm", "base")
            git("checkout", "-qb", "feature")
            Path(d, "base").write_text("base\nchanged\n")
            Path(d, "script.sh").chmod(0o755)
            Path(d, "binary").write_bytes(b"\0binary\0")
            git("add", "."); git("commit", "-qm", "binary-and-mode")
            r = subprocess.run(["bash", str(ROOT / "scripts/collect_range_context.sh"), d, "main", "feature"], text=True, capture_output=True)
            self.assertEqual(r.returncode, 2, r.stderr)
            e = json.loads(next(x[len("EVIDENCE_JSON: "):] for x in r.stdout.splitlines() if x.startswith("EVIDENCE_JSON: ")))
            self.assertFalse(e["evidence_complete"])
            self.assertEqual(e["counts"]["files"], 3)
            self.assertEqual(len(e["file_gaps"]), 2)
            self.assertIn("+changed", r.stdout)
            self.assertIn("========== FILE EVIDENCE GAPS ==========", r.stdout)

    def test_non_merge_target_is_insufficient(self):
        with tempfile.TemporaryDirectory(prefix="merge-review-nonmerge-") as d:
            subprocess.check_call(["git", "-C", d, "init", "-q", "-b", "main"])
            subprocess.check_call(["git", "-C", d, "-c", "user.name=fixture", "-c", "user.email=fixture@example.test", "commit", "--allow-empty", "-qm", "initial"])
            r = subprocess.run(["bash", str(ROOT / "scripts/collect_merge_context.sh"), d, "HEAD"], text=True, capture_output=True)
            self.assertEqual(r.returncode, 2, r.stderr)
            e = json.loads(next(x[len("EVIDENCE_JSON: "):] for x in r.stdout.splitlines() if x.startswith("EVIDENCE_JSON: ")))
            self.assertFalse(e["evidence_complete"])

    def test_range_records_immutable_shas_and_merge_base(self):
        with tempfile.TemporaryDirectory(prefix="merge-review-git-") as d:
            def git(*args):
                return subprocess.check_output(["git", "-C", d, "-c", "user.name=fixture", "-c", "user.email=fixture@example.test", *args], text=True).strip()
            git("init", "-q", "-b", "main")
            Path(d, "base.txt").write_text("base\n"); git("add", "."); git("commit", "-qm", "base")
            base = git("rev-parse", "HEAD")
            git("checkout", "-qb", "feature")
            Path(d, "feature.txt").write_text("feature\n"); git("add", "."); git("commit", "-qm", "feature")
            head = git("rev-parse", "HEAD")
            git("checkout", "-q", "main")
            Path(d, "main-only.txt").write_text("main\n"); git("add", "."); git("commit", "-qm", "main")
            start = git("rev-parse", "HEAD")
            r = subprocess.run(["bash", str(ROOT / "scripts/collect_range_context.sh"), d, "main", "feature"], text=True, capture_output=True)
            self.assertEqual(r.returncode, 0, r.stderr)
            line = next((s for s in r.stdout.splitlines() if s.startswith("EVIDENCE_JSON: ")), None)
            self.assertIsNotNone(line, r.stdout)
            e = json.loads(line[len("EVIDENCE_JSON: "):])
            self.assertEqual(e["snapshot"], {"base_sha": base, "start_sha": start, "head_sha": head})
            self.assertEqual(len(e["commit_ids"]), e["counts"]["commits"])
            self.assertEqual(len(e["paths"]), e["counts"]["files"])
            self.assertNotIn("files", e)  # Local manifest uses paths + whole-patch hash.
            self.assertNotIn("main-only.txt", r.stdout)
            git("merge", "--no-ff", "-qm", "merge", "feature")
            merged = git("rev-parse", "HEAD")
            r = subprocess.run(["bash", str(ROOT / "scripts/collect_merge_context.sh"), d, "HEAD"], text=True, capture_output=True)
            self.assertEqual(r.returncode, 0, r.stderr)
            line = next((s for s in r.stdout.splitlines() if s.startswith("EVIDENCE_JSON: ")), None)
            self.assertIsNotNone(line, r.stdout)
            self.assertEqual(json.loads(line[len("EVIDENCE_JSON: "):])["snapshot"]["head_sha"], merged)


if __name__ == "__main__":
    unittest.main()
