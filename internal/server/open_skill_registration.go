package server

import (
	"context"
	"os"
	"path/filepath"

	"well-ambient/internal/agentruntime"
	"well-ambient/internal/db"
)

// Seed package metadata through the existing registry. Never reactivate or
// replace an operator-managed capability/version that already exists.
func (s *Server) ensureOpenSkillRegistration(ctx context.Context) error {
	paths := []string{filepath.Join(resolveSkillsRoot(), "..", "..", "capabilities", "open", "mcp", "open-capabilities-mcp", "capability-manifest.yaml")}
	for _, child := range officialSkills {
		paths = append(paths, filepath.Join(resolveSkillsRoot(), child.name, "capability-manifest.yaml"))
	}
	paths = append(paths, filepath.Join(resolveSkillsRoot(), openSkillBundleName, "capability-manifest.yaml"))
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		manifest, err := agentruntime.ParseManifest(renderBundleTools(raw))
		if err != nil {
			return err
		}
		if manifest.Kind == agentruntime.KindMCP {
			manifest.Triggers.Intents = []string{"well-ambient-tools"}
		}
		if manifest.ID == openSkillBundleName {
			// The parent routes work; it cannot grant every child tool by virtue
			// of its empty permission set. Children retain their own requirements.
			manifest.Tools = nil
			manifest.Requires = nil
		}
		var count int64
		if err := db.DB.WithContext(ctx).Model(&db.Capability{}).Where("capability_key = ?", manifest.ID).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			continue
		}
		if _, err := s.capabilityRegistry.Register(ctx, manifest); err != nil {
			return err
		}
		if err := s.capabilityRegistry.ActivateVersion(ctx, manifest.ID, manifest.Version, manifest.Scope.Type, manifest.Scope.ID); err != nil {
			return err
		}
	}
	return nil
}
