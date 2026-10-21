package db

import (
	"errors"
	"gorm.io/gorm"
	"time"
	"well-ambient/internal/reviewskill"
)

// InstallBundledCodeReviewSkill is called only by the explicit installer command.
// Previous versions remain intact; startup does not replace user-selected skills.
func InstallBundledCodeReviewSkill(conn *gorm.DB) (SolutionPromptTemplate, error) {
	var installed SolutionPromptTemplate
	err := conn.Transaction(func(tx *gorm.DB) error {
		if tx.Dialector.Name() == "postgres" {
			if err := tx.Exec("SELECT pg_advisory_xact_lock(8695782141)").Error; err != nil {
				return err
			}
		}
		query := tx.Where("purpose = ? AND scope_type = ? AND scope_id = ?", "code_review", "global", "")
		var existing SolutionPromptTemplate
		if err := query.Where("status = ? AND content_hash = ?", "active", reviewskill.Hash()).First(&existing).Error; err == nil {
			if existing.ValidationStatus != "passed" {
				return errors.New("同一技能包的活动版本验证未通过，请先复核该版本的验证记录")
			}
			installed = existing
			return nil
		} else if err != gorm.ErrRecordNotFound {
			return err
		}
		var version int
		if err := tx.Model(&SolutionPromptTemplate{}).Where("purpose = ? AND scope_type = ? AND scope_id = ?", "code_review", "global", "").Select("COALESCE(MAX(version),0)").Scan(&version).Error; err != nil {
			return err
		}
		if err := tx.Model(&SolutionPromptTemplate{}).Where("purpose = ? AND scope_type = ? AND scope_id = ? AND status = ?", "code_review", "global", "", "active").Update("status", "inactive").Error; err != nil {
			return err
		}
		now := time.Now()
		installed = SolutionPromptTemplate{Purpose: "code_review", ScopeType: "global", ScopeID: "", Version: version + 1, Status: "active", Name: "merge-review · " + reviewskill.Hash()[:12], SystemPrompt: reviewskill.Prompt(), ContentHash: reviewskill.Hash(), ValidationStatus: "passed", ValidationSummary: "Trusted user-provided bundle; local collector and release-policy regression suite verified. Runtime keeps its own JSON/evidence contract.", ValidatedBy: "bundled-installer", ValidatedAt: &now, CreatedBy: "bundled-installer", ActivatedBy: "bundled-installer", ActivatedAt: &now, CreatedAt: now}
		return tx.Create(&installed).Error
	})
	return installed, err
}
