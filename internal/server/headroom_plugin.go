package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"well-ambient/internal/agentruntime"
	"well-ambient/internal/db"
	"well-ambient/internal/headroom"
)

func (s *Server) ensureHeadroomPlugin(ctx context.Context) error {
	var count int64
	if err := db.DB.WithContext(ctx).Model(&db.Capability{}).Where("capability_key = ?", headroom.CapabilityKey).Count(&count).Error; err != nil {
		return err
	}
	if count != 0 {
		return nil
	} // Preserve an operator's disabled state.
	manifest := &agentruntime.CapabilityManifest{
		Schema: "capability-manifest/v1", ID: headroom.CapabilityKey,
		Kind: agentruntime.KindPlugin, Version: 1, Owner: "well-ambient",
		Description: "Headroom 0.41.0 系统插件：Agent 启用后默认执行本地无损上下文压缩；保护系统指令与附件，异常回退原文。",
		Provides:    []string{"context.lossless-compression"},
		Resources:   []agentruntime.ResourceDef{{Key: "installation", ContentKind: "json", LoadLevel: agentruntime.LoadLevelL0, Content: `{"package":"headroom-ai","version":"0.41.0","source":"https://github.com/headroomlabs-ai/headroom","mode":"agent-lossless"}`}},
	}
	manifest.Scope.Type = "global"
	manifest.Triggers.Intents = []string{"headroom-context"}
	if _, err := s.capabilityRegistry.Register(ctx, manifest); err != nil {
		return err
	}
	return s.capabilityRegistry.ActivateVersion(ctx, manifest.ID, manifest.Version, "global", "")
}

// A deterministic local-only probe; it never calls the configured LLM provider.
func (s *Server) handleCheckHeadroom(w http.ResponseWriter, r *http.Request) {
	if !headroom.Enabled(r.Context()) {
		http.Error(w, "Headroom is disabled", http.StatusConflict)
		return
	}
	content := "[" + strings.Repeat(`{"component":"agent-check","status":"ok","value":123},`, 199) + `{"component":"critical-check","status":"FATAL","value":2048}]`
	_, reason := headroom.Default().Optimize(r.Context(), content, "gpt-4o")
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"reason": reason, "runtime": headroom.RuntimeStatus(), "source": "本地合成数据检查；未调用模型服务"})
}
