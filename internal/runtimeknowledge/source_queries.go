package runtimeknowledge

import (
	"encoding/json"
	"strconv"

	"gorm.io/gorm"
	"well-ambient/internal/db"
)

type solutionRow struct {
	db.SolutionRevision
	ProjectKey string
}
type capabilityRow struct {
	db.CapabilityVersion
	CapabilityKey string
	Description   string
	Kind          string
}

func jiraItem(row db.TaskTelemetry) db.RuntimeKnowledgeItem {
	return item("jira", row.TaskID, "project", row.ProjectKey, row.SourceUpdatedAt.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"), row.Title,
		row.Description+"\nstatus="+row.Status+"; issue_type="+row.IssueType, "recorded_requirement_not_implementation", row.SourceUpdatedAt)
}
func factItem(row db.ContextFact) db.RuntimeKnowledgeItem {
	return item("context_fact", strconv.FormatUint(uint64(row.ID), 10), row.Scope, row.ScopeID, strconv.Itoa(row.Version), row.Type+": "+row.Summary, row.Content, "curated_claim_requires_validation", row.UpdatedAt)
}
func documentItem(row db.ContextDocument) db.RuntimeKnowledgeItem {
	return item("context_document", strconv.FormatUint(uint64(row.ID), 10), row.Scope, row.ScopeID, strconv.Itoa(row.Version), row.Title, row.Summary+"\n"+row.Content, "documented_claim_requires_validation", row.UpdatedAt)
}
func solutionItem(row solutionRow) (db.RuntimeKnowledgeItem, error) {
	body, err := decode(row.Content, row.ContentEncoding, row.ContentHash)
	if err != nil {
		return db.RuntimeKnowledgeItem{}, err
	}
	return item("solution", strconv.FormatUint(uint64(row.ID), 10), "project", row.ProjectKey, strconv.Itoa(row.Version), row.Title, row.Summary+"\n"+body, "published_design_not_implementation", row.CreatedAt), nil
}
func reviewItem(row db.CodeReviewRun) (db.RuntimeKnowledgeItem, error) {
	var report struct {
		Summary  string `json:"summary"`
		Scenario string `json:"scenario"`
	}
	if err := json.Unmarshal([]byte(row.ReportJSON), &report); err != nil {
		return db.RuntimeKnowledgeItem{}, err
	}
	return item("review", strconv.FormatUint(uint64(row.ID), 10), "repo", row.ProjectID, row.HeadSHA, row.Title, "head_sha="+row.HeadSHA+"; base_sha="+row.BaseSHA+"; review_status="+row.Status+"\n"+report.Summary+"\n"+report.Scenario, "review_observation_at_bound_sha", row.UpdatedAt), nil
}
func capabilityItem(row capabilityRow) db.RuntimeKnowledgeItem {
	return item("capability", strconv.FormatUint(uint64(row.ID), 10), row.ScopeType, row.ScopeID, strconv.Itoa(row.Version), row.CapabilityKey, row.Description+"\nkind="+row.Kind+"; validation_status="+row.ValidationStatus, "registered_capability_not_runtime_acceptance", row.UpdatedAt)
}

func factQuery(tx *gorm.DB) *gorm.DB {
	return tx.Model(&db.ContextFact{}).Where("status = ?", "active").Where("context_document_id = 0 OR EXISTS (SELECT 1 FROM context_documents d WHERE d.id = context_facts.context_document_id AND d.status = ? AND d.ingestion_status = ?)", "active", "completed")
}
func documentQuery(tx *gorm.DB) *gorm.DB {
	return tx.Model(&db.ContextDocument{}).Where("status = ? AND ingestion_status = ?", "active", "completed")
}
func solutionQuery(tx *gorm.DB) *gorm.DB {
	return tx.Model(&db.SolutionRevision{}).Select("solution_revisions.*, t.project_key").
		Joins("JOIN solution_assets a ON a.published_revision_id = solution_revisions.id AND a.id = solution_revisions.solution_asset_id").
		Joins("JOIN task_telemetries t ON t.task_id = a.demand_id").Where("solution_revisions.status = ?", "published")
}
func capabilityQuery(tx *gorm.DB) *gorm.DB {
	return tx.Model(&db.CapabilityVersion{}).Select("capability_versions.*, c.capability_key, c.description, c.kind").Joins("JOIN capabilities c ON c.id = capability_versions.capability_id").Where("capability_versions.status = ? AND c.status = ?", "active", "active")
}
func reviewQuery(tx *gorm.DB) *gorm.DB {
	return tx.Model(&db.CodeReviewRun{}).Where("status IN ?", []string{"completed", "partial"})
}

// Read-time authorization also revalidates the exact live projection, including
// parent-document archival, publication pointers and scope changes.
func loadSource(tx *gorm.DB, kind, id string) (db.RuntimeKnowledgeItem, error) {
	switch kind {
	case "jira":
		var row db.TaskTelemetry
		err := tx.Where("task_id = ? AND source = ?", id, "jira").Take(&row).Error
		return jiraItem(row), err
	case "context_fact":
		var row db.ContextFact
		err := factQuery(tx).Where("context_facts.id = ?", id).Take(&row).Error
		return factItem(row), err
	case "context_document":
		var row db.ContextDocument
		err := documentQuery(tx).Where("context_documents.id = ?", id).Take(&row).Error
		return documentItem(row), err
	case "solution":
		var row solutionRow
		if err := solutionQuery(tx).Where("solution_revisions.id = ?", id).Take(&row).Error; err != nil {
			return db.RuntimeKnowledgeItem{}, err
		}
		return solutionItem(row)
	case "review":
		var row db.CodeReviewRun
		if err := reviewQuery(tx).Where("id = ?", id).Take(&row).Error; err != nil {
			return db.RuntimeKnowledgeItem{}, err
		}
		return reviewItem(row)
	case "capability":
		var row capabilityRow
		err := capabilityQuery(tx).Where("capability_versions.id = ?", id).Take(&row).Error
		return capabilityItem(row), err
	}
	return db.RuntimeKnowledgeItem{}, gorm.ErrRecordNotFound
}

// Extraction uses keyset batches and streams records to storage. There is no
// per-source total cap and no per-record SQL lookup during a corpus rebuild.
func sourceBatches[T any](query *gorm.DB, key string, keyOf func(T) any, convert func(T) (db.RuntimeKnowledgeItem, error), emit func(db.RuntimeKnowledgeItem) error) error {
	var after any
	for {
		page := query.Session(&gorm.Session{})
		if after != nil {
			page = page.Where(key+" > ?", after)
		}
		var rows []T
		if err := page.Order(key + " ASC").Limit(64).Find(&rows).Error; err != nil {
			return err
		}
		for _, row := range rows {
			entry, err := convert(row)
			if err != nil {
				return err
			}
			if err := emit(entry); err != nil {
				return err
			}
		}
		if len(rows) < 64 {
			return nil
		}
		after = keyOf(rows[len(rows)-1])
	}
}

func collect(tx *gorm.DB, consume func(db.RuntimeKnowledgeItem) error) ([]string, error) {
	gaps := []string{}
	emit := func(entry db.RuntimeKnowledgeItem) error {
		if entry.Scope != "global" && entry.Scope != "project" && entry.Scope != "repo" {
			gaps = append(gaps, entry.Kind+":unsupported_scope")
			return nil
		}
		if entry.Scope != "global" && entry.ScopeID == "" {
			gaps = append(gaps, entry.Kind+":missing_scope")
			return nil
		}
		for _, part := range chunks(entry) {
			if err := consume(part); err != nil {
				return err
			}
		}
		return nil
	}
	steps := []func() error{
		func() error {
			return sourceBatches(tx.Model(&db.TaskTelemetry{}).Where("source = ?", "jira"), "task_id", func(v db.TaskTelemetry) any { return v.TaskID }, func(v db.TaskTelemetry) (db.RuntimeKnowledgeItem, error) { return jiraItem(v), nil }, emit)
		},
		func() error {
			return sourceBatches(solutionQuery(tx), "solution_revisions.id", func(v solutionRow) any { return v.ID }, solutionItem, emit)
		},
		func() error {
			return sourceBatches(factQuery(tx), "context_facts.id", func(v db.ContextFact) any { return v.ID }, func(v db.ContextFact) (db.RuntimeKnowledgeItem, error) { return factItem(v), nil }, emit)
		},
		func() error {
			return sourceBatches(documentQuery(tx), "context_documents.id", func(v db.ContextDocument) any { return v.ID }, func(v db.ContextDocument) (db.RuntimeKnowledgeItem, error) { return documentItem(v), nil }, emit)
		},
		func() error {
			return sourceBatches(reviewQuery(tx), "id", func(v db.CodeReviewRun) any { return v.ID }, reviewItem, emit)
		},
		func() error {
			return sourceBatches(capabilityQuery(tx), "capability_versions.id", func(v capabilityRow) any { return v.ID }, func(v capabilityRow) (db.RuntimeKnowledgeItem, error) { return capabilityItem(v), nil }, emit)
		},
	}
	for _, step := range steps {
		if err := step(); err != nil {
			return nil, err
		}
	}
	return unique(gaps), nil
}
