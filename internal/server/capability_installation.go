package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
	"well-ambient/internal/agentruntime"
	"well-ambient/internal/db"
)

var skillInstallationMu sync.Mutex
var installationKey = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,127}$`)

func skillInstallationRoot() string {
	if value := strings.TrimSpace(os.Getenv("WELL_AMBIENT_SKILLS_DIR")); value != "" {
		return value
	}
	return "/var/lib/well-ambient/skills"
}

func safeInstallationSource(value string) string {
	u, err := url.Parse(value)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return "用户导入清单"
	}
	u.User, u.RawQuery, u.Fragment = nil, "", ""
	return u.String()
}

// Explicit installation writes real immutable artifacts and a server receipt.
// Startup registration, distribution downloads and remote clients do not call it.
func (s *Server) installCapability(ctx context.Context, manifest *agentruntime.CapabilityManifest, method, source string) (*db.CapabilityVersion, error) {
	return s.installCapabilityVersion(ctx, manifest, method, source, "")
}

func (s *Server) installCapabilityVersion(ctx context.Context, manifest *agentruntime.CapabilityManifest, method, source, upgradeID string) (*db.CapabilityVersion, error) {
	if _, builtin := builtinCapability(manifest.ID); builtin {
		return nil, fmt.Errorf("系统内置能力不能通过安装接口替换")
	}
	if !installationKey.MatchString(manifest.ID) {
		return nil, fmt.Errorf("invalid installation key")
	}
	root, err := filepath.Abs(skillInstallationRoot())
	if err != nil {
		return nil, err
	}
	skillInstallationMu.Lock()
	defer skillInstallationMu.Unlock()
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	var version *db.CapabilityVersion
	var createdDirectory string
	err = db.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		registry := agentruntime.NewRegistry(tx)
		var registerErr error
		if upgradeID != "" {
			version, registerErr = registry.UpgradeCapability(ctx, upgradeID, manifest)
		} else {
			version, registerErr = registry.Register(ctx, manifest)
		}
		if registerErr != nil {
			return registerErr
		}
		// A valid manifest is the runtime artifact; SKILL.md makes its loaded
		// instructions inspectable even when the source was a JSON/YAML import.
		document := manifest.GenerateSkillMarkdown(manifest.Resources)
		if strings.TrimSpace(document) == "" {
			return fmt.Errorf("empty installation document")
		}
		artifactHash := sha256.Sum256([]byte(document))
		versionHash := sha256.Sum256([]byte(version.ContentDigest))
		directory := filepath.Join(root, manifest.ID, fmt.Sprintf("v%d-%x", version.Version, versionHash[:6]))
		if err := os.MkdirAll(filepath.Dir(directory), 0o700); err != nil {
			return err
		}
		parent, err := filepath.EvalSymlinks(filepath.Dir(directory))
		if err != nil || parent != filepath.Dir(directory) {
			return fmt.Errorf("installation parent must not be a symlink")
		}
		if err := os.Mkdir(directory, 0o700); err != nil {
			return fmt.Errorf("create immutable installation directory: %w", err)
		}
		createdDirectory = directory
		if err := os.WriteFile(filepath.Join(directory, "SKILL.md"), []byte(document), 0o600); err != nil {
			return err
		}
		body, err := json.MarshalIndent(manifest, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(directory, "capability-manifest.json"), body, 0o600); err != nil {
			return err
		}
		receipt := db.CapabilityInstallation{CapabilityKey: manifest.ID, CapabilityVersionID: version.ID, Version: version.Version, Method: method, Source: safeInstallationSource(source), Location: filepath.Join(directory, "SKILL.md"), Digest: version.ContentDigest, ArtifactSHA256: hex.EncodeToString(artifactHash[:]), EvidenceSource: "服务端安装回执 + 文件 SHA256 核验", InstalledAt: time.Now().UTC()}
		return tx.Create(&receipt).Error
	})
	if err != nil && createdDirectory != "" {
		_ = os.RemoveAll(createdDirectory)
	}
	return version, err
}

func verifiedSkillArtifact(receipt db.CapabilityInstallation) bool {
	root, err := filepath.Abs(skillInstallationRoot())
	if err != nil {
		return false
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return false
	}
	location, err := filepath.Abs(receipt.Location)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(root, location)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || filepath.IsAbs(rel) {
		return false
	}
	// Resolve symlinks before reading so a replaced receipt path cannot expose
	// arbitrary host files to an inventory query.
	resolved, err := filepath.EvalSymlinks(location)
	if err != nil || resolved != location {
		return false
	}
	info, err := os.Stat(location)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 2<<20 {
		return false
	}
	body, err := os.ReadFile(location)
	if err != nil {
		return false
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]) == receipt.ArtifactSHA256
}
