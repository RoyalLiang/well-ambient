package user

import (
	"time"
)

// User represents a system user mapping to WellOS identity
type User struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	Username   string    `gorm:"uniqueIndex:idx_user_username;type:varchar(128)" json:"username"` // Unique username e.g. eddie
	Email      string    `gorm:"uniqueIndex:idx_user_email;type:varchar(128)" json:"email"`       // Unique email e.g. eddie@westwell-lab.com
	Name       string    `json:"name"`                                                            // Full name
	Avatar     string    `json:"avatar"`                                                          // Avatar URL
	Department string    `json:"department"`                                                      // Department name
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
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
	ID          uint      `gorm:"primaryKey" json:"id"`
	UserGroupID uint      `gorm:"uniqueIndex:idx_group_perm" json:"user_group_id"`
	PermissionID uint      `gorm:"uniqueIndex:idx_group_perm" json:"permission_id"`
	CreatedAt   time.Time `json:"created_at"`
}

// AuditLog tracks sensitive configuration and RBAC changes
type AuditLog struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	ActorID       uint      `json:"actor_id"`
	ActorName     string    `json:"actor_name"`
	ActorUsername string    `json:"actor_username"`
	Action        string    `json:"action"`          // config_update, config_test, role_assign, admin_transfer, group_create, group_update
	TargetID      string    `json:"target_id"`       // Target identifier (e.g. username, group ID)
	TargetType    string    `json:"target_type"`     // Target type (e.g. user, group, config)
	Detail        string    `gorm:"type:text" json:"detail"`
	IpAddress     string    `json:"ip_address"`
	CreatedAt     time.Time `json:"created_at"`
}
