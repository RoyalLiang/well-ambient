package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
	"well-ambient/internal/db"
	userdb "well-ambient/internal/db/user"
)

// DTO structs for clean API responses
type UserGroupMembershipDTO struct {
	GroupName        string `json:"group_name"`
	GroupDisplayName string `json:"group_display_name"`
	Scope            string `json:"scope"`
	ScopeID          string `json:"scope_id"`
}

type UserDTO struct {
	ID          uint                     `json:"id"`
	Username    string                   `json:"username"`
	Email       string                   `json:"email"`
	Name        string                   `json:"name"`
	Avatar      string                   `json:"avatar"`
	Department  string                   `json:"department"`
	Memberships []UserGroupMembershipDTO `json:"memberships"`
	CreatedAt   time.Time                `json:"created_at"`
}

type GroupDTO struct {
	ID          uint     `json:"id"`
	Name        string   `json:"name"`
	DisplayName string   `json:"display_name"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
}

type PermissionDTO struct {
	ID          uint   `json:"id"`
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// handleGetCurrentUser returns the authenticated user's profile without requiring admin permissions.
func (s *Server) handleGetCurrentUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	username := r.Header.Get("x-authenticated-user-id")
	if username == "" {
		http.Error(w, "Missing authenticated user", http.StatusUnauthorized)
		return
	}

	var u userdb.User
	if err := db.DB.Where("username = ?", username).First(&u).Error; err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	headerName := r.Header.Get("x-authenticated-user-name")
	headerAvatar := r.Header.Get("x-authenticated-user-avatar")
	headerDept := strings.TrimSpace(r.Header.Get("x-authenticated-user-department"))
	needsSave := false
	if u.Name == "" && headerName != "" {
		u.Name = headerName
		needsSave = true
	}
	if u.Avatar == "" && headerAvatar != "" {
		u.Avatar = headerAvatar
		needsSave = true
	}
	if !isMissingDepartment(headerDept) && strings.TrimSpace(u.Department) != headerDept {
		u.Department = headerDept
		needsSave = true
	}
	if needsSave {
		if err := db.DB.Save(&u).Error; err != nil {
			http.Error(w, fmt.Sprintf("Failed to refresh current user profile: %v", err), http.StatusInternalServerError)
			return
		}
	}

	var groupNames []string
	db.DB.Table("user_groups").
		Joins("join user_group_memberships on user_group_memberships.user_group_id = user_groups.id").
		Where("user_group_memberships.user_id = ?", u.ID).
		Pluck("user_groups.name", &groupNames)

	var permCodes []string
	db.DB.Table("permissions").
		Joins("join group_permissions on group_permissions.permission_id = permissions.id").
		Joins("join user_group_memberships on user_group_memberships.user_group_id = group_permissions.user_group_id").
		Where("user_group_memberships.user_id = ?", u.ID).
		Distinct("permissions.code").
		Pluck("permissions.code", &permCodes)

	role := "member"
	for _, g := range groupNames {
		if g == "super_admin" {
			role = "super_admin"
			break
		}
		if g == "admin" {
			role = "admin"
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"user": map[string]interface{}{
			"username":    u.Username,
			"email":       u.Email,
			"name":        u.Name,
			"avatar":      u.Avatar,
			"role":        role,
			"groups":      groupNames,
			"permissions": permCodes,
			"department":  u.Department,
		},
	})
}

// handleGetUsers returns all registered users with their scoped group memberships
func (s *Server) handleGetUsers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var users []userdb.User
	if err := db.DB.WithContext(r.Context()).Order("created_at asc, id asc").Limit(5000).Find(&users).Error; err != nil {
		http.Error(w, fmt.Sprintf("Query users failed: %v", err), http.StatusInternalServerError)
		return
	}

	type membershipRow struct {
		UserID           uint   `gorm:"column:user_id"`
		GroupName        string `gorm:"column:group_name"`
		GroupDisplayName string `gorm:"column:group_display_name"`
		Scope            string `gorm:"column:scope"`
		ScopeID          string `gorm:"column:scope_id"`
	}
	membershipsByUser := make(map[uint][]UserGroupMembershipDTO, len(users))
	if len(users) > 0 {
		userIDs := make([]uint, 0, len(users))
		for _, currentUser := range users {
			userIDs = append(userIDs, currentUser.ID)
			membershipsByUser[currentUser.ID] = []UserGroupMembershipDTO{}
		}
		var rows []membershipRow
		if err := db.DB.WithContext(r.Context()).Table("user_group_memberships").
			Select("user_group_memberships.user_id, user_groups.name AS group_name, user_groups.display_name AS group_display_name, user_group_memberships.scope, user_group_memberships.scope_id").
			Joins("JOIN user_groups ON user_groups.id = user_group_memberships.user_group_id").
			Where("user_group_memberships.user_id IN ?", userIDs).
			Order("user_group_memberships.user_id, user_group_memberships.id").
			Scan(&rows).Error; err != nil {
			http.Error(w, fmt.Sprintf("Query user memberships failed: %v", err), http.StatusInternalServerError)
			return
		}
		for _, row := range rows {
			membershipsByUser[row.UserID] = append(membershipsByUser[row.UserID], UserGroupMembershipDTO{
				GroupName: row.GroupName, GroupDisplayName: row.GroupDisplayName,
				Scope: row.Scope, ScopeID: row.ScopeID,
			})
		}
	}

	var userDTOs []UserDTO
	for _, u := range users {
		userDTOs = append(userDTOs, UserDTO{
			ID:          u.ID,
			Username:    u.Username,
			Email:       u.Email,
			Name:        u.Name,
			Avatar:      u.Avatar,
			Department:  u.Department,
			Memberships: membershipsByUser[u.ID],
			CreatedAt:   u.CreatedAt,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(userDTOs)
}

type UpdateUserDepartmentRequest struct {
	Username   string `json:"username"`
	Department string `json:"department"`
}

// handleUpdateUserDepartment updates a user's department
func (s *Server) handleUpdateUserDepartment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	actorUsername := r.Header.Get("x-authenticated-user-id")

	var req UpdateUserDepartmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Username == "" {
		http.Error(w, "Username is required", http.StatusBadRequest)
		return
	}

	var u userdb.User
	if err := db.DB.Where("username = ?", req.Username).First(&u).Error; err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	oldDept := u.Department
	u.Department = req.Department
	if err := db.DB.Save(&u).Error; err != nil {
		http.Error(w, fmt.Sprintf("Failed to update department: %v", err), http.StatusInternalServerError)
		return
	}

	// Record audit log
	userdb.RecordAuditLog(db.DB, actorUsername, "user_update_dept", "user", fmt.Sprintf("%d", u.ID),
		fmt.Sprintf("修改用户 %s 的部门由 '%s' 到 '%s'", u.Username, oldDept, req.Department), r.RemoteAddr)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "Department updated successfully",
	})
}

type UpdateUserGroupsRequest struct {
	Username  string `json:"username"`
	GroupName string `json:"group_name"`
	Scope     string `json:"scope"`    // global, repo
	ScopeID   string `json:"scope_id"` // Repository name or ProjectID
	Action    string `json:"action"`   // add, remove
}

// handleUpdateUserGroups assigns or removes group membership with scope isolation support
func (s *Server) handleUpdateUserGroups(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	actorUsername := r.Header.Get("x-authenticated-user-id")

	var req UpdateUserGroupsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Username == "" || req.GroupName == "" || req.Action == "" {
		http.Error(w, "Username, GroupName, and Action are required", http.StatusBadRequest)
		return
	}

	if req.Scope != "global" && req.Scope != "repo" {
		http.Error(w, "Scope must be 'global' or 'repo'", http.StatusBadRequest)
		return
	}

	// 1. Fetch user
	var targetUser userdb.User
	if err := db.DB.Where("username = ?", req.Username).First(&targetUser).Error; err != nil {
		http.Error(w, "Target user not found", http.StatusNotFound)
		return
	}

	// 2. Fetch group
	var group userdb.UserGroup
	if err := db.DB.Where("name = ?", req.GroupName).First(&group).Error; err != nil {
		http.Error(w, "Group not found", http.StatusNotFound)
		return
	}

	// Safety: Cannot manually assign or remove super_admin group membership via this endpoint
	if group.Name == "super_admin" {
		http.Error(w, "Cannot modify 'super_admin' membership. Use transfer-admin endpoint.", http.StatusBadRequest)
		return
	}

	// 3. Perform action
	if req.Action == "add" {
		membership := userdb.UserGroupMembership{
			UserID:      targetUser.ID,
			UserGroupID: group.ID,
			Scope:       req.Scope,
			ScopeID:     req.ScopeID,
		}

		if err := db.DB.Create(&membership).Error; err != nil {
			http.Error(w, fmt.Sprintf("Failed to add group membership: %v", err), http.StatusInternalServerError)
			return
		}

		userdb.RecordAuditLog(db.DB, actorUsername, "group_member_add", "user", req.Username,
			fmt.Sprintf("为用户 [%s] 添加用户组 [%s]，作用域: %s (参数: %s)", targetUser.Name, group.DisplayName, req.Scope, req.ScopeID), r.RemoteAddr)

	} else if req.Action == "remove" {
		err := db.DB.Where("user_id = ? AND user_group_id = ? AND scope = ? AND scope_id = ?",
			targetUser.ID, group.ID, req.Scope, req.ScopeID).
			Delete(&userdb.UserGroupMembership{}).Error

		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to remove group membership: %v", err), http.StatusInternalServerError)
			return
		}

		userdb.RecordAuditLog(db.DB, actorUsername, "group_member_remove", "user", req.Username,
			fmt.Sprintf("移除了用户 [%s] 的用户组 [%s] 身份，作用域: %s (参数: %s)", targetUser.Name, group.DisplayName, req.Scope, req.ScopeID), r.RemoteAddr)
	} else {
		http.Error(w, "Invalid action. Must be 'add' or 'remove'", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"success":true,"message":"Membership updated successfully"}`))
}

type TransferAdminRequest struct {
	Username string `json:"username"`
}

// handleTransferAdmin transfers super_admin role to another user and demotes the current actor to admin
func (s *Server) handleTransferAdmin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	actorUsername := r.Header.Get("x-authenticated-user-id")

	var req TransferAdminRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Username == "" {
		http.Error(w, "Target username is required", http.StatusBadRequest)
		return
	}

	if req.Username == actorUsername {
		http.Error(w, "Cannot transfer admin rights to yourself", http.StatusBadRequest)
		return
	}

	// 1. Fetch actor user record
	var actorUser userdb.User
	if err := db.DB.Where("username = ?", actorUsername).First(&actorUser).Error; err != nil {
		http.Error(w, "Actor user not found", http.StatusForbidden)
		return
	}

	// 2. Fetch target user record
	var targetUser userdb.User
	if err := db.DB.Where("username = ?", req.Username).First(&targetUser).Error; err != nil {
		http.Error(w, "Target user not found", http.StatusNotFound)
		return
	}

	// 3. Load groups
	var superAdminGroup, adminGroup userdb.UserGroup
	db.DB.Where("name = ?", "super_admin").First(&superAdminGroup)
	db.DB.Where("name = ?", "admin").First(&adminGroup)

	// Transaction safety
	tx := db.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Verify actor is actually global super_admin
	var superCount int64
	tx.Model(&userdb.UserGroupMembership{}).
		Where("user_id = ? AND user_group_id = ? AND scope = 'global'", actorUser.ID, superAdminGroup.ID).
		Count(&superCount)

	if superCount == 0 {
		tx.Rollback()
		http.Error(w, "Only global super_admin can transfer super_admin rights", http.StatusForbidden)
		return
	}

	// Remove target's existing global memberships to avoid duplication conflicts
	tx.Where("user_id = ? AND scope = 'global'", targetUser.ID).Delete(&userdb.UserGroupMembership{})

	// Promote target to global super_admin
	targetMembership := userdb.UserGroupMembership{
		UserID:      targetUser.ID,
		UserGroupID: superAdminGroup.ID,
		Scope:       "global",
		ScopeID:     "",
	}
	if err := tx.Create(&targetMembership).Error; err != nil {
		tx.Rollback()
		http.Error(w, fmt.Sprintf("Failed to promote target user: %v", err), http.StatusInternalServerError)
		return
	}

	// Remove actor's global super_admin membership
	tx.Where("user_id = ? AND user_group_id = ? AND scope = 'global'", actorUser.ID, superAdminGroup.ID).
		Delete(&userdb.UserGroupMembership{})

	// Demote actor to global admin
	actorMembership := userdb.UserGroupMembership{
		UserID:      actorUser.ID,
		UserGroupID: adminGroup.ID,
		Scope:       "global",
		ScopeID:     "",
	}
	if err := tx.Create(&actorMembership).Error; err != nil {
		tx.Rollback()
		http.Error(w, fmt.Sprintf("Failed to demote actor user: %v", err), http.StatusInternalServerError)
		return
	}

	if err := tx.Commit().Error; err != nil {
		http.Error(w, fmt.Sprintf("Commit failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Record audit log
	userdb.RecordAuditLog(db.DB, actorUsername, "admin_transfer", "user", req.Username,
		fmt.Sprintf("超级管理员将系统控制权转让给了用户 [%s (%s)]，自身降级为管理员", targetUser.Name, targetUser.Username), r.RemoteAddr)

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"success":true,"message":"Super admin rights transferred successfully"}`))
}

// handleGetGroups returns all user groups with their assigned permission codes (Permission Matrix)
func (s *Server) handleGetGroups(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var groups []userdb.UserGroup
	if err := db.DB.Find(&groups).Error; err != nil {
		http.Error(w, fmt.Sprintf("Query groups failed: %v", err), http.StatusInternalServerError)
		return
	}

	var groupDTOs []GroupDTO
	for _, g := range groups {
		var permissions []string
		err := db.DB.Table("permissions").
			Joins("join group_permissions on group_permissions.permission_id = permissions.id").
			Where("group_permissions.user_group_id = ?", g.ID).
			Pluck("permissions.code", &permissions).Error

		if err != nil {
			log.Printf("Query permissions failed for group %s: %v", g.Name, err)
			permissions = []string{}
		}

		groupDTOs = append(groupDTOs, GroupDTO{
			ID:          g.ID,
			Name:        g.Name,
			DisplayName: g.DisplayName,
			Description: g.Description,
			Permissions: permissions,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(groupDTOs)
}

// handleListPermissions returns the live permission catalog used by the admin UI.
func (s *Server) handleListPermissions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var permissions []userdb.Permission
	if err := db.DB.Order("code asc").Find(&permissions).Error; err != nil {
		http.Error(w, fmt.Sprintf("Query permissions failed: %v", err), http.StatusInternalServerError)
		return
	}

	permissionDTOs := make([]PermissionDTO, 0, len(permissions))
	for _, p := range permissions {
		permissionDTOs = append(permissionDTOs, PermissionDTO{
			ID:          p.ID,
			Code:        p.Code,
			Name:        p.Name,
			Description: p.Description,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(permissionDTOs)
}

type CreateGroupRequest struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
}

// handleCreateGroup creates a custom user group
func (s *Server) handleCreateGroup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	actorUsername := r.Header.Get("x-authenticated-user-id")

	var req CreateGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" || req.DisplayName == "" {
		http.Error(w, "Group Name and DisplayName are required", http.StatusBadRequest)
		return
	}

	newGroup := userdb.UserGroup{
		Name:        req.Name,
		DisplayName: req.DisplayName,
		Description: req.Description,
	}

	if err := db.DB.Create(&newGroup).Error; err != nil {
		http.Error(w, fmt.Sprintf("Failed to create group: %v", err), http.StatusInternalServerError)
		return
	}

	userdb.RecordAuditLog(db.DB, actorUsername, "group_create", "group", req.Name,
		fmt.Sprintf("创建了自定义用户组 [%s]，显示名: %s", req.Name, req.DisplayName), r.RemoteAddr)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"success":true,"message":"User group created successfully"}`))
}

func isBuiltInGroupName(name string) bool {
	switch name {
	case "super_admin", "admin", "member":
		return true
	default:
		return false
	}
}

// handleDeleteGroup deletes a custom user group after membership and built-in safety checks.
func (s *Server) handleDeleteGroup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	actorUsername := r.Header.Get("x-authenticated-user-id")
	groupName := strings.TrimSpace(r.PathValue("name"))
	if groupName == "" {
		http.Error(w, "GroupName is required", http.StatusBadRequest)
		return
	}
	if isBuiltInGroupName(groupName) {
		http.Error(w, "Built-in groups cannot be deleted", http.StatusBadRequest)
		return
	}

	var group userdb.UserGroup
	if err := db.DB.Where("name = ?", groupName).First(&group).Error; err != nil {
		http.Error(w, "Group not found", http.StatusNotFound)
		return
	}

	var memberCount int64
	if err := db.DB.Model(&userdb.UserGroupMembership{}).
		Where("user_group_id = ?", group.ID).
		Count(&memberCount).Error; err != nil {
		http.Error(w, fmt.Sprintf("Query group memberships failed: %v", err), http.StatusInternalServerError)
		return
	}
	if memberCount > 0 {
		http.Error(w, fmt.Sprintf("Cannot delete group with %d assigned member(s)", memberCount), http.StatusConflict)
		return
	}

	tx := db.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Where("user_group_id = ?", group.ID).Delete(&userdb.GroupPermission{}).Error; err != nil {
		tx.Rollback()
		http.Error(w, fmt.Sprintf("Failed to clear group permissions: %v", err), http.StatusInternalServerError)
		return
	}
	if err := tx.Delete(&group).Error; err != nil {
		tx.Rollback()
		http.Error(w, fmt.Sprintf("Failed to delete group: %v", err), http.StatusInternalServerError)
		return
	}
	if err := tx.Commit().Error; err != nil {
		http.Error(w, fmt.Sprintf("Commit failed: %v", err), http.StatusInternalServerError)
		return
	}

	userdb.RecordAuditLog(db.DB, actorUsername, "group_delete", "group", groupName,
		fmt.Sprintf("删除了自定义用户组 [%s]，显示名: %s", groupName, group.DisplayName), r.RemoteAddr)

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"success":true,"message":"User group deleted successfully"}`))
}

type SaveGroupPermissionsRequest struct {
	GroupName   string   `json:"group_name"`
	Permissions []string `json:"permissions"`
}

// handleSaveGroupPermissions updates the permission matrix mapping for a user group
func (s *Server) handleSaveGroupPermissions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	actorUsername := r.Header.Get("x-authenticated-user-id")

	var req SaveGroupPermissionsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.GroupName == "" {
		http.Error(w, "GroupName is required", http.StatusBadRequest)
		return
	}

	var group userdb.UserGroup
	if err := db.DB.Where("name = ?", req.GroupName).First(&group).Error; err != nil {
		http.Error(w, "Group not found", http.StatusNotFound)
		return
	}

	// Safety: Cannot modify super_admin permission mappings
	if group.Name == "super_admin" {
		http.Error(w, "Cannot modify 'super_admin' permissions matrix. Super admin always has all rights.", http.StatusBadRequest)
		return
	}

	tx := db.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Delete old permission mappings
	if err := tx.Where("user_group_id = ?", group.ID).Delete(&userdb.GroupPermission{}).Error; err != nil {
		tx.Rollback()
		http.Error(w, fmt.Sprintf("Failed to clear old mappings: %v", err), http.StatusInternalServerError)
		return
	}

	// Bind new permission mappings
	for _, code := range req.Permissions {
		var p userdb.Permission
		if err := tx.Where("code = ?", code).First(&p).Error; err == nil {
			gp := userdb.GroupPermission{
				UserGroupID:  group.ID,
				PermissionID: p.ID,
			}
			if err := tx.Create(&gp).Error; err != nil {
				tx.Rollback()
				http.Error(w, fmt.Sprintf("Failed to map permission %s: %v", code, err), http.StatusInternalServerError)
				return
			}
		}
	}

	if err := tx.Commit().Error; err != nil {
		http.Error(w, fmt.Sprintf("Commit failed: %v", err), http.StatusInternalServerError)
		return
	}

	userdb.RecordAuditLog(db.DB, actorUsername, "group_permissions_update", "group", req.GroupName,
		fmt.Sprintf("修改了用户组 [%s] 的权限矩阵，最新分配的权限集为: %v", group.DisplayName, req.Permissions), r.RemoteAddr)

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"success":true,"message":"Group permissions matrix updated successfully"}`))
}

// handleGetAuditLogs returns audit logs sorted by newest first
func (s *Server) handleGetAuditLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var logs []userdb.AuditLog
	if err := db.DB.Order("created_at desc").Limit(100).Find(&logs).Error; err != nil {
		http.Error(w, fmt.Sprintf("Query audit logs failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(logs)
}
