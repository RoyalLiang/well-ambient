package solutioncatalog

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"well-ambient/internal/db"
)

type ListFilter struct {
	ProjectScope []string
	ProjectKey   string
	Search       string
	Limit        int
	Offset       int
}

type CatalogList struct {
	Items  []db.SolutionCatalogEntry `json:"items"`
	Total  int64                     `json:"total"`
	Limit  int                       `json:"limit"`
	Offset int                       `json:"offset"`
}

type CatalogProject struct {
	ProjectKey string    `json:"project_key"`
	Count      int64     `json:"count"`
	LastSynced time.Time `json:"last_synced"`
}

type ComparisonView struct {
	Comparison  db.SolutionComparison        `json:"comparison"`
	Counterpart db.SolutionCatalogEntry      `json:"counterpart"`
	RoundOne    json.RawMessage              `json:"round_one,omitempty"`
	RoundTwo    json.RawMessage              `json:"round_two,omitempty"`
	Proposal    *StandardizationProposalView `json:"proposal,omitempty"`
}

type StandardizationProposalView struct {
	Proposal db.SolutionStandardizationProposal `json:"proposal"`
	Markdown string                             `json:"markdown"`
}

type CatalogDetail struct {
	Entry             db.SolutionCatalogEntry `json:"entry"`
	Markdown          string                  `json:"markdown"`
	DemandDescription string                  `json:"demand_description"`
	Comparisons       []ComparisonView        `json:"comparisons"`
}

type StandardListItem struct {
	Standard  db.SolutionStandard `json:"standard"`
	Version   int                 `json:"version"`
	Projects  []string            `json:"projects"`
	UpdatedAt time.Time           `json:"updated_at"`
}

type StandardDetail struct {
	Standard db.SolutionStandard         `json:"standard"`
	Revision db.SolutionStandardRevision `json:"revision"`
	Markdown string                      `json:"markdown"`
	Projects []string                    `json:"projects"`
}

func (module *Module) List(ctx context.Context, filter ListFilter) (CatalogList, error) {
	if module == nil || module.conn == nil {
		return CatalogList{}, ErrNotFound
	}
	filter.ProjectScope = db.NormalizeProjectKeys(filter.ProjectScope)
	filter.ProjectKey = strings.ToUpper(strings.TrimSpace(filter.ProjectKey))
	if filter.ProjectKey != "" && !allowedProject(filter.ProjectKey, filter.ProjectScope) {
		return CatalogList{Items: []db.SolutionCatalogEntry{}, Limit: normalizeListLimit(filter.Limit), Offset: max(filter.Offset, 0)}, nil
	}
	limit := normalizeListLimit(filter.Limit)
	offset := max(filter.Offset, 0)
	base := module.conn.WithContext(ctx).Model(&db.SolutionCatalogEntry{})
	base = applyProjectScope(base, filter.ProjectScope)
	if filter.ProjectKey != "" {
		base = base.Where("project_key = ?", filter.ProjectKey)
	}

	if searchTokens := catalogTokens(filter.Search); len(searchTokens) > 0 {
		type searchMatch struct {
			EntryID uint
			Matches int
		}
		search := module.conn.WithContext(ctx).
			Table("solution_catalog_search_tokens AS search_tokens").
			Select("search_tokens.entry_id, COUNT(*) AS matches").
			Joins("JOIN solution_catalog_entries AS catalog_entries ON catalog_entries.id = search_tokens.entry_id").
			Where("search_tokens.token IN ?", searchTokens)
		if len(filter.ProjectScope) > 0 {
			search = search.Where("catalog_entries.project_key IN ?", filter.ProjectScope)
		}
		if filter.ProjectKey != "" {
			search = search.Where("catalog_entries.project_key = ?", filter.ProjectKey)
		}
		var matches []searchMatch
		if err := search.Group("search_tokens.entry_id").Order("matches DESC, search_tokens.entry_id DESC").Scan(&matches).Error; err != nil {
			return CatalogList{}, err
		}
		if len(matches) == 0 {
			return CatalogList{Items: []db.SolutionCatalogEntry{}, Limit: limit, Offset: offset}, nil
		}
		total := int64(len(matches))
		if offset >= len(matches) {
			return CatalogList{Items: []db.SolutionCatalogEntry{}, Total: total, Limit: limit, Offset: offset}, nil
		}
		end := min(len(matches), offset+limit)
		pageMatches := matches[offset:end]
		ids := make([]uint, 0, len(pageMatches))
		rank := make(map[uint]int, len(pageMatches))
		for index, match := range pageMatches {
			ids = append(ids, match.EntryID)
			rank[match.EntryID] = index
		}
		var items []db.SolutionCatalogEntry
		if err := base.Where("id IN ?", ids).Find(&items).Error; err != nil {
			return CatalogList{}, err
		}
		sort.SliceStable(items, func(i, j int) bool { return rank[items[i].ID] < rank[items[j].ID] })
		return CatalogList{Items: items, Total: total, Limit: limit, Offset: offset}, nil
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return CatalogList{}, err
	}
	var items []db.SolutionCatalogEntry
	if err := base.Order("published_at DESC, id DESC").Offset(offset).Limit(limit).Find(&items).Error; err != nil {
		return CatalogList{}, err
	}
	if items == nil {
		items = []db.SolutionCatalogEntry{}
	}
	return CatalogList{Items: items, Total: total, Limit: limit, Offset: offset}, nil
}

func normalizeListLimit(limit int) int {
	if limit <= 0 {
		return 40
	}
	if limit > 100 {
		return 100
	}
	return limit
}

func (module *Module) ListProjects(ctx context.Context, scope []string) ([]CatalogProject, error) {
	type projectAggregate struct {
		ProjectKey     string
		Count          int64
		LastSyncedUnix int64
	}
	query := module.conn.WithContext(ctx).Model(&db.SolutionCatalogEntry{}).
		Select("project_key, COUNT(*) AS count, MAX(unixepoch(synced_at)) AS last_synced_unix")
	query = applyProjectScope(query, scope)
	var aggregates []projectAggregate
	if err := query.Group("project_key").Order("project_key ASC").Scan(&aggregates).Error; err != nil {
		return nil, err
	}
	projects := make([]CatalogProject, 0, len(aggregates))
	for _, aggregate := range aggregates {
		lastSynced := time.Time{}
		if aggregate.LastSyncedUnix > 0 {
			lastSynced = time.Unix(aggregate.LastSyncedUnix, 0).UTC()
		}
		projects = append(projects, CatalogProject{ProjectKey: aggregate.ProjectKey, Count: aggregate.Count, LastSynced: lastSynced})
	}
	return projects, nil
}

func (module *Module) Get(ctx context.Context, id uint, scope []string) (CatalogDetail, error) {
	if id == 0 {
		return CatalogDetail{}, ErrNotFound
	}
	query := module.conn.WithContext(ctx).Where("id = ?", id)
	query = applyProjectScope(query, scope)
	var entry db.SolutionCatalogEntry
	lookup := query.Limit(1).Find(&entry)
	if lookup.Error != nil {
		return CatalogDetail{}, lookup.Error
	}
	if lookup.RowsAffected == 0 {
		return CatalogDetail{}, ErrNotFound
	}
	var revision db.SolutionRevision
	if err := module.conn.WithContext(ctx).First(&revision, entry.PublishedRevisionID).Error; err != nil {
		return CatalogDetail{}, err
	}
	markdown, err := decodeSolutionRevision(revision)
	if err != nil {
		return CatalogDetail{}, err
	}
	var demand db.TaskTelemetry
	description := ""
	demandLookup := module.conn.WithContext(ctx).Where("task_id = ?", entry.DemandID).Limit(1).Find(&demand)
	if demandLookup.Error != nil {
		return CatalogDetail{}, demandLookup.Error
	}
	if demandLookup.RowsAffected > 0 {
		description = strings.TrimSpace(demand.Description)
	}
	comparisons, err := module.comparisonsForEntry(ctx, entry, scope)
	if err != nil {
		return CatalogDetail{}, err
	}
	return CatalogDetail{Entry: entry, Markdown: markdown, DemandDescription: description, Comparisons: comparisons}, nil
}

func (module *Module) comparisonsForEntry(ctx context.Context, entry db.SolutionCatalogEntry, scope []string) ([]ComparisonView, error) {
	var comparisons []db.SolutionComparison
	if err := module.conn.WithContext(ctx).
		Where("left_entry_id = ? OR right_entry_id = ?", entry.ID, entry.ID).
		Order("recall_score DESC, id DESC").Find(&comparisons).Error; err != nil {
		return nil, err
	}
	views := make([]ComparisonView, 0, len(comparisons))
	for _, comparison := range comparisons {
		counterpartID := comparison.RightEntryID
		if counterpartID == entry.ID {
			counterpartID = comparison.LeftEntryID
		}
		var counterpart db.SolutionCatalogEntry
		counterpartQuery := module.conn.WithContext(ctx).Where("id = ?", counterpartID)
		counterpartQuery = applyProjectScope(counterpartQuery, scope)
		lookup := counterpartQuery.Limit(1).Find(&counterpart)
		if lookup.Error != nil {
			return nil, lookup.Error
		}
		if lookup.RowsAffected == 0 {
			continue
		}
		view := ComparisonView{Comparison: comparison, Counterpart: counterpart}
		if len(comparison.RoundOnePayload) > 0 {
			raw, err := decodePayload(comparison.RoundOnePayload, comparison.RoundOneEncoding, comparison.RoundOneHash)
			if err != nil {
				return nil, err
			}
			view.RoundOne = json.RawMessage(raw)
		}
		if len(comparison.RoundTwoPayload) > 0 {
			raw, err := decodePayload(comparison.RoundTwoPayload, comparison.RoundTwoEncoding, comparison.RoundTwoHash)
			if err != nil {
				return nil, err
			}
			view.RoundTwo = json.RawMessage(raw)
		}
		var proposal db.SolutionStandardizationProposal
		proposalLookup := module.conn.WithContext(ctx).Where("comparison_id = ?", comparison.ID).Limit(1).Find(&proposal)
		if proposalLookup.Error != nil {
			return nil, proposalLookup.Error
		}
		if proposalLookup.RowsAffected > 0 {
			markdown, err := decodePayload(proposal.Content, proposal.ContentEncoding, proposal.ContentHash)
			if err != nil {
				return nil, err
			}
			view.Proposal = &StandardizationProposalView{Proposal: proposal, Markdown: string(markdown)}
		}
		views = append(views, view)
	}
	return views, nil
}

func (module *Module) ListStandards(ctx context.Context, scope []string) ([]StandardListItem, error) {
	var standards []db.SolutionStandard
	if err := module.conn.WithContext(ctx).Where("status = ?", StandardActive).Order("updated_at DESC, id DESC").Find(&standards).Error; err != nil {
		return nil, err
	}
	items := make([]StandardListItem, 0, len(standards))
	for _, standard := range standards {
		var revision db.SolutionStandardRevision
		if err := module.conn.WithContext(ctx).First(&revision, standard.CurrentRevisionID).Error; err != nil {
			return nil, err
		}
		projects, allowed, err := module.standardProjects(ctx, revision.ProposalID, scope)
		if err != nil {
			return nil, err
		}
		if !allowed {
			continue
		}
		items = append(items, StandardListItem{Standard: standard, Version: revision.Version, Projects: projects, UpdatedAt: standard.UpdatedAt})
	}
	return items, nil
}

func (module *Module) GetStandard(ctx context.Context, id uint, scope []string) (StandardDetail, error) {
	var standard db.SolutionStandard
	lookup := module.conn.WithContext(ctx).Where("id = ? AND status = ?", id, StandardActive).Limit(1).Find(&standard)
	if lookup.Error != nil {
		return StandardDetail{}, lookup.Error
	}
	if lookup.RowsAffected == 0 {
		return StandardDetail{}, ErrNotFound
	}
	var revision db.SolutionStandardRevision
	if err := module.conn.WithContext(ctx).First(&revision, standard.CurrentRevisionID).Error; err != nil {
		return StandardDetail{}, err
	}
	projects, allowed, err := module.standardProjects(ctx, revision.ProposalID, scope)
	if err != nil {
		return StandardDetail{}, err
	}
	if !allowed {
		return StandardDetail{}, ErrNotFound
	}
	raw, err := decodePayload(revision.Content, revision.ContentEncoding, revision.ContentHash)
	if err != nil {
		return StandardDetail{}, err
	}
	return StandardDetail{Standard: standard, Revision: revision, Markdown: string(raw), Projects: projects}, nil
}

func (module *Module) standardProjects(ctx context.Context, proposalID uint, scope []string) ([]string, bool, error) {
	var proposal db.SolutionStandardizationProposal
	if err := module.conn.WithContext(ctx).First(&proposal, proposalID).Error; err != nil {
		return nil, false, err
	}
	var comparison db.SolutionComparison
	if err := module.conn.WithContext(ctx).First(&comparison, proposal.ComparisonID).Error; err != nil {
		return nil, false, err
	}
	var entries []db.SolutionCatalogEntry
	if err := module.conn.WithContext(ctx).Where("id IN ?", []uint{comparison.LeftEntryID, comparison.RightEntryID}).Find(&entries).Error; err != nil {
		return nil, false, err
	}
	if len(entries) != 2 {
		return nil, false, ErrIntegrity
	}
	projects := db.NormalizeProjectKeys([]string{entries[0].ProjectKey, entries[1].ProjectKey})
	for _, project := range projects {
		if !allowedProject(project, scope) {
			return nil, false, nil
		}
	}
	return projects, true, nil
}
