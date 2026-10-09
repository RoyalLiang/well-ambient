import contextlib
import http.server
import importlib.util
import io
import json
import os
import pty
import select
import signal
from pathlib import Path
import subprocess
import tempfile
import threading
import time
import unittest


ROOT = Path(__file__).resolve().parents[2]
SOURCE = ROOT / "internal/server/open_skills_configure_codex.py"
spec = importlib.util.spec_from_file_location("codex_setup", SOURCE)
setup = importlib.util.module_from_spec(spec)
spec.loader.exec_module(setup)
TOKEN = "wa_live_test.fixture_secret"


@contextlib.contextmanager
def mcp_server(status=200, malformed=False):
    calls = []

    class Handler(http.server.BaseHTTPRequestHandler):
        def log_message(self, *args):
            pass

        def do_POST(self):
            payload = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
            calls.append((payload, dict(self.headers)))
            self.send_response(status if status != 200 else (202 if "id" not in payload else 200))
            self.send_header("Content-Type", "application/json")
            self.end_headers()
            if status != 200:
                self.wfile.write(b"error containing credential must never be logged")
                return
            if "id" not in payload:
                return
            result = {"protocolVersion": "2025-03-26", "capabilities": {"tools": {}},
                      "serverInfo": {"name": "fixture", "version": "1"}}
            if payload["method"] == "tools/list":
                result = {"tools": [{"name": "jira_describe_schema"}]}
            self.wfile.write(b"not json" if malformed else json.dumps({"jsonrpc": "2.0", "id": payload["id"], "result": result}).encode())

    server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        yield "http://127.0.0.1:%s/mcp" % server.server_port, calls
    finally:
        server.shutdown()
        server.server_close()
        thread.join()


class CodexSetupTests(unittest.TestCase):
    def test_real_http_handshake_and_preserving_config(self):
        original = '''model = "gpt-test"
instructions = """Keep this text:
[mcp_servers.well-ambient]
"""
[mcp_servers.other]
command = "other-cli"
[mcp_servers.'well-ambient']
url = "http://old/mcp"
[mcp_servers.'well-ambient'.http_headers]
Authorization = "Bearer previous"
[projects."/path/with spaces"]
trust_level = "trusted"
'''
        with tempfile.TemporaryDirectory() as directory, mcp_server() as (endpoint, calls):
            root = Path(directory)
            target = root / "config.toml"
            target.write_text(original)
            output = io.StringIO()
            with contextlib.redirect_stdout(output):
                setup.configure(directory, endpoint, TOKEN)
                setup.configure(directory, endpoint, TOKEN)
            self.assertNotIn(TOKEN, output.getvalue())
            merged = setup.tomllib.loads(target.read_text())
            expected = setup.tomllib.loads(original)
            expected["mcp_servers"]["well-ambient"] = {"url": endpoint, "http_headers": {"Authorization": "Bearer " + TOKEN}}
            self.assertEqual(merged, expected)
            self.assertIn('instructions = """Keep this text:', target.read_text())
            self.assertEqual(target.stat().st_mode & 0o777, 0o600)
            backups = list(root.glob("*.bak"))
            self.assertEqual(len(backups), 2)
            self.assertTrue(any(item.read_text() == original for item in backups))
            self.assertTrue(all(item.stat().st_mode & 0o777 == 0o600 for item in backups))
            self.assertEqual([call[0]["method"] for call in calls[:3]], ["initialize", "notifications/initialized", "tools/list"])
            self.assertTrue(all(call[1]["Authorization"] == "Bearer " + TOKEN for call in calls))
            self.assertEqual(calls[2][1]["Mcp-Protocol-Version"], "2025-03-26")

    def test_auth_policy_and_protocol_failures_do_not_modify_config(self):
        for status, malformed, message in [(401, False, "401"), (403, False, "403"), (200, True, "JSON")]:
            with self.subTest(status=status), tempfile.TemporaryDirectory() as directory, mcp_server(status, malformed) as (endpoint, _):
                target = Path(directory) / "config.toml"
                target.write_text('model = "keep"\n')
                with self.assertRaisesRegex(setup.SetupError, message):
                    setup.configure(directory, endpoint, TOKEN)
                self.assertEqual(target.read_text(), 'model = "keep"\n')
                self.assertEqual(list(Path(directory).glob("*.bak")), [])

    def test_unusual_toml_is_preserved_or_rejected_without_changes(self):
        examples = [
            "# comment\n[mcp_servers.other]\nurl = 'https://other'\n",
            "text = '''literal\n[mcp_servers.well-ambient]\n'''\n",
            'text = """escaped \\\"\n[mcp_servers.well-ambient]\n"""\n',
            '[[rules]]\nname = "one"\n[[rules]]\nname = "two"\n',
        ]
        for text in examples:
            with self.subTest(text=text):
                merged = setup.merge_config(text, "http://local/mcp", TOKEN)
                self.assertIn(text.strip(), merged)
        for text in ['mcp_servers = { "well-ambient" = { url = "old" } }', 'not valid toml']:
            with self.assertRaises(setup.SetupError):
                setup.merge_config(text, "http://local/mcp", TOKEN)

    def test_symlink_and_invalid_keys_are_rejected(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            original = root / "original.toml"
            original.write_text('model = "keep"')
            (root / "config.toml").symlink_to(original)
            with self.assertRaisesRegex(setup.SetupError, "符号链接"):
                setup.configure(directory, "http://local/mcp", TOKEN)
            self.assertEqual(original.read_text(), 'model = "keep"')
            for token in ["<YOUR_API_KEY>", "key_id", "wa_live_test.secret\nInjected: value"]:
                with self.assertRaises(setup.SetupError):
                    setup.configure(directory, "http://local/mcp", token)

    def test_bash_mcp_only_and_skip_modes(self):
        script_template = (ROOT / "internal/server/open_skills_install.sh").read_text()
        with tempfile.TemporaryDirectory() as directory, mcp_server() as (endpoint, _):
            root = Path(directory)
            script = root / "install.sh"
            script.write_text(script_template.replace("__WELL_AMBIENT_BASE_URL__", repr(endpoint.removesuffix("/mcp"))).replace("__WELL_AMBIENT_CODEX_SETUP__", SOURCE.read_text()))
            env = {**os.environ, "HOME": directory, "WELL_AMBIENT_API_KEY": TOKEN}
            run = subprocess.run(["bash", str(script), "--root", str(root / "custom codex"), "--mcp-only"], env=env, capture_output=True, text=True)
            self.assertEqual(run.returncode, 0, run.stdout + run.stderr)
            self.assertNotIn(TOKEN, run.stdout + run.stderr)
            self.assertTrue((root / "custom codex/config.toml").exists())
            self.assertFalse((root / "custom codex/skills").exists())
            env.pop("WELL_AMBIENT_API_KEY")
            run = subprocess.run(["bash", str(script), "--root", str(root / "empty"), "--mcp-only"], env=env, capture_output=True, text=True, start_new_session=True)
            self.assertNotEqual(run.returncode, 0)
            self.assertIn("WELL_AMBIENT_API_KEY", run.stderr)
            self.assertFalse((root / "empty/config.toml").exists())

    def test_terminal_key_input_is_hidden(self):
        with tempfile.TemporaryDirectory() as directory, mcp_server() as (endpoint, _):
            root = Path(directory)
            script = root / "install.sh"
            template = (ROOT / "internal/server/open_skills_install.sh").read_text()
            script.write_text(template.replace("__WELL_AMBIENT_BASE_URL__", repr(endpoint.removesuffix("/mcp"))).replace("__WELL_AMBIENT_CODEX_SETUP__", SOURCE.read_text()))
            env = {**os.environ, "HOME": directory}
            env.pop("WELL_AMBIENT_API_KEY", None)
            pid, descriptor = pty.fork()
            if pid == 0:
                os.execve("/bin/bash", ["bash", str(script), "--root", str(root / ".codex"), "--mcp-only"], env)
            output = b""
            sent = False
            finished = False
            try:
                deadline = time.monotonic() + 15
                while time.monotonic() < deadline:
                    if select.select([descriptor], [], [], 0.1)[0]:
                        try:
                            chunk = os.read(descriptor, 65536)
                        except OSError:
                            break
                        if not chunk:
                            break
                        output += chunk
                        if "输入隐藏".encode() in output and not sent:
                            os.write(descriptor, (TOKEN + "\n").encode())
                            sent = True
                    done, status = os.waitpid(pid, os.WNOHANG)
                    if done:
                        finished = True
                        self.assertEqual(os.waitstatus_to_exitcode(status), 0)
                        break
                self.assertTrue(sent)
                self.assertNotIn(TOKEN.encode(), output)
                self.assertTrue((root / ".codex/config.toml").exists())
            finally:
                os.close(descriptor)
                if not finished:
                    done, _ = os.waitpid(pid, os.WNOHANG)
                    if not done:
                        os.kill(pid, signal.SIGTERM)
                        os.waitpid(pid, 0)


if __name__ == "__main__":
    unittest.main()
