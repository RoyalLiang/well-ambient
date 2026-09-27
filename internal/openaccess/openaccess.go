package openaccess

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"well-ambient/internal/db"
)

const (
	SourceActive   = "active"
	SourceDisabled = "disabled"

	CredentialActive   = "active"
	CredentialDisabled = "disabled"
	CredentialRevoked  = "revoked"

	PolicyActive  = "active"
	PolicyRetired = "retired"

	DefaultQuotaProfile = "standard"
)

type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string { return e.Message }

func newError(code, message string) error {
	return &Error{Code: code, Message: message}
}

func NewError(code, message string) error {
	return newError(code, message)
}

func ErrorCode(err error) string {
	var target *Error
	if errors.As(err, &target) {
		return target.Code
	}
	return "internal_error"
}

type QueryLimits struct {
	MaxPageSize    int `json:"max_page_size" yaml:"max_page_size"`
	MaxScanRows    int `json:"max_scan_rows" yaml:"max_scan_rows"`
	MaxGroupBy     int `json:"max_group_by" yaml:"max_group_by"`
	MaxTimeBuckets int `json:"max_time_buckets" yaml:"max_time_buckets"`
}

func (l QueryLimits) normalized() QueryLimits {
	if l.MaxPageSize <= 0 || l.MaxPageSize > 500 {
		l.MaxPageSize = 100
	}
	if l.MaxScanRows <= 0 || l.MaxScanRows > 100000 {
		l.MaxScanRows = 10000
	}
	if l.MaxGroupBy <= 0 || l.MaxGroupBy > 4 {
		l.MaxGroupBy = 2
	}
	if l.MaxTimeBuckets <= 0 || l.MaxTimeBuckets > 366 {
		l.MaxTimeBuckets = 120
	}
	return l
}

type PolicySpec struct {
	AllowedProjects     []string            `json:"allowed_projects" yaml:"allowed_projects"`
	AllowedRepositories []string            `json:"allowed_repositories" yaml:"allowed_repositories"`
	Actions             []string            `json:"actions" yaml:"actions"`
	FieldRules          map[string][]string `json:"field_rules" yaml:"field_rules"`
	DataRules           map[string]string   `json:"data_rules" yaml:"data_rules"`
	ApprovalRules       map[string]string   `json:"approval_rules" yaml:"approval_rules"`
	QueryLimits         QueryLimits         `json:"query_limits" yaml:"query_limits"`
}

type Policy struct {
	Version int    `json:"version"`
	Digest  string `json:"digest"`
	PolicySpec
}

func (p Policy) AllowsProject(project string) bool {
	return containsFold(p.AllowedProjects, project)
}

func (p Policy) AllowsRepository(repository string) bool {
	return containsFold(p.AllowedRepositories, repository)
}

func (p Policy) AllowsAction(action string) bool {
	return containsFold(p.Actions, action)
}

func (p Policy) AllowsFields(capability string, fields []string) bool {
	allowed := p.FieldRules[strings.ToLower(strings.TrimSpace(capability))]
	if len(allowed) == 0 {
		return len(fields) == 0
	}
	for _, field := range fields {
		if !containsFold(allowed, field) {
			return false
		}
	}
	return true
}

func (p Policy) JiraCachePublicationVerified() bool {
	return strings.EqualFold(strings.TrimSpace(p.DataRules["jira_issue_visibility"]), "verified_cache")
}

func (p Policy) AllowsRestrictedJiraIssues() bool {
	return strings.EqualFold(strings.TrimSpace(p.DataRules["jira_restricted_issues"]), "allow")
}

func containsFold(values []string, candidate string) bool {
	candidate = strings.TrimSpace(candidate)
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), candidate) {
			return true
		}
	}
	return false
}

type Identity struct {
	SourceID     string `json:"source_id"`
	SourceName   string `json:"source_name"`
	KeyID        string `json:"key_id"`
	QuotaProfile string `json:"quota_profile"`
}

type CallContext struct {
	Identity  Identity
	Policy    Policy
	RequestID string
	Transport string
}

type callContextKey struct{}

func WithCallContext(ctx context.Context, value CallContext) context.Context {
	return context.WithValue(ctx, callContextKey{}, value)
}

func CallContextFrom(ctx context.Context) (CallContext, bool) {
	value, ok := ctx.Value(callContextKey{}).(CallContext)
	return value, ok
}

type IssuedCredential struct {
	KeyID     string     `json:"key_id"`
	Prefix    string     `json:"prefix"`
	Secret    string     `json:"secret"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

type Invocation struct {
	RequestID     string
	Identity      Identity
	Transport     string
	ToolName      string
	ToolVersion   string
	PolicyVersion int
	TargetRefs    []string
	Duration      time.Duration
	Outcome       string
	ErrorCode     string
}

type Service struct {
	db     *gorm.DB
	now    func() time.Time
	random io.Reader
}

func New(database *gorm.DB) *Service {
	return &Service{db: database, now: time.Now, random: rand.Reader}
}

func (s *Service) CreateSource(ctx context.Context, id, name, owner, quotaProfile string) (db.IntegrationSource, error) {
	if s == nil || s.db == nil {
		return db.IntegrationSource{}, newError("unavailable", "integration access storage is unavailable")
	}
	id = strings.TrimSpace(id)
	name = strings.TrimSpace(name)
	if id == "" || name == "" {
		return db.IntegrationSource{}, newError("invalid_request", "source id and name are required")
	}
	if quotaProfile == "" {
		quotaProfile = DefaultQuotaProfile
	}
	source := db.IntegrationSource{
		ID: id, Name: name, Owner: strings.TrimSpace(owner),
		Status: SourceActive, QuotaProfile: quotaProfile,
	}
	if err := s.db.WithContext(ctx).Create(&source).Error; err != nil {
		return db.IntegrationSource{}, fmt.Errorf("create integration source: %w", err)
	}
	return source, nil
}

func (s *Service) SetSourceStatus(ctx context.Context, sourceID, status string) error {
	if status != SourceActive && status != SourceDisabled {
		return newError("invalid_request", "source status must be active or disabled")
	}
	result := s.db.WithContext(ctx).Model(&db.IntegrationSource{}).
		Where("id = ?", strings.TrimSpace(sourceID)).
		Update("status", status)
	if result.Error != nil {
		return fmt.Errorf("update integration source: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return newError("not_found", "integration source was not found")
	}
	return nil
}

func (s *Service) Source(ctx context.Context, sourceID string) (db.IntegrationSource, error) {
	var source db.IntegrationSource
	if err := s.db.WithContext(ctx).First(&source, "id = ?", strings.TrimSpace(sourceID)).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return db.IntegrationSource{}, newError("forbidden", "integration source is unavailable")
		}
		return db.IntegrationSource{}, fmt.Errorf("load integration source: %w", err)
	}
	if source.Status != SourceActive {
		return db.IntegrationSource{}, newError("forbidden", "integration source is disabled")
	}
	return source, nil
}

func (s *Service) IssueCredential(ctx context.Context, sourceID string, ttl time.Duration) (IssuedCredential, error) {
	if ttl <= 0 {
		return IssuedCredential{}, newError("invalid_request", "credential ttl must be positive")
	}
	var source db.IntegrationSource
	if err := s.db.WithContext(ctx).First(&source, "id = ?", strings.TrimSpace(sourceID)).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return IssuedCredential{}, newError("not_found", "integration source was not found")
		}
		return IssuedCredential{}, fmt.Errorf("load integration source: %w", err)
	}
	if source.Status != SourceActive {
		return IssuedCredential{}, newError("forbidden", "integration source is disabled")
	}

	keyID, err := randomToken(s.random, 9)
	if err != nil {
		return IssuedCredential{}, fmt.Errorf("generate credential id: %w", err)
	}
	secretPart, err := randomToken(s.random, 32)
	if err != nil {
		return IssuedCredential{}, fmt.Errorf("generate credential secret: %w", err)
	}
	prefix := "wa_live_" + keyID
	secret := prefix + "." + secretPart
	now := s.now().UTC()
	expiresAtValue := now.Add(ttl)
	expiresAt := &expiresAtValue
	credential := db.IntegrationCredential{
		KeyID: keyID, SourceID: source.ID, Prefix: prefix,
		Verifier: credentialVerifier(secret), Status: CredentialActive,
		ExpiresAt: expiresAt, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.db.WithContext(ctx).Create(&credential).Error; err != nil {
		return IssuedCredential{}, fmt.Errorf("persist integration credential: %w", err)
	}
	return IssuedCredential{KeyID: keyID, Prefix: prefix, Secret: secret, ExpiresAt: expiresAt}, nil
}

func (s *Service) SetCredentialStatus(ctx context.Context, keyID, status string) error {
	if status != CredentialActive && status != CredentialDisabled {
		return newError("invalid_request", "credential status must be active or disabled")
	}
	var credential db.IntegrationCredential
	if err := s.db.WithContext(ctx).First(&credential, "key_id = ?", strings.TrimSpace(keyID)).Error; err != nil {
		return newError("not_found", "integration credential was not found")
	}
	if credential.Status == CredentialRevoked || credential.RevokedAt != nil {
		return newError("forbidden", "revoked integration credentials cannot be re-enabled")
	}
	if credential.ExpiresAt != nil && !credential.ExpiresAt.After(s.now().UTC()) {
		return newError("forbidden", "expired integration credentials cannot be re-enabled")
	}
	return s.db.WithContext(ctx).Model(&db.IntegrationCredential{}).
		Where("id = ?", credential.ID).
		Updates(map[string]any{"status": status, "updated_at": s.now().UTC()}).Error
}

func (s *Service) Credentials(ctx context.Context, sourceID string) ([]db.IntegrationCredential, error) {
	query := s.db.WithContext(ctx).Order("created_at DESC, id DESC")
	if strings.TrimSpace(sourceID) != "" {
		query = query.Where("source_id = ?", strings.TrimSpace(sourceID))
	}
	var credentials []db.IntegrationCredential
	if err := query.Find(&credentials).Error; err != nil {
		return nil, fmt.Errorf("list integration credentials: %w", err)
	}
	return credentials, nil
}

func (s *Service) Credential(ctx context.Context, keyID string) (db.IntegrationCredential, error) {
	var credential db.IntegrationCredential
	if err := s.db.WithContext(ctx).First(&credential, "key_id = ?", strings.TrimSpace(keyID)).Error; err != nil {
		return db.IntegrationCredential{}, newError("not_found", "integration credential was not found")
	}
	return credential, nil
}

func (s *Service) RevokeCredential(ctx context.Context, keyID string) error {
	now := s.now().UTC()
	result := s.db.WithContext(ctx).Model(&db.IntegrationCredential{}).
		Where("key_id = ? AND status <> ?", strings.TrimSpace(keyID), CredentialRevoked).
		Updates(map[string]any{"status": CredentialRevoked, "revoked_at": now, "updated_at": now})
	if result.Error != nil {
		return fmt.Errorf("revoke integration credential: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return newError("not_found", "active integration credential was not found")
	}
	return nil
}

func (s *Service) CredentialActive(ctx context.Context, sourceID, keyID string) error {
	var credential db.IntegrationCredential
	if err := s.db.WithContext(ctx).
		Where("source_id = ? AND key_id = ?", strings.TrimSpace(sourceID), strings.TrimSpace(keyID)).
		First(&credential).Error; err != nil {
		return newError("forbidden", "integration credential is unavailable")
	}
	now := s.now().UTC()
	if credential.Status != CredentialActive || credential.RevokedAt != nil ||
		(credential.ExpiresAt != nil && !credential.ExpiresAt.After(now)) {
		return newError("forbidden", "integration credential is inactive")
	}
	return nil
}

func (s *Service) Authenticate(ctx context.Context, secret string) (Identity, error) {
	keyID, ok := parseCredential(secret)
	if !ok {
		return Identity{}, newError("unauthenticated", "invalid integration credential")
	}
	var credential db.IntegrationCredential
	if err := s.db.WithContext(ctx).First(&credential, "key_id = ?", keyID).Error; err != nil {
		return Identity{}, newError("unauthenticated", "invalid integration credential")
	}
	actual, err := hex.DecodeString(credential.Verifier)
	if err != nil {
		return Identity{}, newError("unauthenticated", "invalid integration credential")
	}
	expectedHash := sha256.Sum256([]byte(secret))
	if subtle.ConstantTimeCompare(actual, expectedHash[:]) != 1 {
		return Identity{}, newError("unauthenticated", "invalid integration credential")
	}
	now := s.now().UTC()
	if credential.Status != CredentialActive || credential.RevokedAt != nil ||
		(credential.ExpiresAt != nil && !credential.ExpiresAt.After(now)) {
		return Identity{}, newError("unauthenticated", "integration credential is inactive")
	}
	var source db.IntegrationSource
	if err := s.db.WithContext(ctx).First(&source, "id = ?", credential.SourceID).Error; err != nil {
		return Identity{}, newError("unauthenticated", "integration source is unavailable")
	}
	if source.Status != SourceActive {
		return Identity{}, newError("forbidden", "integration source is disabled")
	}
	_ = s.db.WithContext(ctx).Model(&db.IntegrationCredential{}).
		Where("id = ?", credential.ID).
		Updates(map[string]any{"last_used_at": now, "updated_at": now}).Error
	return Identity{
		SourceID: source.ID, SourceName: source.Name, KeyID: credential.KeyID,
		QuotaProfile: source.QuotaProfile,
	}, nil
}

func (s *Service) ActivatePolicy(ctx context.Context, spec PolicySpec, actor string) (Policy, error) {
	normalized, err := normalizePolicy(spec)
	if err != nil {
		return Policy{}, err
	}
	allowedProjects, _ := json.Marshal(normalized.AllowedProjects)
	allowedRepositories, _ := json.Marshal(normalized.AllowedRepositories)
	actions, _ := json.Marshal(normalized.Actions)
	fieldRules, _ := json.Marshal(normalized.FieldRules)
	dataRules, _ := json.Marshal(normalized.DataRules)
	approvalRules, _ := json.Marshal(normalized.ApprovalRules)
	queryLimits, _ := json.Marshal(normalized.QueryLimits)
	digest := policyDigest(normalized)
	now := s.now().UTC()
	var result Policy
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var maxVersion int
		if err := tx.Model(&db.IntegrationPolicyVersion{}).
			Select("COALESCE(MAX(version), 0)").Scan(&maxVersion).Error; err != nil {
			return err
		}
		if err := tx.Model(&db.IntegrationPolicyVersion{}).
			Where("status = ?", PolicyActive).
			Update("status", PolicyRetired).Error; err != nil {
			return err
		}
		record := db.IntegrationPolicyVersion{
			Version: maxVersion + 1, Status: PolicyActive,
			AllowedProjectsJSON:     string(allowedProjects),
			AllowedRepositoriesJSON: string(allowedRepositories),
			ActionsJSON:             string(actions), FieldRulesJSON: string(fieldRules),
			DataRulesJSON: string(dataRules), ApprovalRulesJSON: string(approvalRules),
			QueryLimitsJSON: string(queryLimits),
			Digest:          digest, CreatedBy: strings.TrimSpace(actor), CreatedAt: now, ActivatedAt: &now,
		}
		if err := tx.Create(&record).Error; err != nil {
			return err
		}
		result = Policy{Version: record.Version, Digest: digest, PolicySpec: normalized}
		return nil
	})
	if err != nil {
		return Policy{}, fmt.Errorf("activate integration policy: %w", err)
	}
	return result, nil
}

func (s *Service) ActivePolicy(ctx context.Context) (Policy, error) {
	var record db.IntegrationPolicyVersion
	if err := s.db.WithContext(ctx).
		Where("status = ?", PolicyActive).
		Order("version DESC").
		First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Policy{}, newError("forbidden", "no active integration policy is configured")
		}
		return Policy{}, fmt.Errorf("load active integration policy: %w", err)
	}
	var spec PolicySpec
	if err := json.Unmarshal([]byte(record.AllowedProjectsJSON), &spec.AllowedProjects); err != nil {
		return Policy{}, fmt.Errorf("decode policy projects: %w", err)
	}
	if err := json.Unmarshal([]byte(record.AllowedRepositoriesJSON), &spec.AllowedRepositories); err != nil {
		return Policy{}, fmt.Errorf("decode policy repositories: %w", err)
	}
	if err := json.Unmarshal([]byte(record.ActionsJSON), &spec.Actions); err != nil {
		return Policy{}, fmt.Errorf("decode policy actions: %w", err)
	}
	if err := json.Unmarshal([]byte(record.FieldRulesJSON), &spec.FieldRules); err != nil {
		return Policy{}, fmt.Errorf("decode policy fields: %w", err)
	}
	if strings.TrimSpace(record.DataRulesJSON) == "" {
		spec.DataRules = map[string]string{}
	} else if err := json.Unmarshal([]byte(record.DataRulesJSON), &spec.DataRules); err != nil {
		return Policy{}, fmt.Errorf("decode policy data rules: %w", err)
	}
	if err := json.Unmarshal([]byte(record.ApprovalRulesJSON), &spec.ApprovalRules); err != nil {
		return Policy{}, fmt.Errorf("decode policy approvals: %w", err)
	}
	if err := json.Unmarshal([]byte(record.QueryLimitsJSON), &spec.QueryLimits); err != nil {
		return Policy{}, fmt.Errorf("decode policy query limits: %w", err)
	}
	spec.QueryLimits = spec.QueryLimits.normalized()
	return Policy{Version: record.Version, Digest: record.Digest, PolicySpec: spec}, nil
}

func (s *Service) UpsertExecutionBinding(ctx context.Context, binding db.JiraExecutionBinding) error {
	binding.ProjectRef = strings.ToUpper(strings.TrimSpace(binding.ProjectRef))
	binding.ActionClass = strings.ToLower(strings.TrimSpace(binding.ActionClass))
	if binding.ProjectRef == "" || binding.ActionClass == "" ||
		strings.TrimSpace(binding.ConnectorRef) == "" || strings.TrimSpace(binding.ExecutorRef) == "" {
		return newError("invalid_request", "project, action, connector and executor are required")
	}
	if binding.Version <= 0 {
		binding.Version = 1
	}
	if binding.Status == "" {
		binding.Status = SourceActive
	}
	var existing db.JiraExecutionBinding
	err := s.db.WithContext(ctx).
		Where("project_ref = ? AND action_class = ?", binding.ProjectRef, binding.ActionClass).
		First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return s.db.WithContext(ctx).Create(&binding).Error
	}
	if err != nil {
		return err
	}
	binding.ID = existing.ID
	binding.CreatedAt = existing.CreatedAt
	return s.db.WithContext(ctx).Save(&binding).Error
}

func (s *Service) ExecutionBinding(ctx context.Context, project, action string) (db.JiraExecutionBinding, error) {
	var binding db.JiraExecutionBinding
	err := s.db.WithContext(ctx).
		Where("project_ref = ? AND action_class = ? AND status = ?",
			strings.ToUpper(strings.TrimSpace(project)),
			strings.ToLower(strings.TrimSpace(action)),
			SourceActive,
		).
		First(&binding).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return db.JiraExecutionBinding{}, newError("forbidden", "no active Jira execution binding is configured")
	}
	if err != nil {
		return db.JiraExecutionBinding{}, fmt.Errorf("load Jira execution binding: %w", err)
	}
	return binding, nil
}

func (s *Service) RecordInvocation(ctx context.Context, invocation Invocation) error {
	targets, _ := json.Marshal(invocation.TargetRefs)
	record := db.CapabilityInvocation{
		RequestID: strings.TrimSpace(invocation.RequestID),
		SourceID:  invocation.Identity.SourceID, KeyID: invocation.Identity.KeyID,
		Transport: invocation.Transport, ToolName: invocation.ToolName,
		ToolVersion: invocation.ToolVersion, PolicyVersion: invocation.PolicyVersion,
		TargetRefsJSON: string(targets), DurationMS: invocation.Duration.Milliseconds(),
		Outcome: invocation.Outcome, ErrorCode: invocation.ErrorCode, CreatedAt: s.now().UTC(),
	}
	if record.RequestID == "" {
		return newError("invalid_request", "request id is required")
	}
	return s.db.WithContext(ctx).Create(&record).Error
}

func RandomID(prefix string) (string, error) {
	value, err := randomToken(rand.Reader, 18)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(prefix) == "" {
		return value, nil
	}
	return strings.TrimSpace(prefix) + "_" + value, nil
}

func randomToken(source io.Reader, size int) (string, error) {
	value := make([]byte, size)
	if _, err := io.ReadFull(source, value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func credentialVerifier(secret string) string {
	value := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(value[:])
}

func parseCredential(secret string) (string, bool) {
	secret = strings.TrimSpace(secret)
	parts := strings.SplitN(secret, ".", 2)
	if len(parts) != 2 || !strings.HasPrefix(parts[0], "wa_live_") || len(parts[1]) < 32 {
		return "", false
	}
	keyID := strings.TrimPrefix(parts[0], "wa_live_")
	return keyID, keyID != ""
}

func normalizePolicy(spec PolicySpec) (PolicySpec, error) {
	spec.AllowedProjects = normalizeList(spec.AllowedProjects, true)
	spec.AllowedRepositories = normalizeList(spec.AllowedRepositories, false)
	spec.Actions = normalizeList(spec.Actions, false)
	if len(spec.AllowedProjects) == 0 && len(spec.AllowedRepositories) == 0 {
		return PolicySpec{}, newError("invalid_request", "policy must allow at least one project or repository")
	}
	if spec.FieldRules == nil {
		spec.FieldRules = map[string][]string{}
	}
	normalizedFields := make(map[string][]string, len(spec.FieldRules))
	for capability, fields := range spec.FieldRules {
		key := strings.ToLower(strings.TrimSpace(capability))
		if key == "" {
			continue
		}
		normalizedFields[key] = normalizeList(fields, false)
	}
	spec.FieldRules = normalizedFields
	if spec.DataRules == nil {
		spec.DataRules = map[string]string{}
	}
	normalizedDataRules := make(map[string]string, len(spec.DataRules))
	for name, value := range spec.DataRules {
		if key := strings.ToLower(strings.TrimSpace(name)); key != "" {
			normalizedDataRules[key] = strings.ToLower(strings.TrimSpace(value))
		}
	}
	spec.DataRules = normalizedDataRules
	if spec.ApprovalRules == nil {
		spec.ApprovalRules = map[string]string{}
	}
	normalizedApprovals := make(map[string]string, len(spec.ApprovalRules))
	for action, rule := range spec.ApprovalRules {
		if key := strings.ToLower(strings.TrimSpace(action)); key != "" {
			normalizedApprovals[key] = strings.ToLower(strings.TrimSpace(rule))
		}
	}
	spec.ApprovalRules = normalizedApprovals
	spec.QueryLimits = spec.QueryLimits.normalized()
	return spec, nil
}

func normalizeList(values []string, upper bool) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if upper {
			value = strings.ToUpper(value)
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, value)
	}
	sort.Slice(result, func(i, j int) bool {
		return strings.ToLower(result[i]) < strings.ToLower(result[j])
	})
	return result
}

func policyDigest(spec PolicySpec) string {
	value, _ := json.Marshal(spec)
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}

type Quota struct {
	RequestsPerMinute int
	MaxConcurrent     int
}

type Limiter struct {
	db            *gorm.DB
	now           func() time.Time
	profiles      map[string]Quota
	leaseDuration time.Duration
}

func NewLimiter(database *gorm.DB, profiles map[string]Quota) *Limiter {
	if len(profiles) == 0 {
		profiles = map[string]Quota{
			DefaultQuotaProfile: {RequestsPerMinute: 120, MaxConcurrent: 4},
			"read-heavy":        {RequestsPerMinute: 600, MaxConcurrent: 12},
		}
	}
	return &Limiter{
		db: database, now: time.Now, profiles: profiles, leaseDuration: 90 * time.Second,
	}
}

func (l *Limiter) Acquire(ctx context.Context, identity Identity) (func(), error) {
	if l == nil || l.db == nil {
		return nil, newError("unavailable", "integration quota storage is unavailable")
	}
	quota, ok := l.profiles[identity.QuotaProfile]
	if !ok {
		quota = l.profiles[DefaultQuotaProfile]
	}
	if quota.RequestsPerMinute <= 0 || quota.MaxConcurrent <= 0 {
		return nil, newError("rate_limited", "integration source quota exceeded")
	}
	now := l.now().UTC()
	windowStart := now.Truncate(time.Minute)
	leaseID, err := RandomID("quota")
	if err != nil {
		return nil, err
	}
	delay := 5 * time.Millisecond
	for attempt := 0; attempt < 7; attempt++ {
		err = l.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			var source db.IntegrationSource
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				First(&source, "id = ?", identity.SourceID).Error; err != nil {
				return newError("forbidden", "integration source is unavailable")
			}
			if source.Status != SourceActive {
				return newError("forbidden", "integration source is disabled")
			}
			if err := tx.Where("source_id = ? AND expires_at <= ?", identity.SourceID, now).
				Delete(&db.IntegrationQuotaLease{}).Error; err != nil {
				return err
			}
			var activeLeases int64
			if err := tx.Model(&db.IntegrationQuotaLease{}).
				Where("source_id = ? AND expires_at > ?", identity.SourceID, now).
				Count(&activeLeases).Error; err != nil {
				return err
			}
			var window db.IntegrationQuotaWindow
			windowErr := tx.Where(
				"source_id = ? AND window_start = ?", identity.SourceID, windowStart,
			).First(&window).Error
			if errors.Is(windowErr, gorm.ErrRecordNotFound) {
				window = db.IntegrationQuotaWindow{
					SourceID: identity.SourceID, WindowStart: windowStart,
					RequestCount: 0, CreatedAt: now, UpdatedAt: now,
				}
				if err := tx.Create(&window).Error; err != nil {
					return err
				}
			} else if windowErr != nil {
				return windowErr
			}
			if window.RequestCount >= quota.RequestsPerMinute || activeLeases >= int64(quota.MaxConcurrent) {
				return newError("rate_limited", "integration source quota exceeded")
			}
			if err := tx.Model(&db.IntegrationQuotaWindow{}).
				Where("source_id = ? AND window_start = ?", identity.SourceID, windowStart).
				Updates(map[string]any{
					"request_count": gorm.Expr("request_count + 1"),
					"updated_at":    now,
				}).Error; err != nil {
				return err
			}
			lease := db.IntegrationQuotaLease{
				ID: leaseID, SourceID: identity.SourceID,
				ExpiresAt: now.Add(l.leaseDuration), CreatedAt: now,
			}
			if err := tx.Create(&lease).Error; err != nil {
				return err
			}
			return tx.Where(
				"source_id = ? AND window_start < ?", identity.SourceID, windowStart.Add(-time.Minute),
			).Delete(&db.IntegrationQuotaWindow{}).Error
		})
		if err == nil {
			break
		}
		if ErrorCode(err) != "internal_error" || !retryableQuotaStorageError(err) || attempt == 6 {
			return nil, err
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
		if delay < 80*time.Millisecond {
			delay *= 2
		}
	}
	var once sync.Once
	stopRenewal := make(chan struct{})
	go l.renewLease(leaseID, stopRenewal)
	return func() {
		once.Do(func() {
			close(stopRenewal)
			releaseCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = l.db.WithContext(releaseCtx).Delete(&db.IntegrationQuotaLease{}, "id = ?", leaseID).Error
		})
	}, nil
}

func (l *Limiter) renewLease(leaseID string, stop <-chan struct{}) {
	interval := l.leaseDuration / 3
	if interval <= 0 {
		return
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			result := l.db.WithContext(ctx).Model(&db.IntegrationQuotaLease{}).
				Where("id = ?", leaseID).
				Update("expires_at", l.now().UTC().Add(l.leaseDuration))
			cancel()
			if result.Error != nil || result.RowsAffected != 1 {
				return
			}
		}
	}
}

func retryableQuotaStorageError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	for _, fragment := range []string{
		"database is locked",
		"database table is locked",
		"serialization failure",
		"could not serialize",
		"deadlock detected",
		"duplicate key",
		"unique constraint",
	} {
		if strings.Contains(message, fragment) {
			return true
		}
	}
	return false
}
