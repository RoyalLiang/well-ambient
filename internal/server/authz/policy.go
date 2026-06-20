package authz

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"

	userdb "well-ambient/internal/db/user"

	"gorm.io/gorm"
)

const (
	EffectAllow = "allow"
	EffectDeny  = "deny"

	SubjectAny   = "any"
	SubjectUser  = "user"
	SubjectGroup = "group"

	MissingSubject    = "subject:not_registered"
	MissingPermission = "permission:not_granted"
)

// Subject describes the actor asking for a permission decision.
type Subject struct {
	UserID   uint
	Username string
	Groups   []string
}

// Resource describes the protected object and request scope.
type Resource struct {
	Type        string
	ID          string
	Scope       string
	ScopeID     string
	RequestPath string
	IPAddress   string
}

// Decision is the explainable result of an authorization check.
type Decision struct {
	Allowed           bool
	Reason            string
	MatchedPolicy     *userdb.AuthorizationPolicy
	MissingPermission string
	Scope             string
	RiskLevel         string
}

// Evaluator resolves policy-based authorization while retaining legacy RBAC compatibility.
type Evaluator struct {
	db *gorm.DB
}

// NewEvaluator returns a policy evaluator bound to a database handle.
func NewEvaluator(db *gorm.DB) Evaluator {
	return Evaluator{db: db}
}

// Authorize evaluates subject + action + resource with deny override, policy allow, super_admin,
// and legacy group permission compatibility.
func (e Evaluator) Authorize(ctx context.Context, subject Subject, action string, resource Resource) Decision {
	if ctx == nil {
		ctx = context.Background()
	}
	resource = normalizeResource(resource, action)
	decision := Decision{
		Allowed:           false,
		Reason:            "permission not granted",
		MissingPermission: action,
		Scope:             decisionScope(resource),
		RiskLevel:         riskLevelForAction(action),
	}
	if e.db == nil {
		decision.Reason = "authorization database unavailable"
		decision.MissingPermission = MissingPermission
		return decision
	}

	resolved, err := e.resolveSubject(ctx, subject)
	if err != nil {
		decision.Reason = "User not registered"
		decision.MissingPermission = MissingSubject
		e.recordDecisionAudit(ctx, resolved, action, resource, decision)
		return decision
	}

	policies, err := e.matchingPolicies(ctx, resolved, action, resource)
	if err != nil {
		decision.Reason = fmt.Sprintf("authorization policy lookup failed: %v", err)
		decision.MissingPermission = MissingPermission
		e.recordDecisionAudit(ctx, resolved, action, resource, decision)
		return decision
	}

	if deny := firstPolicyWithEffect(policies, EffectDeny); deny != nil {
		decision.Allowed = false
		decision.Reason = policyReason(*deny, "explicit deny policy matched")
		decision.MatchedPolicy = deny
		decision.MissingPermission = ""
		e.recordDecisionAudit(ctx, resolved, action, resource, decision)
		return decision
	}

	if allow := firstPolicyWithEffect(policies, EffectAllow); allow != nil {
		decision.Allowed = true
		decision.Reason = policyReason(*allow, "allow policy matched")
		decision.MatchedPolicy = allow
		decision.MissingPermission = ""
		e.recordDecisionAudit(ctx, resolved, action, resource, decision)
		return decision
	}

	if resolved.hasGlobalGroup("super_admin") {
		decision.Allowed = true
		decision.Reason = "global super_admin break-glass access"
		decision.MissingPermission = ""
		e.recordDecisionAudit(ctx, resolved, action, resource, decision)
		return decision
	}

	if allowed, groupName, err := e.legacyGroupPermissionAllowed(ctx, resolved.User.ID, action, resource); err != nil {
		decision.Reason = fmt.Sprintf("legacy permission lookup failed: %v", err)
		decision.MissingPermission = MissingPermission
		e.recordDecisionAudit(ctx, resolved, action, resource, decision)
		return decision
	} else if allowed {
		decision.Allowed = true
		decision.Reason = fmt.Sprintf("legacy group permission matched: %s", groupName)
		decision.MissingPermission = ""
		e.recordDecisionAudit(ctx, resolved, action, resource, decision)
		return decision
	}

	decision.MissingPermission = action
	e.recordDecisionAudit(ctx, resolved, action, resource, decision)
	return decision
}

type resolvedSubject struct {
	User        userdb.User
	Memberships []membership
	Groups      []string
}

type membership struct {
	GroupName string
	Scope     string
	ScopeID   string
}

func (s resolvedSubject) hasGlobalGroup(name string) bool {
	for _, membership := range s.Memberships {
		if membership.GroupName == name && normalizeScope(membership.Scope) == "global" {
			return true
		}
	}
	return false
}

func (e Evaluator) resolveSubject(ctx context.Context, subject Subject) (resolvedSubject, error) {
	var user userdb.User
	query := e.db.WithContext(ctx)
	if subject.UserID != 0 {
		query = query.Where("id = ?", subject.UserID)
	} else {
		query = query.Where("username = ?", strings.TrimSpace(subject.Username))
	}
	if err := query.First(&user).Error; err != nil {
		return resolvedSubject{}, err
	}

	var memberships []membership
	if err := e.db.WithContext(ctx).
		Table("user_group_memberships ugm").
		Select("user_groups.name as group_name, ugm.scope, ugm.scope_id").
		Joins("join user_groups on user_groups.id = ugm.user_group_id").
		Where("ugm.user_id = ?", user.ID).
		Scan(&memberships).Error; err != nil {
		return resolvedSubject{}, err
	}

	groups := make([]string, 0, len(memberships))
	seen := make(map[string]struct{}, len(memberships))
	for _, membership := range memberships {
		if _, ok := seen[membership.GroupName]; ok {
			continue
		}
		seen[membership.GroupName] = struct{}{}
		groups = append(groups, membership.GroupName)
	}

	return resolvedSubject{User: user, Memberships: memberships, Groups: groups}, nil
}

func (e Evaluator) matchingPolicies(ctx context.Context, subject resolvedSubject, action string, resource Resource) ([]userdb.AuthorizationPolicy, error) {
	var policies []userdb.AuthorizationPolicy
	if err := e.db.WithContext(ctx).
		Where("enabled = ?", true).
		Where("(action = ? OR action = '*' OR action = '')", action).
		Order("priority desc").
		Order("id asc").
		Find(&policies).Error; err != nil {
		return nil, err
	}

	matches := make([]userdb.AuthorizationPolicy, 0, len(policies))
	for _, policy := range policies {
		if policyMatchesSubject(policy, subject) &&
			policyMatchesResource(policy, resource) &&
			conditionMatches(policy.ConditionJSON) {
			matches = append(matches, policy)
		}
	}
	return matches, nil
}

func policyMatchesSubject(policy userdb.AuthorizationPolicy, subject resolvedSubject) bool {
	subjectType := strings.ToLower(strings.TrimSpace(policy.SubjectType))
	subjectID := strings.TrimSpace(policy.SubjectID)
	if subjectType == "" || subjectType == SubjectAny || subjectType == "*" {
		return subjectID == "" || subjectID == "*" || subjectID == subject.User.Username || subjectID == strconv.FormatUint(uint64(subject.User.ID), 10)
	}

	switch subjectType {
	case SubjectUser:
		return subjectID == "" || subjectID == "*" || subjectID == subject.User.Username || subjectID == strconv.FormatUint(uint64(subject.User.ID), 10)
	case SubjectGroup:
		if subjectID == "" || subjectID == "*" {
			return len(subject.Groups) > 0
		}
		for _, group := range subject.Groups {
			if group == subjectID {
				return true
			}
		}
	}
	return false
}

func policyMatchesResource(policy userdb.AuthorizationPolicy, resource Resource) bool {
	policyResourceType := normalizeWildcard(policy.ResourceType)
	if policyResourceType != "" && policyResourceType != resource.Type {
		return false
	}

	policyResourceID := strings.TrimSpace(policy.ResourceID)
	if policyResourceID != "" && policyResourceID != "*" && policyResourceID != resource.ID {
		return false
	}

	policyScope := normalizeWildcard(policy.Scope)
	if policyScope == "" {
		return true
	}
	if policyScope == "global" {
		return true
	}
	if policyScope != resource.Scope {
		return false
	}

	policyScopeID := strings.TrimSpace(policy.ScopeID)
	return policyScopeID == "" || policyScopeID == "*" || policyScopeID == resource.ScopeID
}

func conditionMatches(conditionJSON string) bool {
	conditionJSON = strings.TrimSpace(conditionJSON)
	return conditionJSON == "" || conditionJSON == "{}"
}

type legacyPermissionRow struct {
	GroupName string
	Scope     string
	ScopeID   string
}

func (e Evaluator) legacyGroupPermissionAllowed(ctx context.Context, userID uint, action string, resource Resource) (bool, string, error) {
	var rows []legacyPermissionRow
	err := e.db.WithContext(ctx).
		Table("permissions").
		Select("user_groups.name as group_name, ugm.scope, ugm.scope_id").
		Joins("join group_permissions gp on gp.permission_id = permissions.id").
		Joins("join user_group_memberships ugm on ugm.user_group_id = gp.user_group_id").
		Joins("join user_groups on user_groups.id = ugm.user_group_id").
		Where("ugm.user_id = ? AND permissions.code = ?", userID, action).
		Scan(&rows).Error
	if err != nil {
		return false, "", err
	}

	for _, row := range rows {
		if membershipCoversResource(row.Scope, row.ScopeID, resource) {
			return true, row.GroupName, nil
		}
	}
	return false, "", nil
}

func membershipCoversResource(membershipScope, membershipScopeID string, resource Resource) bool {
	membershipScope = normalizeScope(membershipScope)
	membershipScopeID = strings.TrimSpace(membershipScopeID)
	if resource.ScopeID == "" {
		return membershipScope == "global"
	}
	return membershipScope == "global" || (membershipScope == resource.Scope && membershipScopeID == resource.ScopeID)
}

func firstPolicyWithEffect(policies []userdb.AuthorizationPolicy, effect string) *userdb.AuthorizationPolicy {
	for i := range policies {
		if strings.EqualFold(strings.TrimSpace(policies[i].Effect), effect) {
			return &policies[i]
		}
	}
	return nil
}

func policyReason(policy userdb.AuthorizationPolicy, fallback string) string {
	if reason := strings.TrimSpace(policy.Reason); reason != "" {
		return reason
	}
	return fallback
}

func normalizeResource(resource Resource, action string) Resource {
	resource.Type = normalizeResourceType(resource.Type, action)
	resource.Scope = normalizeScope(resource.Scope)
	resource.ID = strings.TrimSpace(resource.ID)
	resource.ScopeID = strings.TrimSpace(resource.ScopeID)
	if resource.ScopeID != "" && resource.Scope == "global" {
		resource.Scope = "repo"
	}
	if resource.ID == "" && resource.Scope == "repo" {
		resource.ID = resource.ScopeID
	}
	return resource
}

func normalizeResourceType(resourceType, action string) string {
	resourceType = strings.ToLower(strings.TrimSpace(resourceType))
	if resourceType != "" && resourceType != "any" && resourceType != "*" {
		return resourceType
	}

	switch strings.SplitN(action, ":", 2)[0] {
	case "config":
		return "config"
	case "users":
		return "user"
	case "demands":
		return "demand"
	case "ai_context":
		return "config"
	case "policies", "authorization_audit":
		return "global"
	default:
		return "global"
	}
}

func normalizeScope(scope string) string {
	scope = strings.ToLower(strings.TrimSpace(scope))
	if scope == "" || scope == "any" || scope == "*" {
		return "global"
	}
	return scope
}

func normalizeWildcard(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "any" || value == "*" {
		return ""
	}
	return value
}

func decisionScope(resource Resource) string {
	if resource.ScopeID == "" {
		return resource.Scope
	}
	return resource.Scope + ":" + resource.ScopeID
}

func riskLevelForAction(action string) string {
	switch action {
	case "config:write", "users:write", "users:transfer_super_admin", "ai_context:write", "policies:write":
		return "high"
	}
	if strings.HasSuffix(action, ":write") {
		return "medium"
	}
	return "low"
}

func (e Evaluator) recordDecisionAudit(ctx context.Context, subject resolvedSubject, action string, resource Resource, decision Decision) {
	if e.db == nil || (decision.Allowed && decision.RiskLevel != "high") {
		return
	}

	groupsJSON, err := json.Marshal(subject.Groups)
	if err != nil {
		groupsJSON = []byte("[]")
	}

	var matchedPolicyID *uint
	if decision.MatchedPolicy != nil {
		id := decision.MatchedPolicy.ID
		matchedPolicyID = &id
	}

	entry := userdb.AuthorizationAuditLog{
		SubjectUserID:     subject.User.ID,
		SubjectUsername:   subject.User.Username,
		SubjectGroupsJSON: string(groupsJSON),
		Action:            action,
		ResourceType:      resource.Type,
		ResourceID:        resource.ID,
		Scope:             resource.Scope,
		ScopeID:           resource.ScopeID,
		Allowed:           decision.Allowed,
		Reason:            decision.Reason,
		MissingPermission: decision.MissingPermission,
		MatchedPolicyID:   matchedPolicyID,
		RiskLevel:         decision.RiskLevel,
		RequestPath:       resource.RequestPath,
		IpAddress:         resource.IPAddress,
	}
	if err := e.db.WithContext(ctx).Create(&entry).Error; err != nil && !errors.Is(err, gorm.ErrInvalidDB) {
		log.Printf("authorization audit log write failed: %v", err)
	}
}
