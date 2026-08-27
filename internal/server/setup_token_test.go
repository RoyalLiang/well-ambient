package server

import (
	"bytes"
	"encoding/hex"
	"errors"
	"os"
	"strings"
	"testing"
)

func TestProvisionSetupTokenGeneratesOwnerOnlyTemporaryFile(t *testing.T) {
	tempDir := t.TempDir()
	randomBytes := bytes.Repeat([]byte{0xab}, generatedSetupTokenBytes)

	provision, err := provisionSetupToken("", tempDir, bytes.NewReader(randomBytes))
	if err != nil {
		t.Fatalf("provisionSetupToken() error = %v", err)
	}
	if !provision.Generated {
		t.Fatal("generated token was not marked as generated")
	}
	if len(provision.Token) != generatedSetupTokenBytes*2 {
		t.Fatalf("generated token length = %d", len(provision.Token))
	}
	if _, err := hex.DecodeString(provision.Token); err != nil {
		t.Fatalf("generated token is not hexadecimal: %v", err)
	}
	if provision.FilePath == "" {
		t.Fatal("generated token file path is empty")
	}

	info, err := os.Stat(provision.FilePath)
	if err != nil {
		t.Fatalf("stat generated token file: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("generated token file mode = %o, want 600", got)
	}
	data, err := os.ReadFile(provision.FilePath)
	if err != nil {
		t.Fatalf("read generated token file: %v", err)
	}
	if got, want := string(data), provision.Token+"\n"; got != want {
		t.Fatalf("generated token file = %q, want %q", got, want)
	}

	if err := provision.Cleanup(); err != nil {
		t.Fatalf("cleanup generated token: %v", err)
	}
	if _, err := os.Stat(provision.FilePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("generated token file still exists after cleanup: %v", err)
	}
	if err := provision.Cleanup(); err != nil {
		t.Fatalf("second cleanup must be idempotent: %v", err)
	}
}

func TestProvisionSetupTokenPreservesExplicitTokenWithoutWritingFile(t *testing.T) {
	explicit := strings.Repeat("x", minimumSetupTokenSize)
	tempDir := t.TempDir()

	provision, err := provisionSetupToken(explicit, tempDir, bytes.NewReader(nil))
	if err != nil {
		t.Fatalf("provisionSetupToken() error = %v", err)
	}
	if provision.Generated || provision.FilePath != "" || provision.Token != explicit {
		t.Fatalf("explicit provision = %+v", provision)
	}
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("read temp dir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("explicit token created %d temporary files", len(entries))
	}
}

func TestProvisionSetupTokenRejectsShortExplicitToken(t *testing.T) {
	tempDir := t.TempDir()

	_, err := provisionSetupToken("too-short", tempDir, bytes.NewReader(nil))
	if err == nil || !strings.Contains(err.Error(), SetupTokenEnvironment) {
		t.Fatalf("short explicit token error = %v", err)
	}
	entries, readErr := os.ReadDir(tempDir)
	if readErr != nil {
		t.Fatalf("read temp dir: %v", readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("short explicit token created %d temporary files", len(entries))
	}
}

func TestProvisionSetupTokenDoesNotLeaveFileWhenRandomnessFails(t *testing.T) {
	tempDir := t.TempDir()

	_, err := provisionSetupToken("", tempDir, failingSetupTokenReader{})
	if err == nil {
		t.Fatal("randomness failure returned nil error")
	}
	entries, readErr := os.ReadDir(tempDir)
	if readErr != nil {
		t.Fatalf("read temp dir: %v", readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("randomness failure left %d temporary files", len(entries))
	}
}

type failingSetupTokenReader struct{}

func (failingSetupTokenReader) Read([]byte) (int, error) {
	return 0, errors.New("random source unavailable")
}
