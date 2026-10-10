"""Run the real Make/release chain with a Docker recorder, without building images."""

from concurrent.futures import ThreadPoolExecutor
import importlib.util
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tempfile
import unittest


PROJECT = Path(__file__).resolve().parents[2]
spec = importlib.util.spec_from_file_location("docker_images", PROJECT / "scripts/docker-images.py")
builder = importlib.util.module_from_spec(spec)
spec.loader.exec_module(builder)


class DockerImagesTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        (self.root / "scripts").mkdir()
        for path in ("Makefile", "scripts/release-metadata.sh", "scripts/docker-images.py"):
            shutil.copy2(PROJECT / path, self.root / path)
        (self.root / ".gitignore").write_text("deploy/generated/\n")
        for arguments in (("init", "--quiet"), ("checkout", "--quiet", "-b", "Feature/a_b.c"),
                          ("add", "."), ("-c", "user.name=Build Test", "-c", "user.email=build-test@example.invalid", "commit", "--quiet", "-m", "fixture")):
            subprocess.run(["git", *arguments], cwd=self.root, check=True, capture_output=True)
        self.commit = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=self.root, text=True).strip()
        binary = self.root / "bin"
        binary.mkdir()
        docker = binary / "docker"
        docker.write_text(f"#!{sys.executable}\n" + '''import fcntl, json, os, sys, time
with open(os.environ["FAKE_DOCKER_LOG"], "a") as stream:
    fcntl.flock(stream, fcntl.LOCK_EX)
    stream.write(json.dumps(sys.argv[1:]) + "\\n")
arguments = sys.argv[1:]
target = arguments[arguments.index("--target") + 1]
time.sleep(float(os.environ.get("FAKE_DOCKER_DELAY", "0")))
sys.exit(9 if target == os.environ.get("FAKE_DOCKER_FAIL_TARGET") else 0)
''')
        docker.chmod(0o755)
        self.log = self.root / "docker.log"
        self.environment = {key: value for key, value in os.environ.items()
                            if not key.startswith("WELL_AMBIENT_") and key not in ("MAKEFLAGS", "MAKELEVEL", "MFLAGS", "SOURCE_DATE_EPOCH", "BUILD_BRANCH", "BUILD_COUNT", "BUILD_TIME", "COMMIT", "VERSION")}
        self.environment.update({"PATH": f"{binary}{os.pathsep}{os.environ['PATH']}", "FAKE_DOCKER_LOG": str(self.log)})

    def run_make(self, *variables, env=None, target="docker-images"):
        return subprocess.run(["make", target, "BUILD_TIME=2026-10-03T12:34:56Z", f"PYTHON={sys.executable}", *variables],
                              cwd=self.root, env={**self.environment, **(env or {})}, capture_output=True, text=True, timeout=20)

    def calls(self):
        return [json.loads(line) for line in self.log.read_text().splitlines()] if self.log.exists() else []

    def tag(self, count=1, date="261003"):
        version = f"20{date[:2]}.{date[2:4]}.{date[4:]}"
        return f"{version}.Feature_2fa_5fb_2ec.{self.commit[:8]}.{date}.b{count}"

    def test_real_make_builds_server_and_web_with_one_head_and_version(self):
        result = self.run_make()
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        calls = self.calls()
        self.assertEqual(len(calls), 2)
        for arguments, target in zip(calls, ("server", "web")):
            self.assertEqual(arguments[arguments.index("--target") + 1], target)
            self.assertEqual(arguments[arguments.index("-t") + 1], f"well-ambient-{target}:{self.tag()}")
            self.assertIn(f"COMMIT={self.commit}", arguments)
            self.assertIn(f"VERSION={self.tag()}", arguments)
            self.assertIn("BUILD_TIME=2026-10-03T12:34:56Z", arguments)
        metadata = self.root / "deploy/generated/release.env"
        self.assertIn(f"WELL_AMBIENT_VERSION={self.tag()}", metadata.read_text())
        record = json.loads((self.root / "deploy/generated/builds" / self.tag() / "build.json").read_text())
        self.assertEqual(record["status"], "succeeded")
        self.assertEqual(record["commit"], self.commit)
        self.assertEqual(record["build_time"], "2026-10-03T12:34:56Z")
        self.assertEqual(record["image_date"], "2026-10-03")
        self.assertEqual(record["image_timezone"], "Asia/Shanghai")

    def test_concurrent_builds_reserve_distinct_versions_and_keep_metadata_isolated(self):
        with ThreadPoolExecutor(max_workers=2) as pool:
            results = list(pool.map(lambda _: self.run_make(env={"FAKE_DOCKER_DELAY": "0.04"}), range(2)))
        for result in results:
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        references = [arguments[arguments.index("-t") + 1] for arguments in self.calls()]
        self.assertCountEqual(references, [f"well-ambient-{target}:{self.tag(count)}" for count in (1, 2) for target in ("server", "web")])
        for count in (1, 2):
            path = self.root / "deploy/generated/builds" / self.tag(count) / "release.env"
            self.assertIn(f"WELL_AMBIENT_VERSION={self.tag(count)}", path.read_text())
        latest = (self.root / "deploy/generated/release.env").read_text()
        self.assertTrue(any(f"WELL_AMBIENT_VERSION={self.tag(count)}" in latest for count in (1, 2)))

    def test_failed_second_image_does_not_publish_metadata_or_reuse_attempt(self):
        failed = self.run_make(env={"FAKE_DOCKER_FAIL_TARGET": "web"})
        self.assertNotEqual(failed.returncode, 0)
        self.assertFalse((self.root / "deploy/generated/release.env").exists())
        record = json.loads((self.root / "deploy/generated/builds" / self.tag() / "build.json").read_text())
        self.assertEqual(record["status"], "failed")
        self.assertEqual(record["build_time"], "2026-10-03T12:34:56Z")
        self.assertEqual(record["image_date"], "2026-10-03")
        self.assertEqual(record["image_timezone"], "Asia/Shanghai")
        succeeded = self.run_make()
        self.assertEqual(succeeded.returncode, 0, succeeded.stderr)
        self.assertIn(f"WELL_AMBIENT_VERSION={self.tag(2)}", (self.root / "deploy/generated/release.env").read_text())

    def test_ci_count_is_reserved_and_default_continues_above_it(self):
        first = self.run_make("BUILD_COUNT=37", target="docker-image")
        self.assertEqual(first.returncode, 0, first.stderr)
        repeated = self.run_make("BUILD_COUNT=37")
        self.assertNotEqual(repeated.returncode, 0)
        self.assertEqual(len(self.calls()), 2)
        next_build = self.run_make()
        self.assertEqual(next_build.returncode, 0, next_build.stderr)
        self.assertIn(self.tag(38), next_build.stdout)

    def test_detached_checkout_uses_ci_branch_and_rejects_false_commit(self):
        subprocess.run(["git", "checkout", "--quiet", "--detach"], cwd=self.root, check=True)
        result = self.run_make("BUILD_BRANCH=release/1.2")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn(f"2026.10.03.release_2f1_2e2.{self.commit[:8]}.261003.b1", result.stdout)
        failed = self.run_make("COMMIT=" + "0" * 40)
        self.assertNotEqual(failed.returncode, 0)
        self.assertEqual(len(self.calls()), 2)

    def test_daily_count_continues_across_commit_branch_and_version_changes(self):
        first = self.run_make("VERSION=1.2.3")
        self.assertEqual(first.returncode, 0, first.stderr)
        subprocess.run(["git", "checkout", "--quiet", "-b", "release"], cwd=self.root, check=True)
        subprocess.run(["git", "-c", "user.name=Build Test", "-c", "user.email=build-test@example.invalid",
                        "commit", "--quiet", "--allow-empty", "-m", "next commit"], cwd=self.root, check=True)
        commit = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=self.root, text=True).strip()
        second = self.run_make("VERSION=2.0.0")
        self.assertEqual(second.returncode, 0, second.stderr)
        self.assertIn(f"version: 2.0.0.release.{commit[:8]}.261003.b2", second.stdout)
        repeated = self.run_make("VERSION=3.0.0", "BUILD_COUNT=1")
        self.assertNotEqual(repeated.returncode, 0)
        self.assertEqual(len(self.calls()), 4)

    def test_repository_defaults_replace_tags_and_digests_but_keep_registry_port(self):
        result = self.run_make("SERVER_IMAGE=registry.example:5000/team/server:old", "WEB_IMAGE=registry.example:5000/team/web@sha256:" + "a" * 64)
        self.assertEqual(result.returncode, 0, result.stderr)
        references = [arguments[arguments.index("-t") + 1] for arguments in self.calls()]
        self.assertEqual(references, [f"registry.example:5000/team/{target}:{self.tag()}" for target in ("server", "web")])

    def test_invalid_name_counter_repository_and_oversized_branch_do_not_call_docker(self):
        for variable in ("VERSION=bad/name", "VERSION=latest", "BUILD_COUNT=0", "BUILD_COUNT=01", "SERVER_IMAGE=BadRepo", "WEB_IMAGE=https://example.com/web", "BUILD_BRANCH=wrong", "WEB_IMAGE=well-ambient-server"):
            with self.subTest(variable=variable):
                result = self.run_make(variable)
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(self.calls(), [])
        subprocess.run(["git", "checkout", "--quiet", "-b", "a" * 110], cwd=self.root, check=True)
        result = self.run_make()
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(self.calls(), [])

    def test_branch_encoding_does_not_collapse_punctuation_or_case(self):
        branches = ("feature/a", "feature-a", "feature_2fa", "feature.a", "Feature/a", "功能/优化")
        encoded = [builder.branch_segment(branch) for branch in branches]
        self.assertEqual(len(encoded), len(set(encoded)))
        for value in encoded:
            self.assertRegex(value, r"^[A-Za-z0-9_-]+$")

    def test_source_date_epoch_controls_utc_instant_and_legacy_images_still_accept_version(self):
        epoch_environment = {**self.environment, "SOURCE_DATE_EPOCH": "0"}
        result = subprocess.run(["make", "docker-images", f"PYTHON={sys.executable}"], cwd=self.root,
                                env=epoch_environment, capture_output=True, text=True, timeout=20)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn(f"1970.01.01.Feature_2fa_5fb_2ec.{self.commit[:8]}.700101.b1", result.stdout)
        self.assertIn("UTC build time: 1970-01-01T00:00:00Z", result.stdout)
        legacy = self.run_make("VERSION=legacy-contract-test", target="images")
        self.assertEqual(legacy.returncode, 0, legacy.stderr)
        self.assertEqual(self.calls()[-1][self.calls()[-1].index("-t") + 1], "well-ambient-web:legacy-contract-test")

    def test_shanghai_midnight_changes_tag_date_and_starts_a_new_day_counter(self):
        cases = (("2026-10-03T15:59:59Z", "261003", 1),
                 ("2026-10-03T16:00:00Z", "261004", 1),
                 ("2026-10-03T16:00:01Z", "261004", 2))
        for instant, date, count in cases:
            with self.subTest(instant=instant):
                result = self.run_make(f"BUILD_TIME={instant}")
                self.assertEqual(result.returncode, 0, result.stderr)
                version = self.tag(count, date)
                self.assertIn(f"version: {version}", result.stdout)
                record = json.loads((self.root / "deploy/generated/builds" / version / "build.json").read_text())
                self.assertEqual(record["build_time"], instant)
                self.assertEqual(record["image_timezone"], "Asia/Shanghai")
                self.assertEqual(record["image_date"], "2026-10-03" if date == "261003" else "2026-10-04")
        counters = json.loads((self.root / "deploy/generated/.docker-build-counts.json").read_text())
        self.assertCountEqual(counters.values(), [1, 2])

    def test_utc_midnight_keeps_shanghai_tag_date_and_continues_the_same_counter(self):
        for count, instant in enumerate(("2026-10-03T23:59:59Z", "2026-10-04T00:00:00Z"), start=1):
            result = self.run_make(f"BUILD_TIME={instant}")
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertIn(f"version: {self.tag(count, '261004')}", result.stdout)
            self.assertIn(f"WELL_AMBIENT_BUILD_TIME={instant}", (self.root / "deploy/generated/release.env").read_text())
            self.assertIn(f"UTC build time: {instant}", result.stdout)
        counters = json.loads((self.root / "deploy/generated/.docker-build-counts.json").read_text())
        self.assertEqual(list(counters.values()), [2])

    def test_source_date_epoch_at_shanghai_midnight_keeps_utc_build_metadata(self):
        result = subprocess.run(["make", "docker-images", f"PYTHON={sys.executable}"], cwd=self.root,
                                env={**self.environment, "SOURCE_DATE_EPOCH": "1791043200"},
                                capture_output=True, text=True, timeout=20)
        self.assertEqual(result.returncode, 0, result.stderr)
        version = self.tag(1, "261004")
        self.assertIn(f"version: {version}", result.stdout)
        self.assertIn("image date: 2026-10-04 (Asia/Shanghai)", result.stdout)
        record = json.loads((self.root / "deploy/generated/builds" / version / "build.json").read_text())
        self.assertEqual(record["build_time"], "2026-10-03T16:00:00Z")
        self.assertIn("WELL_AMBIENT_BUILD_TIME=2026-10-03T16:00:00Z", (self.root / "deploy/generated/release.env").read_text())

    def test_make_dry_run_does_not_reserve_a_count_or_invoke_docker(self):
        result = subprocess.run(["make", "--dry-run", "docker-images", f"PYTHON={sys.executable}"],
                                cwd=self.root, env=self.environment, capture_output=True, text=True, timeout=20)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("scripts/docker-images.py", result.stdout)
        self.assertFalse((self.root / "deploy/generated").exists())
        self.assertEqual(self.calls(), [])

    def test_arm_platform_reaches_both_docker_build_targets_without_arg_override(self):
        result = self.run_make("PLATFORM=linux/arm64")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(len(self.calls()), 2)
        for arguments in self.calls():
            self.assertEqual(arguments[arguments.index("--platform") + 1], "linux/arm64")
            self.assertFalse(any(argument.startswith(("TARGETOS=", "TARGETARCH=")) for argument in arguments))

    def test_all_server_binaries_inherit_buildkit_target_platform_without_amd64_default(self):
        dockerfile = (PROJECT / "Dockerfile").read_text()
        match = re.search(r"(?ms)^FROM [^\n]+ AS server-build\n(.*?)(?=^FROM\s)", dockerfile)
        self.assertIsNotNone(match, "server-build stage must exist")
        stage = match.group(1)
        for argument in ("TARGETOS", "TARGETARCH"):
            declarations = re.findall(rf"(?m)^ARG {argument}(.*)$", stage)
            self.assertEqual(declarations, [""], f"{argument} must inherit BuildKit's automatic value without a stage default")
            self.assertNotRegex(stage, rf"(?m)^ENV\s+.*\b{argument}=")
        commands = re.sub(r"\\\n\s*", " ", stage)
        builds = re.findall(r"CGO_ENABLED=0 GOOS=\$\{TARGETOS\} GOARCH=\$\{TARGETARCH\} go build\s+(.*?)\./cmd/([a-z-]+)", commands)
        self.assertCountEqual([target for _, target in builds], ["server", "open-access-admin", "open-mcp-stdio"])


if __name__ == "__main__":
    unittest.main()
