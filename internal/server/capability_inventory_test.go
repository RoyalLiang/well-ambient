package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"well-ambient/internal/agentruntime"
	"well-ambient/internal/config"
	"well-ambient/internal/db"
	"well-ambient/internal/headroom"
)

func TestVerifiedSkillInventoryAndBuiltinOwnership(t *testing.T) {
	setupServerTestDB(t)
	s := NewServer(&config.Config{}, "")
	ctx := context.Background()
	manifest := &agentruntime.CapabilityManifest{Schema: "capability-manifest/v1", ID: "test.installed", Kind: agentruntime.KindSkill, Version: 1, Budgets: agentruntime.BudgetDef{InstructionTokens: 800, EvidenceTokens: 2000}}
	manifest.Scope.Type = "global"
	manifest.Triggers.Intents = []string{"inventory-test"}
	ver, err := s.installCapability(ctx, manifest, "remote_manifest", "https://user:secret@example.test/skill?token=secret#secret")
	if err != nil {
		t.Fatal(err)
	}
	var receipt db.CapabilityInstallation
	if err = db.DB.Where("capability_version_id = ?", ver.ID).Take(&receipt).Error; err != nil {
		t.Fatal(err)
	}
	if receipt.Source != "https://example.test/skill" || !verifiedSkillArtifact(receipt) {
		t.Fatalf("invalid receipt: %+v", receipt)
	}
	check := func(want string) {
		t.Helper()
		records, err := s.enrichCapabilityInventory(ctx, []capabilityInventoryItem{{CapabilityKey: manifest.ID, Kind: "skill", Status: "active", LatestVersion: ver.Version, Versions: []db.CapabilityVersion{*ver}}})
		if err != nil {
			t.Fatal(err)
		}
		if records[0].InstallState != want || records[0].ClientInstallation != "未上报" {
			t.Fatalf("unexpected inventory: %+v", records[0])
		}
	}
	check("installed")
	if err = os.WriteFile(receipt.Location, []byte("tampered"), 0600); err != nil {
		t.Fatal(err)
	}
	check("missing")
	if err = os.Remove(receipt.Location); err != nil {
		t.Fatal(err)
	}
	check("missing")
	builtin, err := s.capabilityRegistry.GetCapabilityDetail(ctx, "well-ambient")
	if err != nil {
		t.Fatal(err)
	}
	enriched, err := s.inventoryDetail(ctx, builtin)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(enriched)
	var detail struct {
		Origin       string                     `json:"origin"`
		Components   []inventoryComponent       `json:"included_components"`
		Installation *db.CapabilityInstallation `json:"installation"`
	}
	if err = json.Unmarshal(body, &detail); err != nil {
		t.Fatal(err)
	}
	if detail.Origin != "system_builtin" || detail.Installation != nil || len(detail.Components) < 5 {
		t.Fatalf("builtin ownership: %s", body)
	}
	for _, c := range detail.Components {
		if c.SourceLabel != "系统内置" {
			t.Fatalf("component missing provenance: %+v", c)
		}
	}
	var rootManifest agentruntime.CapabilityManifest
	if err = json.Unmarshal([]byte(builtin.Versions[0].ManifestJSON), &rootManifest); err != nil {
		t.Fatal(err)
	}
	if len(rootManifest.Requires) > 0 || len(rootManifest.Tools) > 0 {
		t.Fatal("display ownership must not grant child tools")
	}
	token := superAdminToken(t, "inventory@example.test", "Inventory", nil)
	for _, id := range []string{"well-ambient", fmt.Sprint(builtin.Capability.ID)} {
		req := httptest.NewRequest(http.MethodDelete, "/api/agent-runtime/capabilities/"+id, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		s.handler.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("builtin removal got %d: %s", w.Code, w.Body.String())
		}
	}
}

func TestHeadroomDefaultBindingAndDisable(t *testing.T) {
	setupServerTestDB(t)
	s := NewServer(&config.Config{}, "")
	ctx := context.Background()
	manifest := &agentruntime.CapabilityManifest{Schema: "capability-manifest/v1", ID: "test.agent", Kind: agentruntime.KindSkill, Version: 1, Budgets: agentruntime.BudgetDef{InstructionTokens: 800, EvidenceTokens: 2000}}
	manifest.Scope.Type = "global"
	manifest.Triggers.Intents = []string{"test-agent"}
	if _, err := s.capabilityRegistry.Register(ctx, manifest); err != nil {
		t.Fatal(err)
	}
	if err := s.capabilityRegistry.ActivateVersion(ctx, manifest.ID, 1, "global", ""); err != nil {
		t.Fatal(err)
	}
	q := agentruntime.ResolveQuery{Intent: "test-agent", AllowedPermissions: []string{"*"}}
	plan, err := s.capabilityRegistry.Resolve(ctx, q)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range plan.Capabilities {
		if c.ID == headroom.CapabilityKey {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing default plugin: %+v", plan.Capabilities)
	}
	q.Intent = "unknown-no-business-match"
	if _, err = s.capabilityRegistry.Resolve(ctx, q); err == nil {
		t.Fatal("system plugin must not satisfy arbitrary intent")
	}
	if err = s.capabilityRegistry.UpdateCapabilityStatus(ctx, headroom.CapabilityKey, "disabled"); err != nil {
		t.Fatal(err)
	}
	if err = s.ensureHeadroomPlugin(ctx); err != nil {
		t.Fatal(err)
	}
	if headroom.Enabled(ctx) {
		t.Fatal("startup reset explicit disable")
	}
	q.Intent = "test-agent"
	plan, err = s.capabilityRegistry.Resolve(ctx, q)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range plan.Capabilities {
		if strings.HasPrefix(c.ID, "system.headroom") {
			t.Fatal("disabled plugin bound")
		}
	}
}

func TestSkillUpgradeCreatesVerifiedReceiptAndPrefersActiveVersion(t *testing.T) {
	setupServerTestDB(t)
	s := NewServer(&config.Config{}, "")
	ctx := context.Background()
	m := &agentruntime.CapabilityManifest{Schema: "capability-manifest/v1", ID: "test.upgrade", Kind: agentruntime.KindSkill, Version: 1, Budgets: agentruntime.BudgetDef{InstructionTokens: 800, EvidenceTokens: 2000}}
	m.Scope.Type = "global"
	first, err := s.installCapability(ctx, m, "manifest_import", "")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.capabilityRegistry.ActivateVersion(ctx, m.ID, first.Version, "global", ""); err != nil {
		t.Fatal(err)
	}
	m.Version = 2
	second, err := s.installCapabilityVersion(ctx, m, "manifest_upgrade", "", m.ID)
	if err != nil {
		t.Fatal(err)
	}
	var receipts []db.CapabilityInstallation
	if err = db.DB.Where("capability_key = ?", m.ID).Find(&receipts).Error; err != nil {
		t.Fatal(err)
	}
	if len(receipts) != 2 {
		t.Fatalf("upgrade receipts=%d", len(receipts))
	}
	for _, r := range receipts {
		if !verifiedSkillArtifact(r) {
			t.Fatal("upgrade artifact missing")
		}
	}
	// A later unactivated registration must not hide the verified active installation.
	m.Version = 3
	if _, err = s.capabilityRegistry.Register(ctx, m); err != nil {
		t.Fatal(err)
	}
	detail, err := s.capabilityRegistry.GetCapabilityDetail(ctx, m.ID)
	if err != nil {
		t.Fatal(err)
	}
	value, err := s.inventoryDetail(ctx, detail)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(value)
	var inventory struct {
		InstallState string                     `json:"install_state"`
		Installation *db.CapabilityInstallation `json:"installation"`
	}
	json.Unmarshal(body, &inventory)
	if inventory.InstallState != "installed" || inventory.Installation == nil || inventory.Installation.CapabilityVersionID != second.ID {
		t.Fatalf("active receipt selection: %s", body)
	}
}
