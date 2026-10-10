#!/usr/bin/env python3
"""Build the existing server/web targets with one immutable release version."""

import argparse
import datetime as dt
import fcntl
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile
from zoneinfo import ZoneInfo


IMAGE_TIMEZONE = ZoneInfo("Asia/Shanghai")


def git(root, *args):
    return subprocess.check_output(["git", "-C", str(root), *args], text=True).strip()


def branch_segment(branch):
    # Escape the escape marker itself: feature/a, feature-a and feature_2fa differ.
    return "".join(chr(byte) if (48 <= byte <= 57 or 65 <= byte <= 90 or 97 <= byte <= 122 or byte == 45)
                   else f"_{byte:02x}" for byte in branch.encode("utf-8"))


def repository_ref(value):
    """Keep registry ports; a production default may already include a tag/digest."""
    repository, separator, digest = value.partition("@")
    if separator and not re.fullmatch(r"sha(?:256:[a-fA-F0-9]{64}|512:[a-fA-F0-9]{128})", digest):
        raise ValueError(f"invalid Docker image digest: {value}")
    if repository.rfind(":") > repository.rfind("/"):
        repository, tag = repository.rsplit(":", 1)
        if not re.fullmatch(r"[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}", tag):
            raise ValueError(f"invalid Docker image tag: {value}")
    components = repository.split("/")
    if len(components) > 1 and ("." in components[0] or ":" in components[0] or components[0] == "localhost"):
        registry = components.pop(0)
        if not re.fullmatch(r"[a-z0-9](?:[a-z0-9.-]*[a-z0-9])?(?::[0-9]{1,5})?", registry):
            raise ValueError(f"invalid Docker registry: {value}")
        if ":" in registry and not 1 <= int(registry.rsplit(":", 1)[1]) <= 65535:
            raise ValueError(f"invalid Docker registry port: {value}")
    if len(repository) > 255 or not all(re.fullmatch(r"[a-z0-9]+(?:(?:[._]|__|-+)[a-z0-9]+)*", component) for component in components):
        raise ValueError(f"invalid Docker image repository: {value}; use a lowercase repository reference")
    return repository


def atomic_write(path, content):
    fd, temporary = tempfile.mkstemp(prefix=f".{path.name}.", dir=path.parent)
    try:
        with os.fdopen(fd, "w") as stream:
            stream.write(content)
            stream.flush()
            os.fsync(stream.fileno())
        os.replace(temporary, path)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)


def build_clock():
    explicit = os.environ.get("BUILD_TIME") or os.environ.get("WELL_AMBIENT_BUILD_TIME")
    if explicit:
        return dt.datetime.strptime(explicit, "%Y-%m-%dT%H:%M:%SZ").replace(tzinfo=dt.timezone.utc)
    epoch = os.environ.get("SOURCE_DATE_EPOCH")
    if epoch is not None:
        if not re.fullmatch(r"[0-9]+", epoch):
            raise ValueError("SOURCE_DATE_EPOCH must be a non-negative integer")
        return dt.datetime.fromtimestamp(int(epoch), tz=dt.timezone.utc)
    return dt.datetime.now(tz=dt.timezone.utc)


def reserve_build(output, name, branch, commit, clock, explicit_count):
    image_day = clock.astimezone(IMAGE_TIMEZONE).strftime("%y%m%d")
    prefix = f"{name}.{branch_segment(branch)}.{commit[:8]}.{image_day}.b"
    if len(prefix + "1") > 128:
        raise ValueError("image version exceeds Docker's 128-character tag limit; shorten VERSION or branch")
    key = f"daily:{image_day}"
    output.mkdir(parents=True, exist_ok=True)
    state_file = output / ".docker-build-counts.json"
    with (output / ".docker-build.lock").open("a+") as lock:
        fcntl.flock(lock, fcntl.LOCK_EX)
        counters = json.loads(state_file.read_text()) if state_file.exists() else {}
        if not isinstance(counters, dict) or any(not isinstance(value, int) or isinstance(value, bool) or value < 1 for value in counters.values()):
            raise ValueError("invalid build counter state; restore it instead of resetting published versions")
        if explicit_count:
            if not re.fullmatch(r"[1-9][0-9]*", explicit_count):
                raise ValueError("BUILD_COUNT must be a positive integer without leading zeroes")
            count = int(explicit_count)
            if count <= counters.get(key, 0):
                raise ValueError("BUILD_COUNT must exceed the last reserved build for this Shanghai date")
        else:
            count = counters.get(key, 0) + 1
        version = prefix + str(count)
        if len(version) > 128:
            raise ValueError("image version exceeds Docker's 128-character tag limit")
        build_dir = output / "builds" / version
        build_dir.parent.mkdir(exist_ok=True)
        # Refuse a repeated CI count or edited local counter; immutable tags stay unique.
        build_dir.mkdir()
        counters[key] = max(counters.get(key, 0), count)
        atomic_write(state_file, json.dumps(counters, sort_keys=True) + "\n")
    return version, count, build_dir


def main():
    parser = argparse.ArgumentParser(description=__doc__, epilog=(
        "Version: <VERSION>.<encoded branch>.<actual HEAD8>.<Asia/Shanghai yymmdd>.b<BUILD_COUNT>. "
        "VERSION defaults to the Shanghai calendar date (YYYY.MM.DD). "
        "Branch UTF-8 punctuation is escaped as _xx. Local counters cover all builds on the Shanghai day; "
        "failed builds retain their number. CI should set a unique BUILD_COUNT. "
        "SOURCE_DATE_EPOCH or BUILD_TIME pins the UTC instant; build_time metadata remains UTC. "
        "Image dates and daily counters follow Asia/Shanghai. Requires Python 3.9+ and IANA timezone data. "
        "This command never pushes images."))
    parser.add_argument("--make", default="make", help="GNU Make executable")
    args = parser.parse_args()
    root = Path(__file__).resolve().parent.parent
    clock = build_clock()
    name = os.environ.get("VERSION") or clock.astimezone(IMAGE_TIMEZONE).strftime("%Y.%m.%d")
    if name == "latest" or not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9_.-]*", name):
        raise ValueError("VERSION must be an immutable Docker tag segment")
    repositories = {}
    for target, fallback in (("SERVER_IMAGE", "well-ambient-server"), ("WEB_IMAGE", "well-ambient-web")):
        original = os.environ.get(target) or fallback
        repositories[target] = repository_ref(original)
        if original != repositories[target]:
            print(f"{target}: using repository {repositories[target]} (replacing existing tag/digest)", flush=True)
    if len(set(repositories.values())) != 2:
        raise ValueError("SERVER_IMAGE and WEB_IMAGE must use different repositories to avoid overwriting one another")
    commit = git(root, "rev-parse", "--verify", "HEAD")
    if not re.fullmatch(r"[a-f0-9]{40,64}", commit):
        raise ValueError("Git HEAD is not a full commit hash")
    requested_commit = os.environ.get("COMMIT") or os.environ.get("WELL_AMBIENT_COMMIT")
    if requested_commit and requested_commit != commit:
        raise ValueError("COMMIT must equal the actual full Git HEAD for this build")
    branch_result = subprocess.run(["git", "-C", str(root), "symbolic-ref", "--quiet", "--short", "HEAD"], capture_output=True, text=True)
    branch = branch_result.stdout.strip()
    requested_branch = os.environ.get("BUILD_BRANCH", "")
    if branch and requested_branch and requested_branch != branch:
        raise ValueError("BUILD_BRANCH must match the checked-out branch; it is intended for detached CI checkouts")
    branch = branch or requested_branch or "detached"
    if not branch or len(branch.encode("utf-8")) > 512:
        raise ValueError("BUILD_BRANCH is empty or too long")
    output = root / "deploy" / "generated"
    version, count, build_dir = reserve_build(output, name, branch, commit, clock, os.environ.get("BUILD_COUNT", ""))
    dirty = bool(git(root, "status", "--porcelain", "--untracked-files=normal"))
    record = {"version": version, "branch": branch, "commit": commit, "build_time": f"{clock:%Y-%m-%dT%H:%M:%SZ}",
              "image_date": clock.astimezone(IMAGE_TIMEZONE).strftime("%Y-%m-%d"), "image_timezone": IMAGE_TIMEZONE.key,
              "build_count": count, "worktree_dirty": dirty, "status": "building"}
    atomic_write(build_dir / "build.json", json.dumps(record, indent=2) + "\n")
    print(f"version: {version}\nbranch: {branch}\ncommit: {commit}\nUTC build time: {record['build_time']}\nimage date: {record['image_date']} ({record['image_timezone']})\nattempt: {count} (retained on failure)\nworktree dirty: {str(dirty).lower()}", flush=True)
    environment = os.environ.copy()
    # Existing release helpers allow environment overrides; pin these to this build.
    environment.update({"WELL_AMBIENT_VERSION": version, "WELL_AMBIENT_COMMIT": commit,
                        "WELL_AMBIENT_BUILD_TIME": record["build_time"], "WELL_AMBIENT_WORKTREE_DIRTY": str(dirty).lower()})
    result = subprocess.run([args.make, "images", f"RELEASE_DIR={build_dir}", f"VERSION={version}",
                             *(f"{target}={repository}" for target, repository in repositories.items()),
                             f"COMMIT={commit}", f"BUILD_TIME={record['build_time']}"], cwd=root, env=environment)
    record["status"] = "succeeded" if result.returncode == 0 else "failed"
    record["exit_code"] = result.returncode
    atomic_write(build_dir / "build.json", json.dumps(record, indent=2) + "\n")
    if result.returncode != 0:
        print(f"build failed; reserved version is not reused: {version}\nmetadata: {build_dir}", file=sys.stderr)
        return result.returncode
    with (output / ".docker-build.lock").open("a+") as lock:
        fcntl.flock(lock, fcntl.LOCK_EX)
        for filename in ("release-notes.txt", "release.env"):
            atomic_write(output / filename, (build_dir / filename).read_text())
    for repository in repositories.values():
        print(f"image: {repository}:{version}")
    print(f"metadata: {build_dir}\nlast completed successful metadata: {output / 'release.env'}")
    return 0


if __name__ == "__main__":
    try:
        sys.exit(main())
    except (ValueError, OSError, subprocess.CalledProcessError) as error:
        print(f"docker-images: {error}", file=sys.stderr)
        sys.exit(2)
