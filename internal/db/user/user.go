package user

import (
	"time"
)

// User represents a system user mapping to WellOS identity
type User struct {
	ID         uint   `gorm:"primaryKey" json:"id"`
	Username   string `gorm:"uniqueIndex:idx_user_username;type:varchar(128)" json:"username"` // Unique username e.g. eddie
	Email      string `gorm:"uniqueIndex:idx_user_email;type:varchar(128)" json:"email"`       // Unique email e.g. eddie@westwell-lab.com
	Name       string `json:"name"`                                                            // Full name
	Avatar     string `json:"avatar"`                                                          // Avatar URL
	Department string `json:"department"`                                                      // Department name
	// LocalPasswordHash is only used to authenticate known users during planned WellOS outages.
	LocalPasswordHash      string     `gorm:"type:text" json:"-"`
	LocalPasswordUpdatedAt *time.Time `json:"-"`
	CreatedAt              time.Time  `json:"created_at"`
	UpdatedAt              time.Time  `json:"updated_at"`
}

// UserGroup represents a role or user group
type UserGroup struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"uniqueIndex:idx_group_name;type:varchar(64)" json:"name"` // e.g. super_admin, admin, member
	DisplayName string    `json:"display_name"`                                            // e.g. 超级管理员, 系统管理员, 普通成员
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// UserGroupMembership connects Users with UserGroups, supporting scope-based isolation
type UserGroupMembership struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	UserID      uint      `gorm:"uniqueIndex:idx_user_group_scope" json:"user_id"`
	UserGroupID uint      `gorm:"uniqueIndex:idx_user_group_scope" json:"user_group_id"`
	Scope       string    `gorm:"uniqueIndex:idx_user_group_scope;type:varchar(32);default:'global'" json:"scope"` // global, repo
	ScopeID     string    `gorm:"uniqueIndex:idx_user_group_scope;type:varchar(128);default:''" json:"scope_id"`   // Repository name/ProjectID, empty if global
	CreatedAt   time.Time `json:"created_at"`
}

// Permission represents atomic system permissions
type Permission struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Code        string    `gorm:"uniqueIndex:idx_perm_code;type:varchar(64)" json:"code"` // e.g. config:write, users:write
	Name        string    `json:"name"`                                                   // e.g. 修改系统配置, 权限管理
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// GroupPermission maps Permissions to UserGroups
type GroupPermission struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	UserGroupID  uint      `gorm:"uniqueIndex:idx_group_perm" json:"user_group_id"`
	PermissionID uint      `gorm:"uniqueIndex:idx_group_perm" json:"permission_id"`
	CreatedAt    time.Time `json:"created_at"`
}

// AuthorizationPolicy records fine-grained allow/deny rules layered above legacy group permissions.
type AuthorizationPolicy struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	Effect        string    `gorm:"index;type:varchar(16);not null" json:"effect"` // allow, deny
	SubjectType   string    `gorm:"index;type:varchar(16);not null;default:'any'" json:"subject_type"`
	SubjectID     string    `gorm:"index;type:varchar(128);default:''" json:"subject_id"`
	Action        string    `gorm:"index;type:varchar(64);not null" json:"action"`
	ResourceType  string    `gorm:"index;type:varchar(32);not null;default:'global'" json:"resource_type"`
	ResourceID    string    `gorm:"index;type:varchar(128);default:''" json:"resource_id"`
	Scope         string    `gorm:"index;type:varchar(32);not null;default:'global'" json:"scope"`
	ScopeID       string    `gorm:"index;type:varchar(128);default:''" json:"scope_id"`
	ConditionJSON string    `gorm:"type:text" json:"condition_json"`
	Priority      int       `gorm:"index;default:0" json:"priority"`
	Enabled       bool      `gorm:"index;default:true" json:"enabled"`
	Reason        string    `gorm:"type:text" json:"reason"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// AuthorizationAuditLog stores evaluated policy decisions that need admin diagnosis.
type AuthorizationAuditLog struct {
	ID                uint      `gorm:"primaryKey" json:"id"`
	SubjectUserID     uint      `gorm:"index" json:"subject_user_id"`
	SubjectUsername   string    `gorm:"index;type:varchar(128)" json:"subject_username"`
	SubjectGroupsJSON string    `gorm:"type:text" json:"subject_groups_json"`
	Action            string    `gorm:"index;type:varchar(64)" json:"action"`
	ResourceType      string    `gorm:"index;type:varchar(32)" json:"resource_type"`
	ResourceID        string    `gorm:"index;type:varchar(128)" json:"resource_id"`
	Scope             string    `gorm:"index;type:varchar(32)" json:"scope"`
	ScopeID           string    `gorm:"index;type:varchar(128)" json:"scope_id"`
	Allowed           bool      `gorm:"index" json:"allowed"`
	Reason            string    `gorm:"type:text" json:"reason"`
	MissingPermission string    `gorm:"type:varchar(128)" json:"missing_permission"`
	MatchedPolicyID   *uint     `gorm:"index" json:"matched_policy_id"`
	RiskLevel         string    `gorm:"index;type:varchar(16)" json:"risk_level"`
	RequestPath       string    `gorm:"type:varchar(256)" json:"request_path"`
	IpAddress         string    `gorm:"type:varchar(128)" json:"ip_address"`
	CreatedAt         time.Time `gorm:"index" json:"created_at"`
}

// AuditLog tracks sensitive configuration and RBAC changes
type AuditLog struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	ActorID       uint      `json:"actor_id"`
	ActorName     string    `json:"actor_name"`
	ActorUsername string    `json:"actor_username"`
	Action        string    `json:"action"`      // config_update, config_test, role_assign, admin_transfer, group_create, group_update
	TargetID      string    `json:"target_id"`   // Target identifier (e.g. username, group ID)
	TargetType    string    `json:"target_type"` // Target type (e.g. user, group, config)
	Detail        string    `gorm:"type:text" json:"detail"`
	IpAddress     string    `json:"ip_address"`
	CreatedAt     time.Time `gorm:"index" json:"created_at"`
}
