package reviewread

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
	"well-ambient/internal/codereview"
	"well-ambient/internal/db"
	"well-ambient/internal/openaccess"
	"well-ambient/internal/readmodel"
)

const (
	SchemaVersion = "open-review/v1"
	ToolVersion   = "1"
)

type Meta struct {
	RequestID        string    `json:"request_id,omitempty"`
	SchemaVersion    string    `json:"schema_version"`
	PolicyVersion    int       `json:"policy_version"`
	DataAsOf         time.Time `json:"data_as_of"`
	QuerySnapshotRef string    `json:"query_snapshot_ref"`
	MissingReasons   []string  `json:"missing_reasons"`
	SourceRefs       []string  `json:"source_refs,omitempty"`
}

type SearchRequest struct {
	Repositories []string   `json:"repositories,omitempty"`
	Kind         string     `json:"kind,omitempty"`
	Ref          string     `json:"ref,omitempty"`
	HeadSHA      string     `json:"head_sha,omitempty"`
	BaseSHA      string     `json:"base_sha,omitempty"`
	Statuses     []string   `json:"statuses,omitempty"`
	CreatedFrom  *time.Time `json:"created_from,omitempty"`
	CreatedUntil *time.Time `json:"created_until,omitempty"`
	Cursor       string     `json:"cursor,omitempty"`
	Limit        int        `json:"limit,omitempty"`
}

type SearchResponse struct {
	Items      []Summary `json:"items"`
	NextCursor string    `json:"next_cursor,omitempty"`
	HasMore    bool      `json:"has_more"`
	Meta       Meta      `json:"meta"`
}

type Summary struct {
	RunID            uint       `json:"run_id"`
	Repository       string     `json:"repository"`
	Kind             string     `json:"kind"`
	Ref              string     `json:"ref"`
	BaseSHA          string     `json:"base_sha"`
	HeadSHA          string     `json:"head_sha"`
	Title            string     `json:"title"`
	RunStatus        string     `json:"run_status"`
	ReviewedAt       time.Time  `json:"reviewed_at"`
	CoverageGaps     []string   `json:"coverage_gaps"`
	EvidenceComplete bool       `json:"evidence_complete"`
	FindingsCount    int        `json:"findings_count"`
	CodeFreshness    string     `json:"code_freshness"`
	FreshnessChecked *time.Time `json:"freshness_checked_at,omitempty"`
}

type GetRequest struct {
	RunID          uint     `json:"run_id"`
	Severities     []string `json:"severities,omitempty"`
	Dimensions     []string `json:"dimensions,omitempty"`
	Files          []string `json:"files,omitempty"`
	CurrentHeadSHA string   `json:"current_head_sha,omitempty"`
	Offset         int      `json:"offset,omitempty"`
	Limit          int      `json:"limit,omitempty"`
}

type Finding struct {
	Dimension    string   `json:"dimension"`
	Severity     string   `json:"severity"`
	Title        string   `json:"title"`
	File         string   `json:"file"`
	Line         int      `json:"line"`
	Evidence     string   `json:"evidence"`
	Impact       string   `json:"impact"`
	Suggestion   string   `json:"suggestion"`
	Verification string   `json:"verification"`
	KnowledgeIDs []uint   `json:"knowledge_ids"`
	EvidenceRefs []string `json:"evidence_refs"`
}

type Detail struct {
	Summary    Summary   `json:"summary"`
	Overview   string    `json:"overview"`
	Scenario   string    `json:"scenario,omitempty"`
	Validation string    `json:"validation,omitempty"`
	Questions  []string  `json:"questions"`
	Findings   []Finding `json:"findings"`
	Total      int       `json:"total"`
	HasMore    bool      `json:"has_more"`
	Meta       Meta      `json:"meta"`
}

type Module struct {
	db  *gorm.DB
	now func() time.Time
}

var (
	reviewCredentialPattern = regexp.MustCompile(`(?i)\b(authorization|api[_ -]?key|token|password|passwd|secret)\s*[:=]\s*[^\s,;]+`)
	reviewBearerPattern     = regexp.MustCompile(`(?i)\bbearer\s+[A-Za-z0-9._~+/=-]+`)
	reviewPrivateKeyPattern = regexp.MustCompile(`(?is)-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----.*?-----END [A-Z0-9 ]*PRIVATE KEY-----`)
)

func New(database *gorm.DB) *Module {
	return &Module{db: database, now: time.Now}
}

func (m *Module) Search(ctx context.Context, policy openaccess.Policy, requestID string, request SearchRequest) (SearchResponse, error) {
	if err := authorizeReviewRead(policy, "review_search", []string{
		"run_id", "repository", "kind", "ref", "base_sha", "head_sha", "title",
		"run_status", "reviewed_at", "coverage_gaps", "evidence_complete",
		"findings_count", "code_freshness",
	}); err != nil {
		return SearchResponse{}, err
	}
	statuses, err := terminalReviewStatuses(request.Statuses)
	if err != nil {
		return SearchResponse{}, err
	}
	request.Statuses = statuses
	repositories, err := authorizedRepositories(policy, request.Repositories)
	if err != nil {
		return SearchResponse{}, err
	}
	limit := request.Limit
	if limit <= 0 {
		limit = min(50, policy.QueryLimits.MaxPageSize)
	}
	if limit > policy.QueryLimits.MaxPageSize {
		limit = policy.QueryLimits.MaxPageSize
	}
	scope := struct {
		PolicyVersion int
		Repositories  []string
		Kind          string
		Ref           string
		HeadSHA       string
		BaseSHA       string
		Statuses      []string
		CreatedFrom   *time.Time
		CreatedUntil  *time.Time
	}{
		policy.Version, repositories, strings.ToLower(strings.TrimSpace(request.Kind)),
		strings.TrimSpace(request.Ref), strings.TrimSpace(request.HeadSHA),
		strings.TrimSpace(request.BaseSHA), normalized(request.Statuses),
		request.CreatedFrom, request.CreatedUntil,
	}
	codec, err := readmodel.NewCursorCodec("open-review-search/v1", scope)
	if err != nil {
		return SearchResponse{}, err
	}
	dataAsOf, err := m.dataAsOf(ctx, repositories)
	if err != nil {
		return SearchResponse{}, err
	}
	watermark := dataAsOf.UTC().Format(time.RFC3339Nano)
	position := searchPosition{}
	if request.Cursor != "" {
		cursorWatermark, decodeErr := codec.Decode(request.Cursor, &position)
		if decodeErr != nil {
			return SearchResponse{}, openaccess.NewError("invalid_query", "cursor is invalid for this review query")
		}
		parsed, parseErr := time.Parse(time.RFC3339Nano, cursorWatermark)
		if parseErr != nil {
			return SearchResponse{}, openaccess.NewError("invalid_query", "cursor watermark is invalid")
		}
		dataAsOf = parsed
		watermark = cursorWatermark
	}
	query := m.filteredQuery(ctx, repositories, request).Where("created_at <= ?", dataAsOf)
	if position.ID > 0 {
		query = query.Where(
			"(created_at < ?) OR (created_at = ? AND id < ?)",
			position.CreatedAt, position.CreatedAt, position.ID,
		)
	}
	var runs []db.CodeReviewRun
	if err := query.Order("created_at DESC, id DESC").Limit(limit + 1).Find(&runs).Error; err != nil {
		return SearchResponse{}, fmt.Errorf("search review reports: %w", err)
	}
	hasMore := len(runs) > limit
	if hasMore {
		runs = runs[:limit]
	}
	items := make([]Summary, 0, len(runs))
	refs := make([]string, 0, len(runs))
	missing := []string{}
	for _, run := range runs {
		var report codereview.Report
		var reportRef *codereview.Report
		decodeMissing := []string{}
		if strings.TrimSpace(run.ReportJSON) != "" {
			if err := json.Unmarshal([]byte(run.ReportJSON), &report); err != nil {
				decodeMissing = append(decodeMissing, "stored review report is not readable")
			} else {
				reportRef = &report
			}
		}
		summary, summaryMissing := summarize(run, "", reportRef, m.now().UTC())
		decodeMissing = append(decodeMissing, summaryMissing...)
		var snapshot struct {
			Complete bool     `json:"complete"`
			Gaps     []string `json:"gaps"`
		}
		if strings.TrimSpace(run.SnapshotJSON) != "" &&
			json.Unmarshal([]byte(run.SnapshotJSON), &snapshot) == nil &&
			!snapshot.Complete {
			summary.CoverageGaps = append(summary.CoverageGaps, snapshot.Gaps...)
		}
		items = append(items, summary)
		missing = append(missing, decodeMissing...)
		refs = append(refs, reviewSourceRef(run))
	}
	var nextCursor string
	if hasMore && len(runs) > 0 {
		last := runs[len(runs)-1]
		nextCursor, err = codec.Encode(watermark, searchPosition{CreatedAt: last.CreatedAt, ID: last.ID})
		if err != nil {
			return SearchResponse{}, err
		}
	}
	return SearchResponse{
		Items: items, NextCursor: nextCursor, HasMore: hasMore,
		Meta: buildMeta(requestID, policy, dataAsOf, scope, missing, refs),
	}, nil
}

func (m *Module) Get(ctx context.Context, policy openaccess.Policy, requestID string, request GetRequest) (Detail, error) {
	if err := authorizeReviewRead(policy, "review_get", []string{
		"run_id", "repository", "kind", "ref", "base_sha", "head_sha", "title",
		"run_status", "reviewed_at", "coverage_gaps", "evidence_complete",
		"findings_count", "code_freshness", "overview", "scenario", "validation",
		"questions", "findings", "evidence_refs",
	}); err != nil {
		return Detail{}, err
	}
	if request.RunID == 0 {
		return Detail{}, openaccess.NewError("invalid_query", "review run id is required")
	}
	if len(policy.AllowedRepositories) == 0 {
		return Detail{}, openaccess.NewError("forbidden", "review reading has no published repository scope")
	}
	var run db.CodeReviewRun
	err := m.db.WithContext(ctx).
		Where(
			"id = ? AND LOWER(project_id) IN ? AND LOWER(status) IN ?",
			request.RunID, lower(policy.AllowedRepositories), []string{"completed", "partial", "failed"},
		).
		First(&run).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Detail{}, openaccess.NewError("not_found", "review report was not found")
	}
	if err != nil {
		return Detail{}, fmt.Errorf("load review report: %w", err)
	}
	var report codereview.Report
	missing := []string{}
	if strings.TrimSpace(run.ReportJSON) != "" {
		if err := json.Unmarshal([]byte(run.ReportJSON), &report); err != nil {
			missing = append(missing, "stored review report is not readable")
		}
	}
	var snapshot struct {
		Complete bool     `json:"complete"`
		Gaps     []string `json:"gaps"`
	}
	if strings.TrimSpace(run.SnapshotJSON) != "" {
		if err := json.Unmarshal([]byte(run.SnapshotJSON), &snapshot); err != nil {
			missing = append(missing, "stored review coverage is not readable")
		}
	}
	summary, summaryMissing := summarize(run, request.CurrentHeadSHA, &report, m.now().UTC())
	if !snapshot.Complete {
		summary.CoverageGaps = append(summary.CoverageGaps, snapshot.Gaps...)
	}
	missing = append(missing, summaryMissing...)
	if run.Status == "partial" && len(summary.CoverageGaps) == 0 {
		summary.CoverageGaps = append(summary.CoverageGaps, "review run completed with partial coverage")
	}
	filtered := filterFindings(report.Findings, request)
	total := len(filtered)
	offset := max(0, request.Offset)
	limit := request.Limit
	if limit <= 0 {
		limit = min(100, policy.QueryLimits.MaxPageSize)
	}
	if limit > policy.QueryLimits.MaxPageSize {
		limit = policy.QueryLimits.MaxPageSize
	}
	if offset > len(filtered) {
		offset = len(filtered)
	}
	end := min(len(filtered), offset+limit)
	items := make([]Finding, 0, end-offset)
	refs := []string{reviewSourceRef(run)}
	for _, source := range filtered[offset:end] {
		item := Finding{
			Dimension: publicReviewText(source.Dimension, 64),
			Severity:  publicReviewText(source.Severity, 32),
			Title:     publicReviewText(source.Title, 500),
			File:      publicReviewText(source.File, 512), Line: source.Line,
			Evidence:     publicReviewText(source.Evidence, 2000),
			Impact:       publicReviewText(source.Impact, 2000),
			Suggestion:   publicReviewText(source.Suggestion, 2000),
			Verification: publicReviewText(source.Verification, 2000),
			KnowledgeIDs: append([]uint(nil), source.KnowledgeIDs...),
		}
		item.EvidenceRefs = append(item.EvidenceRefs, fmt.Sprintf(
			"review:%d:file:%s:line:%d", run.ID, source.File, source.Line,
		))
		for _, id := range source.KnowledgeIDs {
			item.EvidenceRefs = append(item.EvidenceRefs, "knowledge:"+strconv.FormatUint(uint64(id), 10))
		}
		refs = append(refs, item.EvidenceRefs...)
		items = append(items, item)
	}
	questions := make([]string, 0, len(report.Questions))
	for _, question := range report.Questions {
		questions = append(questions, publicReviewText(question, 1500))
	}
	dataAsOf := run.UpdatedAt
	if dataAsOf.IsZero() {
		dataAsOf = run.CreatedAt
	}
	return Detail{
		Summary: summary, Overview: publicReviewText(report.Summary, 4000),
		Scenario:   publicReviewText(report.Scenario, 160),
		Validation: publicReviewText(report.Validation, 2000),
		Questions:  questions, Findings: items,
		Total: total, HasMore: end < total,
		Meta: buildMeta(requestID, policy, dataAsOf, request, missing, refs),
	}, nil
}

func publicReviewText(value string, maxRunes int) string {
	value = strings.TrimSpace(strings.ToValidUTF8(value, "\uFFFD"))
	value = reviewPrivateKeyPattern.ReplaceAllString(value, "[REDACTED PRIVATE KEY]")
	value = reviewBearerPattern.ReplaceAllString(value, "Bearer [REDACTED]")
	value = reviewCredentialPattern.ReplaceAllString(value, "$1=[REDACTED]")
	runes := []rune(value)
	if maxRunes > 0 && len(runes) > maxRunes {
		value = string(runes[:maxRunes])
	}
	return value
}

type searchPosition struct {
	CreatedAt time.Time `json:"created_at"`
	ID        uint      `json:"id"`
}

func (m *Module) filteredQuery(ctx context.Context, repositories []string, request SearchRequest) *gorm.DB {
	query := m.db.WithContext(ctx).Model(&db.CodeReviewRun{}).
		Where("LOWER(project_id) IN ?", lower(repositories))
	if value := strings.ToLower(strings.TrimSpace(request.Kind)); value != "" {
		query = query.Where("LOWER(kind) = ?", value)
	}
	if value := strings.TrimSpace(request.Ref); value != "" {
		query = query.Where("ref = ?", value)
	}
	if value := strings.TrimSpace(request.HeadSHA); value != "" {
		query = query.Where("head_sha = ?", value)
	}
	if value := strings.TrimSpace(request.BaseSHA); value != "" {
		query = query.Where("base_sha = ?", value)
	}
	if values := normalized(request.Statuses); len(values) > 0 {
		query = query.Where("LOWER(status) IN ?", values)
	}
	if request.CreatedFrom != nil {
		query = query.Where("created_at >= ?", request.CreatedFrom.UTC())
	}
	if request.CreatedUntil != nil {
		query = query.Where("created_at < ?", request.CreatedUntil.UTC())
	}
	return query
}

func (m *Module) dataAsOf(ctx context.Context, repositories []string) (time.Time, error) {
	var run db.CodeReviewRun
	err := m.db.WithContext(ctx).
		Select("created_at").
		Where("LOWER(project_id) IN ?", lower(repositories)).
		Order("created_at DESC").
		First(&run).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return m.now().UTC(), nil
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("read review data watermark: %w", err)
	}
	if run.CreatedAt.IsZero() {
		return m.now().UTC(), nil
	}
	return run.CreatedAt.UTC(), nil
}

func authorizedRepositories(policy openaccess.Policy, requested []string) ([]string, error) {
	if len(policy.AllowedRepositories) == 0 {
		return nil, openaccess.NewError("forbidden", "review reading has no published repository scope")
	}
	if len(requested) == 0 {
		return normalized(policy.AllowedRepositories), nil
	}
	result := normalized(requested)
	for _, repository := range result {
		if !policy.AllowsRepository(repository) {
			return nil, openaccess.NewError("forbidden", "requested review repository is not published")
		}
	}
	return result, nil
}

func authorizeReviewRead(policy openaccess.Policy, capability string, fields []string) error {
	if !policy.AllowsAction("review.read") {
		return openaccess.NewError("forbidden", "review reading is not allowed by the active policy")
	}
	if !policy.AllowsFields(capability, fields) {
		return openaccess.NewError("forbidden", "one or more review fields are not published")
	}
	return nil
}

func summarize(run db.CodeReviewRun, currentHead string, report *codereview.Report, now time.Time) (Summary, []string) {
	missing := []string{}
	summary := Summary{
		RunID: run.ID, Repository: run.Repo, Kind: run.Kind, Ref: run.Ref,
		BaseSHA: run.BaseSHA, HeadSHA: run.HeadSHA, Title: run.Title,
		RunStatus: normalizedRunStatus(run.Status), ReviewedAt: run.UpdatedAt,
		CodeFreshness: "not_checked", CoverageGaps: []string{},
	}
	if summary.ReviewedAt.IsZero() {
		summary.ReviewedAt = run.CreatedAt
	}
	if report != nil {
		summary.EvidenceComplete = report.EvidenceComplete
		summary.FindingsCount = len(report.Findings)
		if !report.EvidenceComplete {
			summary.CoverageGaps = append(summary.CoverageGaps, "one or more findings did not pass evidence validation")
		}
	}
	currentHead = strings.TrimSpace(currentHead)
	if currentHead != "" {
		summary.FreshnessChecked = &now
		if strings.EqualFold(currentHead, run.HeadSHA) {
			summary.CodeFreshness = "matches_current_head"
		} else {
			summary.CodeFreshness = "outdated"
		}
	}
	if run.Status == "failed" {
		missing = append(missing, "review run failed; internal error details are not published")
	}
	return summary, missing
}

func normalizedRunStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "completed", "partial", "failed":
		return strings.ToLower(strings.TrimSpace(status))
	default:
		return strings.ToLower(strings.TrimSpace(status))
	}
}

func terminalReviewStatuses(values []string) ([]string, error) {
	if len(values) == 0 {
		return []string{"completed", "failed", "partial"}, nil
	}
	result := normalized(values)
	for _, value := range result {
		switch value {
		case "completed", "partial", "failed":
		default:
			return nil, openaccess.NewError("invalid_query", "review search only publishes completed, partial, or failed reports")
		}
	}
	return result, nil
}

func filterFindings(findings []codereview.Finding, request GetRequest) []codereview.Finding {
	severities := setOf(request.Severities)
	dimensions := setOf(request.Dimensions)
	files := setOfExact(request.Files)
	result := make([]codereview.Finding, 0, len(findings))
	for _, finding := range findings {
		if len(severities) > 0 && !severities[strings.ToLower(finding.Severity)] {
			continue
		}
		if len(dimensions) > 0 && !dimensions[strings.ToLower(finding.Dimension)] {
			continue
		}
		if len(files) > 0 && !files[finding.File] {
			continue
		}
		result = append(result, finding)
	}
	return result
}

func setOf(values []string) map[string]bool {
	result := make(map[string]bool, len(values))
	for _, value := range values {
		if value = strings.ToLower(strings.TrimSpace(value)); value != "" {
			result[value] = true
		}
	}
	return result
}

func setOfExact(values []string) map[string]bool {
	result := make(map[string]bool, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			result[value] = true
		}
	}
	return result
}

func normalized(values []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result
}

func lower(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, strings.ToLower(strings.TrimSpace(value)))
	}
	return result
}

func reviewSourceRef(run db.CodeReviewRun) string {
	return fmt.Sprintf("review-run:%d@%s", run.ID, run.HeadSHA)
}

func buildMeta(
	requestID string,
	policy openaccess.Policy,
	dataAsOf time.Time,
	query any,
	missing, refs []string,
) Meta {
	if missing == nil {
		missing = []string{}
	}
	if refs == nil {
		refs = []string{}
	}
	payload, _ := json.Marshal(struct {
		PolicyVersion int
		DataAsOf      time.Time
		Query         any
	}{policy.Version, dataAsOf.UTC(), query})
	sum := sha256.Sum256(payload)
	return Meta{
		RequestID: requestID, SchemaVersion: SchemaVersion, PolicyVersion: policy.Version,
		DataAsOf: dataAsOf.UTC(), QuerySnapshotRef: "review-query:" + hex.EncodeToString(sum[:]),
		MissingReasons: unique(missing), SourceRefs: unique(refs),
	}
}

func unique(values []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}
