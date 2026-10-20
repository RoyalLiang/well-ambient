"""Private stdio bridge to the pinned Headroom Agent API; no provider calls."""
import contextlib
import importlib.metadata
import json
import os
import sys

os.environ.update({
    "HF_HUB_OFFLINE": "1", "TRANSFORMERS_OFFLINE": "1",
    "LITELLM_LOCAL_MODEL_COST_MAP": "True", "OTEL_SDK_DISABLED": "true",
    "DO_NOT_TRACK": "1", "HEADROOM_DISABLE_TELEMETRY": "1",
})

with contextlib.redirect_stdout(sys.stderr):
    from headroom.compress import densify

version = importlib.metadata.version("headroom-ai")
if version != "0.41.0":
    raise RuntimeError("Unsupported Headroom package version")
print(json.dumps({"ready": True, "version": version, "mode": "agent-lossless"}), flush=True)

for line in sys.stdin:
    try:
        if len(line) > 12 * 1024 * 1024:
            raise ValueError("Input too large")
        request = json.loads(line)
        text = request["content"]
        if not isinstance(text, str):
            raise TypeError("String content required")
        # The prompt envelope is kept in Go. Only the contextual body enters
        # Headroom; Agent mode must prove its changes round-trip losslessly.
        with contextlib.redirect_stdout(sys.stderr):
            result = densify(
                [{"role": "user", "content": text}],
                model=request.get("model") or "gpt-4o",
                compress_user_messages=True, compress_system_messages=False,
                protect_recent=0, protect_analysis_context=True,
            )
        output = result.messages[0]["content"]
        verified = result.lossless is True
        if not verified or not isinstance(output, str) or len(output.encode()) > len(text.encode()):
            output = text
        print(json.dumps({
            "content": output, "lossless": verified,
            "tokens_before": result.tokens_before, "tokens_after": result.tokens_after,
            "transforms": result.transforms_applied,
        }, ensure_ascii=False), flush=True)
    except Exception as error:
        # Exception messages can contain source text; expose only the type.
        print(json.dumps({"error": type(error).__name__}), flush=True)
