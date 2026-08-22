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
		{Code: "delivery:read", Name: "查看交付计划与发布版本", Description: "有权查看交付项、项目版本、发布范围和同步状态"},
		{Code: "delivery:plan", Name: "维护交付计划", Description: "有权通过统一规划入口调整项目、版本、负责人、截止日和规划状态"},
		{Code: "release:manage", Name: "管理发布版本", Description: "有权同步 Jira 版本目录以及创建或维护本地发布版本"},
		{Code: "decision:read", Name: "查看决策大屏页面", Description: "有权查看红区卡点诊断盘与决策会议大屏"},
		{Code: "data_asset:read", Name: "查看治理数据资产", Description: "有权读取跨来源资产事件、追溯载荷以及不可变报告和分析快照"},
		{Code: "ai_context:read", Name: "查看 AI 上下文注册表", Description: "有权查看用于 AI 需求解构的架构、流程、功能边界与估算规则上下文"},
		{Code: "ai_context:write", Name: "管理 AI 上下文注册表", Description: "有权新增、修改、停用 AI 上下文事实、文档与上下文包配置"},
		{Code: "ai_context:preview", Name: "预览 AI 上下文包", Description: "有权按需求范围预览 AI 解构将使用的上下文包内容"},
		{Code: "policies:read", Name: "查看授权策略", Description: "有权查看策略化授权规则、作用范围与命中原因"},
		{Code: "policies:write", Name: "管理授权策略", Description: "有权新增、修改、启停 allow/deny 授权策略"},
		{Code: "authorization_audit:read", Name: "查看授权决策审计", Description: "有权查看拒绝或高风险授权决策的审计日志"},
		{Code: "demand_spec:read", Name: "查看可执行需求规格", Description: "有权查看需求的版本化 AI 解构、人工修订和冻结结果"},
		{Code: "demand_spec:write", Name: "编辑可执行需求规格", Description: "有权创建和修订需求规格草案"},
		{Code: "demand_spec:freeze", Name: "冻结可执行需求规格", Description: "有权将人工审核后的需求规格冻结为自动执行依据"},
		{Code: "review_contract:manage", Name: "管理审核契约", Description: "有权维护需求的审核角色、候选人、验收人与审批规则"},
		{Code: "review_contract:resolve", Name: "解析最终审核人", Description: "有权根据真实变更路径解析并确认 MR 审核人"},
		{Code: "execution:preflight", Name: "执行自治交付预检", Description: "有权检查需求冻结、审核契约、仓库和变更集是否满足执行条件"},
		{Code: "execution:start", Name: "启动自治交付", Description: "有权让系统创建受控分支、提交和 Draft MR"},
		{Code: "execution:cancel", Name: "取消自治交付", Description: "有权停止尚未交付的执行运行并保留审计记录"},
		{Code: "execution:accept", Name: "验收自治交付", Description: "有权以审核契约中的业务验收人身份确认或拒绝交付结果"},
		{Code: "corpus_candidate:read", Name: "查看语料候选", Description: "有权查看交付复盘生成的知识和案例候选"},
		{Code: "corpus_candidate:review", Name: "审核语料候选", Description: "有权接受或拒绝语料候选并将接受项纳入版本化上下文事实"},
		{Code: "solution:read", Name: "查看需求方案", Description: "有权查看需求的方案草案、候选版本、已发布版本及其来源状态"},
		{Code: "solution:write", Name: "编辑与润色需求方案", Description: "有权编辑需求方案草案、请求 Agent 润色并人工应用候选版本"},
		{Code: "solution:publish", Name: "发布需求方案", Description: "有权把人工确认的需求方案草案发布为需求当前方案"},
		{Code: "solution_prompt:manage", Name: "管理方案润色提示词", Description: "仅全局超级管理员可新增、测试、启用和回滚方案润色提示词版本"},
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

	// Admin gets operational configuration, read-only security diagnostics, and AI context management.
	adminID := groupMap["admin"]
	adminPermCodes := []string{
		"config:read",
		"config:write",
		"users:read",
		"kpi:read",
		"dashboard:read",
		"demands:read",
		"delivery:read",
		"delivery:plan",
		"release:manage",
		"decision:read",
		"data_asset:read",
		"ai_context:read",
		"ai_context:write",
		"ai_context:preview",
		"policies:read",
		"authorization_audit:read",
		"demand_spec:read",
		"demand_spec:write",
		"demand_spec:freeze",
		"review_contract:manage",
		"review_contract:resolve",
		"execution:preflight",
		"execution:start",
		"execution:cancel",
		"execution:accept",
		"corpus_candidate:read",
		"corpus_candidate:review",
		"solution:read",
		"solution:write",
		"solution:publish",
	}
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
	memberPermCodes := []string{"dashboard:read", "demands:read", "delivery:read", "decision:read", "demand_spec:read", "solution:read"}
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

	// Compatibility bridge during the delivery-domain cutover. Custom groups
	// that already carry demands permissions keep equivalent access without
	// widening any project scope.
	compatibilityMappings := map[string]string{
		"demands:read":  "delivery:read",
		"demands:write": "delivery:plan",
	}
	for legacyCode, deliveryCode := range compatibilityMappings {
		legacyID, legacyOK := permMap[legacyCode]
		deliveryID, deliveryOK := permMap[deliveryCode]
		if !legacyOK || !deliveryOK {
			continue
		}
		var legacyGrants []GroupPermission
		if err := db.Where("permission_id = ?", legacyID).Find(&legacyGrants).Error; err != nil {
			return err
		}
		for _, grant := range legacyGrants {
			var count int64
			db.Model(&GroupPermission{}).
				Where("user_group_id = ? AND permission_id = ?", grant.UserGroupID, deliveryID).
				Count(&count)
			if count == 0 {
				if err := db.Create(&GroupPermission{
					UserGroupID:  grant.UserGroupID,
					PermissionID: deliveryID,
				}).Error; err != nil {
					return err
				}
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
