package user

import (
	"log"
	"time"

	"gorm.io/gorm"
)

// InitializeSeeds seeds the initial database with permissions and default groups
func InitializeSeeds(db *gorm.DB) error {
	// 1. Seed Permissions
	defaultPerms := []Permission{
		{Code: "config:read", Name: "查看系统集成配置", Description: "有权查看 GitLab、飞书、Jira 以及 AI 大模型等集成密钥及连接状态"},
		{Code: "config:write", Name: "修改及测试系统配置", Description: "有权修改并测试 GitLab、飞书、Jira 以及 AI 大模型等核心配置参数"},
		{Code: "users:read", Name: "查看成员及权限列表", Description: "有权查看所有注册用户、用户组以及审计日志"},
		{Code: "users:write", Name: "管理成员组与权限分配", Description: "有权创建自定义组、修改用户组拥有的权限、以及将用户分配至特定组（包含项目级空间隔离 Scope 绑定）"},
		{Code: "users:transfer_super_admin", Name: "转让超级管理员角色", Description: "仅超级管理员可用，转让超级管理员权限给其他用户，自身降级为系统管理员"},
		{Code: "kpi:read", Name: "查看团队 KPI 看板", Description: "有权查看团队成员的 KPI 绩效统计、完成任务及 Bug 指标"},
		{Code: "demands:write", Name: "创建与指派需求", Description: "有权在需求看板中创建新需求并指派负责人"},
		{Code: "dashboard:read", Name: "查看协同看板页面", Description: "有权查看主界面协同看板、AI 需求解构日志与全部任务看板"},
		{Code: "demands:read", Name: "查看需求看板页面", Description: "有权查看需求看板泳道及其排期卡片"},
		{Code: "decision:read", Name: "查看决策大屏页面", Description: "有权查看红区卡点诊断盘与决策会议大屏"},
	}

	log.Println("Seeding default permissions (incremental)...")
	for _, p := range defaultPerms {
		var count int64
		db.Model(&Permission{}).Where("code = ?", p.Code).Count(&count)
		if count == 0 {
			if err := db.Create(&p).Error; err != nil {
				return err
			}
		}
	}

	// 2. Seed User Groups
	defaultGroups := []UserGroup{
		{Name: "super_admin", DisplayName: "超级管理员组", Description: "拥有系统所有管理权限，包含权限管理和超管转让操作，为系统的最高权限"},
		{Name: "admin", DisplayName: "系统管理员组", Description: "拥有修改集成配置、查看成员及审计日志的权限，无法管理权限以及进行超管转让"},
		{Name: "member", DisplayName: "普通成员组", Description: "普通开发成员，无系统集成配置的查看与修改权限，只能查看看板和接收通知"},
	}

	log.Println("Seeding default user groups (incremental)...")
	for _, g := range defaultGroups {
		var count int64
		db.Model(&UserGroup{}).Where("name = ?", g.Name).Count(&count)
		if count == 0 {
			if err := db.Create(&g).Error; err != nil {
				return err
			}
		}
	}

	// 3. Seed Group Permissions mapping
	log.Println("Seeding group-permission mappings (incremental)...")

	// Load groups and permissions from database
	var groups []UserGroup
	db.Find(&groups)
	groupMap := make(map[string]uint)
	for _, g := range groups {
		groupMap[g.Name] = g.ID
	}

	var perms []Permission
	db.Find(&perms)
	permMap := make(map[string]uint)
	for _, p := range perms {
		permMap[p.Code] = p.ID
	}

	// Super Admin gets all permissions
	superAdminID := groupMap["super_admin"]
	for _, pID := range permMap {
		var count int64
		db.Model(&GroupPermission{}).Where("user_group_id = ? AND permission_id = ?", superAdminID, pID).Count(&count)
		if count == 0 {
			db.Create(&GroupPermission{
				UserGroupID:  superAdminID,
				PermissionID: pID,
			})
		}
	}

	// Admin gets config:read, config:write, users:read, kpi:read
	adminID := groupMap["admin"]
	adminPermCodes := []string{"config:read", "config:write", "users:read", "kpi:read", "dashboard:read", "demands:read", "decision:read"}
	for _, code := range adminPermCodes {
		if pID, ok := permMap[code]; ok {
			var count int64
			db.Model(&GroupPermission{}).Where("user_group_id = ? AND permission_id = ?", adminID, pID).Count(&count)
			if count == 0 {
				db.Create(&GroupPermission{
					UserGroupID:  adminID,
					PermissionID: pID,
				})
			}
		}
	}

	// Member gets dashboard:read, demands:read, decision:read
	memberID := groupMap["member"]
	memberPermCodes := []string{"dashboard:read", "demands:read", "decision:read"}
	for _, code := range memberPermCodes {
		if pID, ok := permMap[code]; ok {
			var count int64
			db.Model(&GroupPermission{}).Where("user_group_id = ? AND permission_id = ?", memberID, pID).Count(&count)
			if count == 0 {
				db.Create(&GroupPermission{
					UserGroupID:  memberID,
					PermissionID: pID,
				})
			}
		}
	}

	return nil
}

// RecordAuditLog registers a security/admin action to the database audit_logs table
func RecordAuditLog(db *gorm.DB, actorUsername string, action, targetType, targetID, detail, ip string) error {
	var actorID uint = 0
	actorName := actorUsername

	var u User
	if err := db.Where("username = ?", actorUsername).First(&u).Error; err == nil {
		actorID = u.ID
		actorName = u.Name
	}

	logEntry := AuditLog{
		ActorID:       actorID,
		ActorName:     actorName,
		ActorUsername: actorUsername,
		Action:        action,
		TargetID:      targetID,
		TargetType:    targetType,
		Detail:        detail,
		IpAddress:     ip,
		CreatedAt:     time.Now(),
	}

	return db.Create(&logEntry).Error
}
