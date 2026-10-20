package headroom

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"well-ambient/internal/db"
)

const CapabilityKey = "system.headroom"
const Version = "0.41.0"

type Status struct {
	Version         string    `json:"version"`
	Mode            string    `json:"mode"`
	DefaultEnabled  bool      `json:"default_enabled"`
	State           string    `json:"state"`
	StateSource     string    `json:"state_source"`
	WorkerPath      string    `json:"worker_path"`
	Calls           uint64    `json:"calls"`
	Compressed      uint64    `json:"compressed"`
	Fallbacks       uint64    `json:"fallbacks"`
	InputBytes      uint64    `json:"input_bytes"`
	OutputBytes     uint64    `json:"output_bytes"`
	TokensSaved     uint64    `json:"tokens_saved"`
	LastReason      string    `json:"last_reason,omitempty"`
	LastInputSHA256 string    `json:"last_input_sha256,omitempty"`
	LastCheckedAt   time.Time `json:"last_checked_at,omitempty"`
}

type Result struct {
	Content      string `json:"content"`
	Lossless     bool   `json:"lossless"`
	TokensBefore int    `json:"tokens_before"`
	TokensAfter  int    `json:"tokens_after"`
	Error        string `json:"error"`
}

type Optimizer struct {
	Python, Script string
	Timeout        time.Duration
	gate           chan struct{}
	mu             sync.Mutex
	status         Status
	cmd            *exec.Cmd
	in             io.WriteCloser
	out            *bufio.Scanner
	retryAfter     time.Time
}

func WorkerPath() string {
	if value := strings.TrimSpace(os.Getenv("WELL_AMBIENT_HEADROOM_WORKER")); value != "" {
		return value
	}
	return filepath.Join("capabilities", "plugins", "headroom", "worker.py")
}

func New(python, script string) *Optimizer {
	return &Optimizer{Python: python, Script: script, Timeout: 8 * time.Second, gate: make(chan struct{}, 1), status: Status{Version: Version, Mode: "agent-lossless", DefaultEnabled: true, State: "pending", StateSource: "当前服务进程的真实压缩桥接状态；统计在重启后归零", WorkerPath: script}}
}

var defaultOnce sync.Once
var defaultOptimizer *Optimizer

func Default() *Optimizer {
	defaultOnce.Do(func() {
		python := strings.TrimSpace(os.Getenv("WELL_AMBIENT_HEADROOM_PYTHON"))
		if python == "" {
			python = "python3"
		}
		defaultOptimizer = New(python, WorkerPath())
	})
	return defaultOptimizer
}

func RuntimeStatus() Status         { return Default().Status() }
func (o *Optimizer) Status() Status { o.mu.Lock(); defer o.mu.Unlock(); return o.status }

// The runtime registration is a control, not proof that Python or the package
// can execute. A missing/disabled registration never silently enables it.
func Enabled(ctx context.Context) bool {
	if db.DB == nil {
		return false
	}
	var capability db.Capability
	if err := db.DB.WithContext(ctx).Where("capability_key = ?", CapabilityKey).Take(&capability).Error; err != nil {
		return false
	}
	return capability.Status == "active"
}

func (o *Optimizer) stop() {
	if o.cmd != nil {
		_ = o.in.Close()
		_ = o.cmd.Process.Kill()
		_ = o.cmd.Wait()
	}
	o.cmd, o.in, o.out = nil, nil, nil
}

func (o *Optimizer) start(ctx context.Context) error {
	if o.cmd != nil {
		return nil
	}
	cmd := exec.Command(o.Python, "-I", o.Script)
	cmd.Env = append(os.Environ(), "PYTHONUNBUFFERED=1")
	input, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		_ = input.Close()
		return err
	}
	// No prompt text is written to logs, including upstream library warnings.
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		_ = input.Close()
		_ = output.Close()
		return err
	}
	o.cmd, o.in = cmd, input
	o.out = bufio.NewScanner(output)
	o.out.Buffer(make([]byte, 64<<10), 16<<20)
	scanner := o.out
	ready := make(chan error, 1)
	go func() {
		if !scanner.Scan() {
			ready <- errors.New("worker_start_failed")
			return
		}
		var response struct {
			Ready   bool   `json:"ready"`
			Version string `json:"version"`
		}
		if json.Unmarshal(scanner.Bytes(), &response) != nil || !response.Ready || response.Version != Version {
			ready <- errors.New("worker_version_mismatch")
			return
		}
		ready <- nil
	}()
	select {
	case err := <-ready:
		if err != nil {
			o.stop()
		}
		return err
	case <-ctx.Done():
		o.stop()
		<-ready
		return ctx.Err()
	}
}

func (o *Optimizer) Optimize(ctx context.Context, content, model string) (string, string) {
	if len(content) < 4096 {
		return content, "below_threshold"
	}
	if len(content) > 8<<20 {
		return content, "input_limit"
	}
	select {
	case o.gate <- struct{}{}:
	default:
		return content, "busy"
	}
	defer func() { <-o.gate }()
	if time.Now().Before(o.retryAfter) {
		return content, "cooldown"
	}
	deadline := o.Timeout
	if deadline <= 0 {
		deadline = 8 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, deadline)
	defer cancel()
	reason := "unchanged"
	output := content
	var result Result
	if err := o.start(ctx); err != nil {
		reason = "worker_unavailable"
	} else {
		input, scanner := o.in, o.out
		response := make(chan error, 1)
		go func() {
			if err := json.NewEncoder(input).Encode(map[string]string{"content": content, "model": model}); err != nil {
				response <- err
				return
			}
			if !scanner.Scan() {
				response <- errors.New("worker_read_failed")
				return
			}
			response <- json.Unmarshal(scanner.Bytes(), &result)
		}()
		select {
		case err := <-response:
			if err != nil || result.Error != "" {
				reason = "worker_error"
				o.stop()
			} else if !result.Lossless {
				reason = "lossless_unverified"
			} else if strings.Contains(result.Content, "<<ccr:") && !strings.Contains(content, "<<ccr:") {
				reason = "opaque_output"
			} else if result.Content == "" || len(result.Content) > len(content) {
				reason = "invalid_output"
			} else {
				output = result.Content
				if output != content {
					reason = "compressed"
				}
			}
		case <-ctx.Done():
			reason = "timeout"
			o.stop()
			<-response
		}
	}
	if reason != "compressed" && reason != "unchanged" {
		o.retryAfter = time.Now().Add(30 * time.Second)
	} else {
		o.retryAfter = time.Time{}
	}
	sum := sha256.Sum256([]byte(content))
	o.mu.Lock()
	o.status.Calls++
	o.status.InputBytes += uint64(len(content))
	o.status.OutputBytes += uint64(len(output))
	o.status.LastInputSHA256 = hex.EncodeToString(sum[:])
	o.status.LastReason, o.status.LastCheckedAt = reason, time.Now().UTC()
	if reason == "compressed" {
		o.status.Compressed++
	}
	if reason != "compressed" && reason != "unchanged" {
		o.status.Fallbacks++
		o.status.State = "degraded"
	} else {
		o.status.State = "ready"
	}
	if output != content && result.TokensBefore > result.TokensAfter {
		o.status.TokensSaved += uint64(result.TokensBefore - result.TokensAfter)
	}
	o.mu.Unlock()
	return output, reason
}

func (o *Optimizer) Close() {
	o.gate <- struct{}{}
	defer func() { <-o.gate }()
	o.stop()
}
