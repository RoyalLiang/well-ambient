package codereview

import (
	"context"
	"errors"
	"fmt"
	"strings"

	skillassets "well-ambient"
	"well-ambient/internal/db"
)

type ReviewMethod struct {
	Name          string `json:"name"`
	CapabilityKey string `json:"capability_key"`
	VersionID     uint   `json:"version_id"`
	Version       int    `json:"version"`
	Hash          string `json:"hash"`
}

type frozenReviewMethod struct {
	ReviewMethod
	Instructions string `json:"instructions"`
}

type frozenReviewPolicy struct {
	db.CodeReviewPolicy
	Method *frozenReviewMethod `json:"review_method,omitempty"`
}

func (s *Service) freezeReviewMethod(ctx context.Context) (*frozenReviewMethod, error) {
	// Compatibility with installations predating the capability registry. Once
	// a review capability is registered, disabling it must never fall back.
	if !s.DB.Migrator().HasTable(&db.Capability{}) {
		return nil, nil
	}
	var capability db.Capability
	for _, key := range []string{"code_review", "merge-review"} {
		result := s.DB.WithContext(ctx).Where("capability_key = ?", key).Limit(1).Find(&capability)
		if result.Error != nil {
			return nil, result.Error
		}
		if result.RowsAffected != 0 {
			break
		}
	}
	if capability.ID == 0 {
		return nil, nil
	}
	if capability.Status != "active" {
		return nil, errors.New("merge-review 代码评审技能已停用")
	}
	var version db.CapabilityVersion
	result := s.DB.WithContext(ctx).Where("capability_id = ? AND scope_type = ? AND scope_id = ? AND status = ? AND validation_status = ?", capability.ID, "global", "", "active", "passed").Order("version DESC").Limit(1).Find(&version)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, errors.New("已启用的代码评审技能缺少通过验证的版本")
	}
	var resources []db.CapabilityResource
	if err := s.DB.WithContext(ctx).Where("capability_version_id = ?", version.ID).Order("id ASC").Find(&resources).Error; err != nil {
		return nil, err
	}
	content := skillassets.MergeReviewBundle()
	for _, resource := range resources {
		if strings.TrimSpace(resource.Content) == "" {
			continue
		}
		expected := strings.TrimPrefix(resource.ContentHash, "sha256:")
		if len(expected) != 64 || expected != digest(resource.Content) {
			return nil, fmt.Errorf("启用的评审技能资源 %s 内容校验失败", resource.ResourceKey)
		}
		if len(content)+len(resource.Content) > 80000 {
			return nil, errors.New("启用的评审技能资源超出指令预算")
		}
		content += fmt.Sprintf("\n## 已启用资源 %s\n%s\n", resource.ResourceKey, resource.Content)
	}
	return &frozenReviewMethod{ReviewMethod: ReviewMethod{Name: "merge-review", CapabilityKey: capability.CapabilityKey, VersionID: version.ID, Version: version.Version, Hash: digest(content)}, Instructions: content}, nil
}

func (s *Service) reviewMethodEnabled(ctx context.Context, method *frozenReviewMethod) error {
	if method == nil {
		return nil
	}
	if digest(method.Instructions) != method.Hash {
		return errors.New("冻结的 merge-review 技能内容校验失败")
	}
	var capability db.Capability
	if err := s.DB.WithContext(ctx).Where("capability_key = ?", method.CapabilityKey).First(&capability).Error; err != nil {
		return err
	}
	if capability.Status != "active" {
		return errors.New("merge-review 代码评审技能已停用")
	}
	return nil
}
