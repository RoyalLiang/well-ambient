package server

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"

	"well-ambient/internal/agentruntime"
	"well-ambient/internal/db"
	"well-ambient/internal/headroom"
	"well-ambient/internal/opennames"
)

type inventoryComponent struct {
	Key         string   `json:"key"`
	Kind        string   `json:"kind"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tools       []string `json:"tools,omitempty"`
	Origin      string   `json:"origin"`
	SourceLabel string   `json:"source_label"`
}

type capabilityInventoryMetadata struct {
	Origin             string                     `json:"origin"`
	InstallState       string                     `json:"install_state"`
	StatusSource       string                     `json:"status_source"`
	Installation       *db.CapabilityInstallation `json:"installation,omitempty"`
	ClientInstallation string                     `json:"client_installation"`
	BuiltinLocation    string                     `json:"builtin_location,omitempty"`
	PluginRuntime      *headroom.Status           `json:"plugin_runtime,omitempty"`
}

type builtinDefinition struct{ Parent, Location string }

func absoluteBuiltinPath(path string) string {
	resolved, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return resolved
}

func builtinCapability(key string) (builtinDefinition, bool) {
	switch key {
	case "well-ambient":
		return builtinDefinition{Location: absoluteBuiltinPath(filepath.Join(resolveSkillsRoot(), "well-ambient", "SKILL.md"))}, true
	case "well-ambient.open-capabilities-mcp":
		return builtinDefinition{Parent: "well-ambient", Location: "capabilities/open/mcp/open-capabilities-mcp/capability-manifest.yaml"}, true
	case "knowledge.runtime-context":
		return builtinDefinition{Parent: "well-ambient", Location: "程序内置运行时知识提供方"}, true
	case "code_review":
		return builtinDefinition{Location: "程序内置代码评审适配器"}, true
	case "gitlab.snapshot", "knowledge.search":
		return builtinDefinition{Parent: "code_review", Location: "程序内置组件"}, true
	case headroom.CapabilityKey:
		return builtinDefinition{Location: absoluteBuiltinPath(headroom.WorkerPath())}, true
	}
	for _, child := range officialSkills {
		if key == "well-ambient."+child.name {
			return builtinDefinition{Parent: "well-ambient", Location: filepath.Join(resolveSkillsRoot(), child.name, "SKILL.md")}, true
		}
	}
	return builtinDefinition{}, false
}

func builtinComponents(key string) []inventoryComponent {
	var result []inventoryComponent
	if key == "well-ambient" {
		result = append(result, inventoryComponent{Key: "well-ambient.open-capabilities-mcp", Kind: "mcp", Name: "开放能力 MCP 适配器", Description: "统一开放工具传输与授权校验；登记不代表客户端安装或连接成功。", Origin: "system_builtin", SourceLabel: "系统内置"})
		for _, child := range officialSkills {
			var tools []string
			for _, tool := range child.tools {
				tools = append(tools, opennames.Name(opennames.DefaultPrefix, tool))
			}
			result = append(result, inventoryComponent{Key: "well-ambient." + child.name, Kind: "skill", Name: child.title, Description: child.description, Tools: tools, Origin: "system_builtin", SourceLabel: "系统内置"})
		}
		result = append(result, inventoryComponent{Key: "knowledge.runtime-context", Kind: "context_provider", Name: "运行时知识提供方", Description: "授权知识背景与来源版本。", Origin: "system_builtin", SourceLabel: "系统内置"})
	}
	if key == "code_review" {
		result = append(result, inventoryComponent{Key: "gitlab.snapshot", Kind: "plugin", Name: "GitLab 快照插件", Description: "代码与 MR 快照抽取", Origin: "system_builtin", SourceLabel: "系统内置"}, inventoryComponent{Key: "knowledge.search", Kind: "context_provider", Name: "知识检索提供方", Description: "领域与工程规范语料检索", Origin: "system_builtin", SourceLabel: "系统内置"})
	}
	return result
}

func (s *Server) enrichCapabilityInventory(ctx context.Context, records []capabilityInventoryItem) ([]capabilityInventoryItem, error) {
	var receipts []db.CapabilityInstallation
	keys := make([]string, 0, len(records))
	for _, record := range records {
		keys = append(keys, record.CapabilityKey)
	}
	if len(keys) > 0 && db.DB.Migrator().HasTable(&db.CapabilityInstallation{}) {
		if err := db.DB.WithContext(ctx).Where("capability_key IN ?", keys).Order("installed_at DESC, id DESC").Find(&receipts).Error; err != nil {
			return nil, err
		}
	}
	byVersion := make(map[uint]db.CapabilityInstallation, len(receipts))
	for _, receipt := range receipts {
		byVersion[receipt.CapabilityVersionID] = receipt
	}
	for i := range records {
		record := &records[i]
		record.capabilityInventoryMetadata = capabilityInventoryMetadata{Origin: "registry_only", InstallState: "unverified", StatusSource: "运行时注册表 capabilities.status", ClientInstallation: "未上报"}
		if definition, ok := builtinCapability(record.CapabilityKey); ok {
			record.Origin, record.InstallState = "system_builtin", "builtin"
			record.ParentSkillKey, record.BuiltinLocation = definition.Parent, definition.Location
			record.IncludedComponents = builtinComponents(record.CapabilityKey)
			if record.CapabilityKey == headroom.CapabilityKey {
				runtime := headroom.RuntimeStatus()
				record.PluginRuntime = &runtime
			}
			continue
		}
		versions := append([]db.CapabilityVersion(nil), record.Versions...)
		sort.SliceStable(versions, func(i, j int) bool { return versions[i].Version > versions[j].Version })
		selectedVersion := record.LatestVersion
		for _, version := range versions {
			if version.Status == "active" {
				selectedVersion = version.Version
				break
			}
		}
		for _, version := range versions {
			if version.Version != selectedVersion {
				continue
			}
			receipt, ok := byVersion[version.ID]
			if !ok || receipt.CapabilityKey != record.CapabilityKey || receipt.Digest != version.ContentDigest {
				continue
			}
			record.Origin, record.InstallState, record.Installation = "installed", "missing", &receipt
			if verifiedSkillArtifact(receipt) {
				record.InstallState = "installed"
			}
			if record.Status == "uninstalled" || record.Status == "archived" {
				record.InstallState = record.Status
			}
			break
		}
	}
	return records, nil
}

func (s *Server) inventoryDetail(ctx context.Context, detail *agentruntime.CapabilityDetail) (any, error) {
	cap := detail.Capability
	record := capabilityInventoryItem{ID: cap.ID, CapabilityKey: cap.CapabilityKey, Kind: cap.Kind, Status: cap.Status}
	for _, version := range detail.Versions {
		record.Versions = append(record.Versions, version.CapabilityVersion)
		if version.Version > record.LatestVersion {
			record.LatestVersion = version.Version
		}
	}
	for _, component := range detail.IncludedComponents {
		record.IncludedComponents = append(record.IncludedComponents, inventoryComponent{Key: component.Key, Kind: component.Kind, Name: component.Name, Description: component.Description, Tools: component.Tools, Origin: "registry_only", SourceLabel: "注册依赖"})
	}
	items, err := s.enrichCapabilityInventory(ctx, []capabilityInventoryItem{record})
	if err != nil {
		return nil, fmt.Errorf("installation evidence unavailable: %w", err)
	}
	item := items[0]
	return struct {
		*agentruntime.CapabilityDetail
		capabilityInventoryMetadata
		ParentSkillKey     string               `json:"parent_skill_key,omitempty"`
		IncludedComponents []inventoryComponent `json:"included_components,omitempty"`
	}{detail, item.capabilityInventoryMetadata, item.ParentSkillKey, item.IncludedComponents}, nil
}

type capabilityInventoryItem struct {
	ID                 uint                   `json:"id"`
	CapabilityKey      string                 `json:"capability_key"`
	Kind               string                 `json:"kind"`
	Status             string                 `json:"status"`
	Owner              string                 `json:"owner"`
	Sensitivity        string                 `json:"sensitivity"`
	Description        string                 `json:"description"`
	DiskSizeBytes      int64                  `json:"disk_size_bytes"`
	DiskSizeFormatted  string                 `json:"disk_size_formatted"`
	BindingsCount      int64                  `json:"bindings_count"`
	IsArchived         bool                   `json:"is_archived"`
	IsTopLevelSkill    bool                   `json:"is_top_level_skill"`
	ParentSkillKey     string                 `json:"parent_skill_key,omitempty"`
	IncludedComponents []inventoryComponent   `json:"included_components,omitempty"`
	LatestVersion      int                    `json:"latest_version"`
	Versions           []db.CapabilityVersion `json:"versions"`
	CreatedAt          string                 `json:"created_at"`
	UpdatedAt          string                 `json:"updated_at"`
	capabilityInventoryMetadata
}
