package runtimeknowledge

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"well-ambient/internal/db"
	"well-ambient/internal/openaccess"
)

// Browse is a management view. Its HTTP caller must require global administrator
// access; externally published knowledge continues to use the scoped Context API.
type BrowseRequest struct {
	SnapshotID uint
	After      uint
	Limit      int
	Kind       string
	ScopeID    string
	Query      string
	SourceRef  string
}

type BrowseItem struct {
	ID uint `json:"id"`
	db.RuntimeKnowledgeItem
	SourceState string `json:"source_state"`
}

type BrowseResponse struct {
	SnapshotID uint                          `json:"snapshot_id"`
	CheckedAt  *time.Time                    `json:"checked_at"`
	Generation db.RuntimeKnowledgeGeneration `json:"generation"`
	Items      []BrowseItem                  `json:"items"`
	Total      int64                         `json:"total"`
	Next       uint                          `json:"next_cursor,omitempty"`
	Gaps       []string                      `json:"gaps"`
}

func (s *Service) Browse(ctx context.Context, req BrowseRequest) (BrowseResponse, error) {
	out := BrowseResponse{Items: []BrowseItem{}, Gaps: []string{"local_snapshot_not_live_provider_evidence"}}
	if req.Limit == 0 {
		req.Limit = 20
	}
	if req.Limit < 1 || req.Limit > 50 || len(req.Query) > 200 || len(req.ScopeID) > 160 || len(req.SourceRef) > 255 || (req.Kind != "" && !contains(kinds, req.Kind)) {
		return out, openaccess.NewError("invalid_query", "invalid knowledge browser filters")
	}
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var state db.RuntimeKnowledgeState
		if err := tx.First(&state, 1).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		} else if err != nil {
			return err
		}
		out.CheckedAt = state.LastCheckedAt
		out.SnapshotID = req.SnapshotID
		if out.SnapshotID == 0 {
			out.SnapshotID = state.GenerationID
		}
		if out.SnapshotID == 0 {
			return nil
		}
		if err := tx.First(&out.Generation, out.SnapshotID).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return openaccess.NewError("conflict", "snapshot expired; reload the current snapshot")
		} else if err != nil {
			return err
		}
		q := tx.Model(&db.RuntimeKnowledgeItem{}).Where("generation_id = ?", out.SnapshotID)
		if req.Kind != "" {
			q = q.Where("kind = ?", req.Kind)
		}
		if req.ScopeID != "" {
			q = q.Where("LOWER(scope_id) = ?", strings.ToLower(strings.TrimSpace(req.ScopeID)))
		}
		if req.SourceRef != "" {
			q = q.Where("source_ref = ?", req.SourceRef)
		}
		if term := strings.ToLower(strings.TrimSpace(req.Query)); term != "" {
			term = "%" + strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(term) + "%"
			q = q.Where("LOWER(title) LIKE ? ESCAPE '!' OR LOWER(content) LIKE ? ESCAPE '!'", term, term)
		}
		if err := q.Count(&out.Total).Error; err != nil {
			return err
		}
		var rows []db.RuntimeKnowledgeItem
		if err := q.Where("id > ?", req.After).Order("id").Limit(req.Limit + 1).Find(&rows).Error; err != nil {
			return err
		}
		if len(rows) > req.Limit {
			rows = rows[:req.Limit]
			out.Next = rows[len(rows)-1].ID
		}
		for _, row := range rows {
			item := BrowseItem{ID: row.ID, RuntimeKnowledgeItem: row, SourceState: "current"}
			current, err := loadSource(tx, row.Kind, row.SourceID)
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			var valid bool
			if err == nil {
				current, valid = matchingChunk(current, row)
				if valid {
					current, valid = s.canonical(current)
				}
			}
			if !valid || current.Fingerprint != row.Fingerprint {
				item.Content = ""
				item.SourceState = "changed_or_unpublished"
			} else if req.SourceRef == "" {
				item.Content = clip(row.Content, 600)
				item.Truncated = row.Truncated || len(item.Content) < len(row.Content)
			}
			out.Items = append(out.Items, item)
		}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return out, err
}
