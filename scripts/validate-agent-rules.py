#!/usr/bin/env python3
"""Validate the project-local agent rule runtime.

The script intentionally uses only Python's standard library. If PyYAML is
available, YAML files are parsed strictly; otherwise a lightweight syntax check
and path-reference validation keep the pre-commit hook dependency-free.
"""

from __future__ import annotations

import json
import re
import sys
from pathlib import Path
from typing import Any

ROOT = Path(__file__).resolve().parents[1]
PATH_RE = re.compile(r"\./[A-Za-z0-9_.\-/]+(?:#[A-Za-z0-9_.\-/]+)?")

try:
    import yaml  # type: ignore
except ImportError:  # pragma: no cover - environment dependent
    yaml = None


def rel(path: Path | str) -> str:
    text = str(path)
    if text.startswith("./"):
        return text
    try:
        return f"./{Path(text).resolve().relative_to(ROOT)}"
    except ValueError:
        return text


def abs_path(path: str) -> Path:
    clean = path.split("#", 1)[0]
    return ROOT / clean.removeprefix("./")


def fail(errors: list[str], message: str) -> None:
    errors.append(message)


def collect_paths(value: Any, paths: set[str]) -> None:
    if isinstance(value, dict):
        for child in value.values():
            collect_paths(child, paths)
    elif isinstance(value, list):
        for child in value:
            collect_paths(child, paths)
    elif isinstance(value, str):
        paths.update(PATH_RE.findall(value))


def collect_paths_from_text(text: str, paths: set[str]) -> None:
    paths.update(PATH_RE.findall(text))


def parse_json_file(path: str, errors: list[str]) -> Any | None:
    target = abs_path(path)
    try:
        text = target.read_text(encoding="utf-8")
    except FileNotFoundError:
        fail(errors, f"{path} does not exist")
        return None

    if not text.strip():
        fail(errors, f"{path} is empty JSON")
        return None

    try:
        return json.loads(text)
    except json.JSONDecodeError as exc:
        fail(errors, f"{path} JSON parse failed: {exc}")
        return None


def parse_jsonl_file(path: str, errors: list[str]) -> list[dict[str, Any]]:
    target = abs_path(path)
    records: list[dict[str, Any]] = []
    try:
        lines = target.read_text(encoding="utf-8").splitlines()
    except FileNotFoundError:
        fail(errors, f"{path} does not exist")
        return records

    for index, line in enumerate(lines, start=1):
        if not line.strip():
            continue
        try:
            record = json.loads(line)
        except json.JSONDecodeError as exc:
            fail(errors, f"{path}:{index} JSONL parse failed: {exc}")
            continue
        if not isinstance(record, dict):
            fail(errors, f"{path}:{index} JSONL record must be an object")
            continue
        records.append(record)
    return records


def lightweight_yaml_check(path: str, errors: list[str]) -> None:
    target = abs_path(path)
    try:
        lines = target.read_text(encoding="utf-8").splitlines()
    except FileNotFoundError:
        fail(errors, f"{path} does not exist")
        return

    for index, line in enumerate(lines, start=1):
        if not line.strip() or line.lstrip().startswith("#"):
            continue
        if "\t" in line:
            fail(errors, f"{path}:{index} contains a tab indentation")
        indent = len(line) - len(line.lstrip(" "))
        if indent % 2 != 0:
            fail(errors, f"{path}:{index} indentation is not a multiple of 2")
        stripped = line.strip()
        if stripped.startswith("- "):
            stripped = stripped[2:].strip()
            if not stripped:
                continue
            if ":" not in stripped:
                continue
        if ":" not in stripped:
            fail(errors, f"{path}:{index} is not a key/value or scalar list entry")


def parse_yaml_file(path: str, errors: list[str]) -> Any | None:
    target = abs_path(path)
    if yaml is None:
        lightweight_yaml_check(path, errors)
        return None

    try:
        with target.open("r", encoding="utf-8") as handle:
            return yaml.safe_load(handle)
    except FileNotFoundError:
        fail(errors, f"{path} does not exist")
    except Exception as exc:  # PyYAML uses parser/scanner exception subclasses.
        fail(errors, f"{path} YAML parse failed: {exc}")
    return None


def registry_paths(errors: list[str]) -> set[str]:
    path = "./.agents/registry.yaml"
    target = abs_path(path)
    paths: set[str] = set()
    try:
        text = target.read_text(encoding="utf-8")
    except FileNotFoundError:
        fail(errors, f"{path} does not exist")
        return paths

    parsed = parse_yaml_file(path, errors)
    if parsed is not None:
        for entry in parsed.get("required", []):
            required_path = entry.get("path")
            if required_path:
                paths.add(required_path)
        collect_paths(parsed.get("modules", {}), paths)
        collect_paths(parsed.get("memory", {}), paths)
    else:
        for match in re.finditer(r"path:\s*[\"']?(\./[^\"'\s]+)", text):
            paths.add(match.group(1))
        collect_paths_from_text(text, paths)
    return paths


def validate_yaml_files(errors: list[str]) -> None:
    for path in sorted((ROOT / ".agents").rglob("*.yaml")):
        if path.name.startswith("._"):
            continue
        parse_yaml_file(rel(path), errors)


def validate_json_files(errors: list[str]) -> None:
    for path in sorted((ROOT / ".agents").rglob("*.json")):
        if path.name.startswith("._"):
            continue
        parse_json_file(rel(path), errors)


def validate_jsonl_files(errors: list[str]) -> None:
    for path in sorted((ROOT / ".agents").rglob("*.jsonl")):
        if path.name.startswith("._"):
            continue
        parse_jsonl_file(rel(path), errors)


def validate_references(errors: list[str]) -> None:
    for path in registry_paths(errors):
        if not abs_path(path).exists():
            fail(errors, f"{path} is missing")

    router_path = "./.agents/router.yaml"
    try:
        router_text = abs_path(router_path).read_text(encoding="utf-8")
    except FileNotFoundError:
        fail(errors, f"{router_path} does not exist")
        return

    references: set[str] = set()
    parsed_router = parse_yaml_file(router_path, errors)
    if parsed_router is not None:
        collect_paths(parsed_router, references)
    else:
        collect_paths_from_text(router_text, references)

    optional = {"./task_plan.md", "./findings.md", "./progress.md"}
    for path in references:
        target = path.split("#", 1)[0]
        if target in optional:
            continue
        if not abs_path(target).exists():
            fail(errors, f"{path} referenced by router is missing")


def validate_cold_boot_budgets(errors: list[str]) -> None:
    budgets = {
        "./.agents/boot.yaml": 80,
        "./.agents/core-lite.yaml": 80,
        "./.agents/state-lite.json": 40,
        "./.agents/route-presets.jsonl": 80,
    }
    for path, limit in budgets.items():
        try:
            line_count = len(abs_path(path).read_text(encoding="utf-8").splitlines())
        except FileNotFoundError:
            fail(errors, f"{path} does not exist")
            continue
        if line_count > limit:
            fail(errors, f"{path} has {line_count} lines, exceeds cold-boot budget {limit}")


def validate_memory_index(errors: list[str]) -> None:
    lite_index_path = "./.agents/memory/lite-index.jsonl"
    for index, record in enumerate(parse_jsonl_file(lite_index_path, errors), start=1):
        latest = record.get("latest")
        files = record.get("files") or []
        if not latest:
            continue
        if not isinstance(files, list):
            fail(errors, f"{lite_index_path}:{index} files must be a list")
            continue
        records: list[dict[str, Any]] = []
        for file_path in files:
            if not isinstance(file_path, str):
                fail(errors, f"{lite_index_path}:{index} file path must be a string")
                continue
            records.extend(parse_jsonl_file(file_path, errors))
        if not any(item.get("id") == latest for item in records):
            fail(errors, f"{lite_index_path}:{index} latest {latest} does not resolve")


def main() -> int:
    errors: list[str] = []
    validate_references(errors)
    validate_yaml_files(errors)
    validate_json_files(errors)
    validate_jsonl_files(errors)
    validate_cold_boot_budgets(errors)
    validate_memory_index(errors)

    if errors:
        print("Agent rule validation failed:", file=sys.stderr)
        for error in errors:
            print(f"- {error}", file=sys.stderr)
        return 1

    mode = "strict YAML" if yaml is not None else "dependency-free YAML checks"
    print(f"Agent rule validation passed ({mode}).")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
