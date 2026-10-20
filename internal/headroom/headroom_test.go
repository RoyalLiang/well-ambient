package headroom

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fakeWorker(t *testing.T, response string) *Optimizer {
	t.Helper()
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 unavailable")
	}
	script := filepath.Join(t.TempDir(), "worker.py")
	code := "import json,sys,time\nprint(json.dumps({'ready':True,'version':'0.41.0'}),flush=True)\nfor line in sys.stdin:\n request=json.loads(line)\n " + response + "\n"
	if err = os.WriteFile(script, []byte(code), 0600); err != nil {
		t.Fatal(err)
	}
	o := New(python, script)
	t.Cleanup(o.Close)
	return o
}

func TestLosslessFallbackAndBoundedWorker(t *testing.T) {
	input := strings.Repeat("critical=2048;", 600)
	for _, tt := range []struct{ name, response, want string }{{"lossless", "print(json.dumps({'content':'critical=2048','lossless':True,'tokens_before':1000,'tokens_after':10}),flush=True)", "compressed"}, {"lossy", "print(json.dumps({'content':'removed','lossless':False}),flush=True)", "lossless_unverified"},
		{"opaque", "print(json.dumps({'content':'<<ccr:opaque>>','lossless':True}),flush=True)", "opaque_output"}, {"error", "print(json.dumps({'error':'Failure'}),flush=True)", "worker_error"}, {"timeout", "time.sleep(5)", "timeout"}} {
		t.Run(tt.name, func(t *testing.T) {
			o := fakeWorker(t, tt.response)
			if tt.name == "timeout" {
				o.Timeout = 200 * time.Millisecond
			}
			started := time.Now()
			out, reason := o.Optimize(context.Background(), input, "gpt-4o")
			if reason != tt.want {
				t.Fatalf("reason %s want %s", reason, tt.want)
			}
			if tt.want != "compressed" && out != input {
				t.Fatal("fallback altered content")
			}
			if time.Since(started) > 2*time.Second {
				t.Fatal("worker exceeded timeout")
			}
			status := o.Status()
			if status.Calls != 1 || status.LastInputSHA256 == "" {
				t.Fatalf("missing observability: %+v", status)
			}
			if tt.want != "compressed" {
				if _, reason = o.Optimize(context.Background(), input, "gpt-4o"); reason != "cooldown" {
					t.Fatalf("expected cooldown, got %s", reason)
				}
			}
		})
	}
	o := fakeWorker(t, "print(json.dumps({'content':request['content'],'lossless':True}),flush=True)")
	if _, reason := o.Optimize(context.Background(), "short", "gpt-4o"); reason != "below_threshold" {
		t.Fatal(reason)
	}
	o.gate <- struct{}{}
	if out, reason := o.Optimize(context.Background(), input, "gpt-4o"); reason != "busy" || out != input {
		t.Fatal("busy worker changed input")
	}
	<-o.gate
}

func TestPinnedHeadroomPackage(t *testing.T) {
	python := os.Getenv("WELL_AMBIENT_HEADROOM_TEST_PYTHON")
	if python == "" {
		t.Skip("set package Python path for real integration")
	}
	script, err := filepath.Abs("../../capabilities/plugins/headroom/worker.py")
	if err != nil {
		t.Fatal(err)
	}
	o := New(python, script)
	o.Timeout = 30 * time.Second
	t.Cleanup(o.Close)
	rows := make([]string, 200)
	for i := range rows {
		level := "INFO"
		if i == 167 {
			level = "FATAL"
		}
		rows[i] = fmt.Sprintf(`{"id":%d,"component":"inventory-test","level":"%s","limit":2048}`, i, level)
	}
	input := "[" + strings.Join(rows, ",") + "]"
	out, reason := o.Optimize(context.Background(), input, "gpt-4o")
	if reason != "compressed" || !strings.Contains(out, "FATAL") || !strings.Contains(out, "2048") || len(out) >= len(input) {
		t.Fatalf("real package failed reason=%s input=%d output=%d status=%+v", reason, len(input), len(out), o.Status())
	}
	t.Logf("headroom %s: %d -> %d bytes, tokens_saved=%d", Version, len(input), len(out), o.Status().TokensSaved)
}
