"""Verify the pinned root wheel against committed official PyPI metadata."""
import hashlib
import json
from pathlib import Path
import sys

metadata = json.loads(Path(__file__).with_name("UPSTREAM.json").read_text())
wheels = list(Path(sys.argv[1]).glob("*.whl"))
if len(wheels) != 1:
    raise SystemExit("Expected exactly one pinned Headroom wheel")
wheel = wheels[0]
expected = metadata["wheels"].get(wheel.name)
if not expected or hashlib.sha256(wheel.read_bytes()).hexdigest() != expected:
    raise SystemExit("Headroom wheel does not match official PyPI SHA256")
print("Verified official Headroom wheel SHA256:", wheel.name)
