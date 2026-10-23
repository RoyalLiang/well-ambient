"""Verify offline Headroom compression as the image's default application user."""
import json
import subprocess
import sys

image = sys.argv[1]
user = subprocess.check_output(["docker", "image", "inspect", "--format", "{{.Config.User}}", image], text=True).strip()
assert user == "1000:1000", "Smoke test must exercise the application's unprivileged user"
content = json.dumps([{"service": "review-acceptance", "status": "running", "task_id": 170435, "site": "FDE", "message": "repeated synthetic log for lossless compression verification"} for _ in range(250)])
result = subprocess.run(["docker", "run", "--rm", "--network", "none", "-i", "--entrypoint", "/opt/well-ambient/headroom/bin/python", image, "-I", "/opt/well-ambient/plugins/headroom/worker.py"], input=json.dumps({"content": content, "model": "gpt-4o"}) + "\n", text=True, capture_output=True, timeout=90)
assert result.returncode == 0, result.stderr[-1800:]
lines = [json.loads(line) for line in result.stdout.splitlines() if line.strip()]
assert lines[0].get("ready") is True and lines[0]["version"] == "0.41.0"
compressed = lines[-1]
assert compressed.get("lossless") is True and "content" in compressed, compressed
assert len(compressed["content"].encode()) < len(content.encode()), "Synthetic input was not compressed"
assert "170435" in compressed["content"] and "FDE" in compressed["content"], "Critical input values were lost"
print(json.dumps({"image": image, "user": user, "network": "none", "headroom_version": lines[0]["version"], "lossless": True, "input_bytes": len(content.encode()), "output_bytes": len(compressed["content"].encode()), "tokens_before": compressed["tokens_before"], "tokens_after": compressed["tokens_after"]}))
