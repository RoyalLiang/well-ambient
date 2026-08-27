package server

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
)

const generatedSetupTokenBytes = 32

// SetupTokenProvision contains the token accepted by the setup server and,
// when generated locally, the owner-only temporary file that contains it.
type SetupTokenProvision struct {
	Token     string
	FilePath  string
	Generated bool
}

// ProvisionSetupToken preserves an operator-provided token or generates one
// in the operating system's temporary directory when no token was provided.
func ProvisionSetupToken(explicit string) (SetupTokenProvision, error) {
	return provisionSetupToken(explicit, os.TempDir(), rand.Reader)
}

func provisionSetupToken(explicit, tempDir string, random io.Reader) (SetupTokenProvision, error) {
	if explicit != "" {
		if err := validateSetupToken(explicit); err != nil {
			return SetupTokenProvision{}, err
		}
		return SetupTokenProvision{Token: explicit}, nil
	}

	randomBytes := make([]byte, generatedSetupTokenBytes)
	if _, err := io.ReadFull(random, randomBytes); err != nil {
		return SetupTokenProvision{}, fmt.Errorf("generate database setup token: %w", err)
	}
	token := hex.EncodeToString(randomBytes)

	file, err := os.CreateTemp(tempDir, "well-ambient-setup-token-*.txt")
	if err != nil {
		return SetupTokenProvision{}, fmt.Errorf("create database setup token file: %w", err)
	}
	filePath := file.Name()
	removeIncomplete := func() {
		_ = file.Close()
		_ = os.Remove(filePath)
	}
	if err := file.Chmod(0o600); err != nil {
		removeIncomplete()
		return SetupTokenProvision{}, fmt.Errorf("protect database setup token file: %w", err)
	}
	if _, err := io.WriteString(file, token+"\n"); err != nil {
		removeIncomplete()
		return SetupTokenProvision{}, fmt.Errorf("write database setup token file: %w", err)
	}
	if err := file.Sync(); err != nil {
		removeIncomplete()
		return SetupTokenProvision{}, fmt.Errorf("sync database setup token file: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(filePath)
		return SetupTokenProvision{}, fmt.Errorf("close database setup token file: %w", err)
	}

	return SetupTokenProvision{
		Token:     token,
		FilePath:  filePath,
		Generated: true,
	}, nil
}

func validateSetupToken(token string) error {
	if len(token) < minimumSetupTokenSize {
		return fmt.Errorf("%s must contain at least %d characters", SetupTokenEnvironment, minimumSetupTokenSize)
	}
	return nil
}

// Cleanup removes a generated token file. It is safe to call more than once.
func (p SetupTokenProvision) Cleanup() error {
	if !p.Generated || p.FilePath == "" {
		return nil
	}
	if err := os.Remove(p.FilePath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove database setup token file: %w", err)
	}
	return nil
}
