package server

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
	"well-ambient/internal/db"
	userdb "well-ambient/internal/db/user"
)

type NotificationAlert struct {
	ID        uint      `json:"id"`
	Type      string    `json:"type"` // delay, git_push, mr_event, ai_review, semantic_linker
	TaskID    string    `json:"task_id"`
	Title     string    `json:"title"`
	Assignee  string    `json:"assignee"`
	DelayDays int       `json:"delay_days"`
	Severity  string    `json:"severity"` // warning, critical, info
	Status    string    `json:"status"`
	Message   string    `json:"message"`
	Link      string    `json:"link"`       // Actionable URL (GitLab commit/MR page)
	CreatedAt time.Time `json:"created_at"` // Creation timestamp
}

// NotificationSender defines an extension interface for outgoing email/SMS notifications
type NotificationSender interface {
	SendDelayAlert(alert NotificationAlert) error
}

// EmailNotificationSender is a stub implementation of NotificationSender for future extensions
type EmailNotificationSender struct{}

func (e *EmailNotificationSender) SendDelayAlert(alert NotificationAlert) error {
	// Extension point: Send SMTP email alert
	log.Printf("Email extension hook: Send delay alert email to %s regarding task %s", alert.Assignee, alert.TaskID)
	return nil
}

// Active SSE client channels
var (
	clientsMu sync.Mutex
	clients   = make(map[chan struct{}]bool)
)

// BroadcastNotifications triggers an immediate refresh on all connected SSE clients
func BroadcastNotifications() {
	clientsMu.Lock()
	defer clientsMu.Unlock()
	for ch := range clients {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

// handleNotificationsSSE streams active task alerts and notifications to clients via SSE
func (s *Server) handleNotificationsSSE(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	userID := r.Header.Get("x-authenticated-user-id")
	if userID == "" {
		userID = r.URL.Query().Get("user_id")
	}
	if userID == "" {
		userID = "Eddie"
	}

	// Create notifier channel for this client.
	// Buffer size of 1 is sufficient as a notification is just a trigger signal.
	notifier := make(chan struct{}, 1)

	clientsMu.Lock()
	clients[notifier] = true
	clientsMu.Unlock()

	defer func() {
		clientsMu.Lock()
		delete(clients, notifier)
		clientsMu.Unlock()
		close(notifier)
	}()

	// Send initial payload immediately
	if !s.sendNotificationsEvent(w, r, flusher, userID) {
		return
	}

	// Keep a heartbeat ticker (15 seconds) to maintain connection open and ensure fallback sync
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			// Client disconnected
			return
		case <-notifier:
			if !s.sendNotificationsEvent(w, r, flusher, userID) {
				return
			}
		case <-ticker.C:
			if !s.sendNotificationsEvent(w, r, flusher, userID) {
				return
			}
		}
	}
}

// sendNotificationsEvent returns true on success, false if client connection closed/error detected
func (s *Server) sendNotificationsEvent(w http.ResponseWriter, r *http.Request, flusher http.Flusher, userID string) bool {
	// Guard: Do not serialize if client context is already aborted
	if r.Context().Err() != nil {
		return false
	}

	alerts := s.getMergedNotifications(userID)
	dataBytes, err := json.Marshal(alerts)
	if err != nil {
		log.Printf("SSE: failed to serialize alerts: %v", err)
		return true
	}

	// Direct write check to immediately intercept connection drops
	_, err = fmt.Fprintf(w, "data: %s\n\n", string(dataBytes))
	if err != nil {
		log.Printf("SSE: client disconnected (write failed): %v", err)
		return false
	}
	flusher.Flush()
	return true
}

func (s *Server) getMergedNotifications(userID string) []NotificationAlert {
	var merged []NotificationAlert

	// Load dismissed notification keys for this user
	dismissedKeys := make(map[string]bool)
	if db.DB != nil && userID != "" {
		var states []db.UserNotificationState
		if err := db.DB.Where("user_id = ? AND status = ?", userID, "dismissed").Find(&states).Error; err == nil {
			for _, st := range states {
				dismissedKeys[st.NotificationKey] = true
			}
		}
	}

	// 1. Dynamic delay alerts
	delays := s.computeDelayAlerts()
	for _, delay := range delays {
		// Key for delay is "delay_" + task_id
		key := fmt.Sprintf("delay_%s", delay.TaskID)
		if !dismissedKeys[key] {
			merged = append(merged, delay)
		}
	}

	// 2. Database persistent notifications (Git, AI review, Semantic link events)
	if db.DB != nil {
		var dbNotifs []db.Notification
		if err := db.DB.Order("created_at desc").Limit(50).Find(&dbNotifs).Error; err == nil {
			for _, dn := range dbNotifs {
				// Key for database persistent notification is "type_" + id
				key := fmt.Sprintf("%s_%d", dn.Type, dn.ID)
				if dismissedKeys[key] {
					continue
				}

				severity := "info"
				if dn.Type == "ai_review" {
					severity = "warning"
				} else if dn.Type == "semantic_linker" {
					severity = "info"
				}

				// Context Compression: Truncate message payload on SSE broadcast to limit network traffic
				msg := dn.Message
				if len(msg) > 150 {
					msg = msg[:150] + "... [已压缩]"
				}

				merged = append(merged, NotificationAlert{
					ID:        dn.ID,
					Type:      dn.Type,
					TaskID:    dn.TaskID,
					Title:     dn.Title,
					Assignee:  dn.Assignee,
					Severity:  severity,
					Message:   msg,
					Link:      dn.Link,
					CreatedAt: dn.CreatedAt,
				})
			}
		}
	}

	// 3. Sort by CreatedAt descending (Newest first)
	sort.Slice(merged, func(i, j int) bool {
		return merged[i].CreatedAt.After(merged[j].CreatedAt)
	})

	return merged
}

func (s *Server) computeDelayAlerts() []NotificationAlert {
	var alerts []NotificationAlert
	if db.DB == nil {
		return alerts
	}

	var tasks []db.TaskTelemetry
	// Select active tasks that are NOT done
	err := db.DB.Where("status != 'done'").Find(&tasks).Error
	if err != nil {
		log.Printf("SSE: failed to query active tasks: %v", err)
		return alerts
	}

	now := time.Now()
	for _, t := range tasks {
		if t.TaskCreatedAt.IsZero() {
			continue
		}

		delayDuration := now.Sub(t.TaskCreatedAt)
		delayDays := int(delayDuration.Hours() / 24)

		if delayDays >= 3 {
			severity := "warning"
			if delayDays >= 7 {
				severity = "critical"
			}

			statusLabel := ""
			switch strings.ToLower(t.Status) {
			case "backlog":
				statusLabel = "待办中"
			case "progress":
				if strings.ToLower(t.IssueType) == "bug" {
					statusLabel = "排查中 (Investigation)"
				} else {
					statusLabel = "开发中"
				}
			case "review":
				statusLabel = "代码评审中"
			default:
				statusLabel = t.Status
			}

			msg := fmt.Sprintf("[%s] %s 的任务 %s 已延期 %d 天 (%s)",
				strings.ToUpper(severity), t.Assignee, t.TaskID, delayDays, statusLabel)

			alert := NotificationAlert{
				Type:      "delay",
				TaskID:    t.TaskID,
				Title:     t.Title,
				Assignee:  t.Assignee,
				DelayDays: delayDays,
				Severity:  severity,
				Status:    t.Status,
				Message:   msg,
				CreatedAt: t.TaskCreatedAt.AddDate(0, 0, delayDays), // Align delay alert timestamp to when it crossed threshold
			}

			alerts = append(alerts, alert)

			// Trigger extension hook (e.g. Email Alert)
			var sender NotificationSender = &EmailNotificationSender{}
			_ = sender.SendDelayAlert(alert)
		}
	}

	return alerts
}

type NotificationReadRequest struct {
	UserID string   `json:"user_id"`
	Keys   []string `json:"keys"`
	Status string   `json:"status"` // e.g. "dismissed"
}

func (s *Server) handleMarkNotificationRead(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var req NotificationReadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Bad Request: %v", err), http.StatusBadRequest)
		return
	}

	// Security: override UserID with the authenticated user from JWT middleware
	authUserID := r.Header.Get("x-authenticated-user-id")
	if authUserID != "" {
		req.UserID = authUserID
	}

	if req.UserID == "" {
		http.Error(w, "Bad Request: missing authenticated user_id", http.StatusBadRequest)
		return
	}
	if len(req.Keys) == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"success","affected":0}`))
		return
	}

	if req.Status == "" {
		req.Status = "dismissed"
	}

	if db.DB == nil {
		http.Error(w, "Database connection not initialized", http.StatusInternalServerError)
		return
	}

	// Batch upsert to database using sqlite compatible logic
	tx := db.DB.Begin()
	for _, key := range req.Keys {
		var state db.UserNotificationState
		err := tx.Where("user_id = ? AND notification_key = ?", req.UserID, key).First(&state).Error
		if err == nil {
			// Record exists, update
			state.Status = req.Status
			state.UpdatedAt = time.Now()
			if err := tx.Save(&state).Error; err != nil {
				tx.Rollback()
				http.Error(w, fmt.Sprintf("Database Save Error: %v", err), http.StatusInternalServerError)
				return
			}
		} else {
			// Record does not exist, insert
			newState := db.UserNotificationState{
				UserID:          req.UserID,
				NotificationKey: key,
				Status:          req.Status,
				UpdatedAt:       time.Now(),
			}
			if err := tx.Create(&newState).Error; err != nil {
				tx.Rollback()
				http.Error(w, fmt.Sprintf("Database Create Error: %v", err), http.StatusInternalServerError)
				return
			}
		}
	}
	if err := tx.Commit().Error; err != nil {
		http.Error(w, fmt.Sprintf("Transaction Commit Error: %v", err), http.StatusInternalServerError)
		return
	}

	// Trigger broadcast to refresh all active SSE client sessions
	BroadcastNotifications()

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(fmt.Sprintf(`{"status":"success","affected":%d}`, len(req.Keys))))
}

type LoginRequest struct {
	Username          string `json:"username"`
	Password          string `json:"password"`
	EncryptedPassword string `json:"encrypted_password"`
}

func extractDept(val interface{}) string {
	return extractDeptHelper(val, false)
}

func extractDeptCandidate(val interface{}) string {
	return extractDeptHelper(val, true)
}

func extractDeptHelper(val interface{}, inDeptContext bool) string {
	if val == nil {
		return ""
	}
	if s, ok := val.(string); ok {
		s = normalizeDeptString(s)
		if inDeptContext && s != "" {
			return s
		}
		return ""
	}
	if m, ok := val.(map[string]interface{}); ok {
		// 1. 优先在当前层级寻找明确的部门键
		deptKeys := []string{
			"department_name", "departmentName", "department", "department_info",
			"department_display_name", "departmentDisplayName", "department_full_name", "departmentFullName",
			"dept_name", "deptName", "dept", "dept_info", "dept_id_data",
			"dept_display_name", "deptDisplayName", "dept_full_name", "deptFullName",
			"dep_name", "depName", "dep",
			"organization_name", "organizationName", "organization", "org_name", "orgName", "org",
			"organization_display_name", "organizationDisplayName", "organization_full_name", "organizationFullName",
			"unit_name", "unitName", "unit",
			"部门", "部门名称", "组织", "组织名称",
		}
		for _, k := range deptKeys {
			if v, exists := m[k]; exists {
				if res := extractDeptHelper(v, true); res != "" {
					return res
				}
			}
		}

		// 2. 检查是否有包含 dept 或 department 的键，并进入部门上下文
		for k, v := range m {
			kLower := strings.ToLower(k)
			if isDeptKeywordKey(kLower) {
				if res := extractDeptHelper(v, true); res != "" {
					return res
				}
			}
		}

		// 3. 只有当处于部门上下文时，我们才允许提取 "name", "label", "title" 等作为部门名
		if inDeptContext {
			for _, k := range []string{"name", "label", "title", "display_name", "displayName", "full_name", "fullName", "path_name", "pathName", "cn_name", "cnName", "text", "value"} {
				if v, exists := m[k]; exists {
					if res := extractDeptHelper(v, true); res != "" {
						return res
					}
				}
			}
		}

		// 4. 如果没有找到，递归遍历所有其他键，排除明显的个人属性键。
		// 非部门键只在普通上下文里搜索，避免把 user_info.name 误判成部门名。
		for k, v := range m {
			kLower := strings.ToLower(k)
			if kLower == "name" || kLower == "username" || kLower == "email" || kLower == "avatar" || kLower == "avatar_url" || kLower == "token" {
				continue
			}
			nextDeptContext := inDeptContext && isDeptWrapperKey(kLower)
			if res := extractDeptHelper(v, nextDeptContext); res != "" {
				return res
			}
		}
	}
	if slice, ok := val.([]interface{}); ok {
		for _, item := range slice {
			if res := extractDeptHelper(item, inDeptContext); res != "" {
				return res
			}
		}
	}
	return ""
}

func isDeptWrapperKey(key string) bool {
	switch key {
	case "data", "info", "detail", "details", "value", "values", "item", "items", "list", "nodes", "children":
		return true
	default:
		return false
	}
}

func isDeptKeywordKey(key string) bool {
	if strings.Contains(key, "dept") ||
		strings.Contains(key, "department") ||
		strings.Contains(key, "organization") ||
		strings.Contains(key, "org_") ||
		strings.Contains(key, "unit_") ||
		strings.Contains(key, "部门") ||
		strings.Contains(key, "组织") {
		return true
	}
	switch key {
	case "org", "org_name", "orgname", "unit", "unit_name", "unitname", "dep", "dep_name", "depname":
		return true
	default:
		return false
	}
}

func normalizeDeptString(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" || s == "-" {
		return ""
	}
	lower := strings.ToLower(s)
	if lower == "null" || lower == "none" || lower == "undefined" {
		return ""
	}

	return s
}

func redactSensitiveLoginPayload(body []byte) string {
	var payload interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return fmt.Sprintf("[non-json response omitted; %d bytes]", len(body))
	}

	redacted := redactSensitiveLoginValue(payload)
	redactedBytes, err := json.Marshal(redacted)
	if err != nil {
		return fmt.Sprintf("[unserializable response omitted; %d bytes]", len(body))
	}
	return string(redactedBytes)
}

func redactSensitiveLoginValue(val interface{}) interface{} {
	switch typed := val.(type) {
	case map[string]interface{}:
		out := make(map[string]interface{}, len(typed))
		for k, v := range typed {
			if isSensitiveLoginKey(k) {
				out[k] = "[REDACTED]"
			} else {
				out[k] = redactSensitiveLoginValue(v)
			}
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(typed))
		for i, item := range typed {
			out[i] = redactSensitiveLoginValue(item)
		}
		return out
	default:
		return val
	}
}

func isSensitiveLoginKey(key string) bool {
	k := strings.ToLower(strings.TrimSpace(key))
	switch k {
	case "token", "access_token", "refresh_token", "id_token", "session", "session_id", "sessionid",
		"secret", "client_secret", "authorization", "password", "passwd", "encrypted_password":
		return true
	}
	return strings.Contains(k, "token") ||
		strings.Contains(k, "secret") ||
		strings.Contains(k, "password") ||
		strings.Contains(k, "passwd")
}

func extractString(val interface{}, key string) string {
	if val == nil {
		return ""
	}
	if m, ok := val.(map[string]interface{}); ok {
		if v, exists := m[key]; exists {
			if s, ok := v.(string); ok {
				return s
			}
			return fmt.Sprintf("%v", v)
		}
	}
	if slice, ok := val.([]interface{}); ok {
		for _, item := range slice {
			if res := extractString(item, key); res != "" {
				return res
			}
		}
	}
	return ""
}

type WellOSLoginResponse struct {
	Code       int         `json:"code"`
	Msg        string      `json:"msg"`
	Data       interface{} `json:"data"`
	Token      string      `json:"token"`
	Name       string      `json:"name"`
	Expire     int         `json:"expire"`
	Avatar     string      `json:"avatar"`
	AvatarURL  string      `json:"avatar_url"`
	Department interface{} `json:"department"`
	Dept       interface{} `json:"dept"`
	DeptIdData interface{} `json:"dept_id_data"`
}

const wellOSLoginURL = "https://wellos.westwell-lab.com/api/user/login"

var wellOSLoginDoer = doWellOSLoginRequest

const localPasswordHashIterations = 210000

func newWellOSLoginClient() *http.Client {
	transport := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		ForceAttemptHTTP2:   false,
		DisableKeepAlives:   true,
		MaxIdleConns:        0,
		IdleConnTimeout:     0,
		TLSHandshakeTimeout: 8 * time.Second,
	}
	return &http.Client{
		Timeout:   15 * time.Second,
		Transport: transport,
	}
}

func newWellOSLoginRequest(username, password string) (*http.Request, error) {
	payload := map[string]string{
		"account": username,
		"passwd":  password,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, wellOSLoginURL, strings.NewReader(string(payloadBytes)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("User-Agent", "Mozilla/5.0 well-ambient/1.0")
	req.Header.Set("Origin", "https://wellos.westwell-lab.com")
	req.Header.Set("Referer", "https://wellos.westwell-lab.com/")
	req.Close = true
	return req, nil
}

func isRetryableWellOSLoginError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "eof") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "server closed idle connection") {
		return true
	}
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		return true
	}
	return false
}

func doWellOSLoginRequest(client *http.Client, username, password string) (*http.Response, error) {
	req, err := newWellOSLoginRequest(username, password)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err == nil || !isRetryableWellOSLoginError(err) {
		return resp, err
	}
	log.Printf("WellOS login request failed once with retryable network error: %v; retrying with a fresh connection", err)
	req, retryReqErr := newWellOSLoginRequest(username, password)
	if retryReqErr != nil {
		return nil, retryReqErr
	}
	return client.Do(req)
}

func deriveLocalPasswordHash(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key, err := pbkdf2.Key(sha256.New, password, salt, localPasswordHashIterations, 32)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(
		"pbkdf2-sha256$%d$%s$%s",
		localPasswordHashIterations,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

func verifyLocalPasswordHash(password, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2-sha256" {
		return false
	}
	var iterations int
	if _, err := fmt.Sscanf(parts[1], "%d", &iterations); err != nil || iterations <= 0 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil {
		return false
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil {
		return false
	}
	actual, err := pbkdf2.Key(sha256.New, password, salt, iterations, len(expected))
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(actual, expected) == 1
}

func (s *Server) handleDegradedLogin(w http.ResponseWriter, r *http.Request, username, password string, upstreamErr error) {
	if isDevAuthEnabled() && isLoopbackRequest(r) {
		s.handleDevLogin(w, r, username, upstreamErr)
		return
	}

	if db.DB == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte(fmt.Sprintf(`{"status":"error", "message":"Authentication service unreachable: %v"}`, upstreamErr)))
		return
	}

	var dbUser userdb.User
	if err := db.DB.Where("username = ?", username).First(&dbUser).Error; err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte(`{"status":"error", "message":"WellOS is under maintenance and no local session fallback is available for this account"}`))
		return
	}
	if dbUser.LocalPasswordHash == "" || !verifyLocalPasswordHash(password, dbUser.LocalPasswordHash) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"status":"error", "message":"WellOS is under maintenance and local credential verification failed for this account"}`))
		return
	}

	groupNames, permCodes := loadUserAccessSnapshot(dbUser.ID)
	if len(groupNames) == 0 || len(permCodes) == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"status":"error", "message":"WellOS is under maintenance and this account has no local access snapshot"}`))
		return
	}

	role := roleFromGroups(groupNames)
	token, err := GenerateJWT(dbUser.Username, dbUser.Name, "wellos-maintenance-fallback", dbUser.Avatar, groupNames, permCodes, dbUser.Department)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"status":"error", "message":"Failed to generate degraded session token"}`))
		return
	}

	detail := fmt.Sprintf("WellOS unavailable, issued degraded local session for existing user. upstream=%v", upstreamErr)
	userdb.RecordAuditLog(db.DB, dbUser.Username, "user_login_degraded", "user", fmt.Sprintf("%d", dbUser.ID), detail, r.RemoteAddr)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":          "success",
		"token":           token,
		"degraded":        true,
		"degraded_reason": "wellos_unavailable",
		"message":         "WellOS 维护中，已使用本地已知用户资料创建临时会话",
		"user": map[string]interface{}{
			"username":    dbUser.Username,
			"name":        dbUser.Name,
			"avatar":      dbUser.Avatar,
			"role":        role,
			"groups":      groupNames,
			"permissions": permCodes,
			"department":  dbUser.Department,
		},
	})
}

func (s *Server) handleDevLogin(w http.ResponseWriter, r *http.Request, username string, upstreamErr error) {
	if db.DB == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte(fmt.Sprintf(`{"status":"error", "message":"Dev auth unavailable: database is not initialized; upstream=%v"}`, upstreamErr)))
		return
	}

	name := "Local Developer"
	if parts := strings.Split(username, "@"); len(parts) > 0 && strings.TrimSpace(parts[0]) != "" {
		name = strings.TrimSpace(parts[0])
	}

	var dbUser userdb.User
	if err := db.DB.Where("username = ?", username).First(&dbUser).Error; err != nil {
		dbUser = userdb.User{
			Username:   username,
			Email:      username,
			Name:       name,
			Department: "Local Development",
		}
		if err := db.DB.Create(&dbUser).Error; err != nil {
			log.Printf("Error creating local dev user %s: %v", username, err)
		}
	} else {
		if dbUser.Name == "" {
			dbUser.Name = name
		}
		if isMissingDepartment(dbUser.Department) {
			dbUser.Department = "Local Development"
		}
		db.DB.Save(&dbUser)
	}

	var superGroup userdb.UserGroup
	if err := db.DB.Where("name = ?", "super_admin").First(&superGroup).Error; err == nil {
		var count int64
		db.DB.Model(&userdb.UserGroupMembership{}).
			Where("user_id = ? AND user_group_id = ? AND scope = ? AND scope_id = ?", dbUser.ID, superGroup.ID, "global", "").
			Count(&count)
		if count == 0 {
			db.DB.Create(&userdb.UserGroupMembership{
				UserID:      dbUser.ID,
				UserGroupID: superGroup.ID,
				Scope:       "global",
				ScopeID:     "",
			})
		}
	}

	groupNames, permCodes := loadUserAccessSnapshot(dbUser.ID)
	if len(groupNames) == 0 {
		groupNames = []string{"super_admin"}
	}
	role := roleFromGroups(groupNames)
	token, err := GenerateJWT(dbUser.Username, dbUser.Name, "dev-local-auth", dbUser.Avatar, groupNames, permCodes, dbUser.Department)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"status":"error", "message":"Failed to generate dev session token"}`))
		return
	}

	detail := fmt.Sprintf("Local development auth issued because WellOS was unavailable. upstream=%v", upstreamErr)
	userdb.RecordAuditLog(db.DB, dbUser.Username, "user_login_dev", "user", fmt.Sprintf("%d", dbUser.ID), detail, r.RemoteAddr)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":          "success",
		"token":           token,
		"degraded":        true,
		"degraded_reason": "local_dev_auth",
		"message":         "本地开发模式已启用，已绕过 WellOS 创建本地调试会话",
		"user": map[string]interface{}{
			"username":    dbUser.Username,
			"name":        dbUser.Name,
			"avatar":      dbUser.Avatar,
			"role":        role,
			"groups":      groupNames,
			"permissions": permCodes,
			"department":  dbUser.Department,
		},
	})
}

func loadUserAccessSnapshot(userID uint) ([]string, []string) {
	var groupNames []string
	db.DB.Table("user_groups").
		Joins("join user_group_memberships on user_group_memberships.user_group_id = user_groups.id").
		Where("user_group_memberships.user_id = ?", userID).
		Pluck("user_groups.name", &groupNames)

	var permCodes []string
	db.DB.Table("permissions").
		Joins("join group_permissions on group_permissions.permission_id = permissions.id").
		Joins("join user_group_memberships on user_group_memberships.user_group_id = group_permissions.user_group_id").
		Where("user_group_memberships.user_id = ?", userID).
		Distinct("permissions.code").
		Pluck("permissions.code", &permCodes)

	return groupNames, permCodes
}

func roleFromGroups(groupNames []string) string {
	role := "member"
	for _, g := range groupNames {
		if g == "super_admin" {
			return "super_admin"
		}
		if g == "admin" {
			role = "admin"
		}
	}
	return role
}

func isDevAuthEnabled() bool {
	return os.Getenv("WELL_AMBIENT_DEV_AUTH") == "1"
}

func isLoopbackRequest(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// If encrypted password is provided, decrypt it first using Django's SECRET_KEY compatibility
	if req.EncryptedPassword != "" {
		decrypted, err := DecryptCryptoJSAES(req.EncryptedPassword, "django-insecure-()$+l&t333b4ncc0hrw!u!^yd_oja&0qc4n#&xnfcm)5r^n$5k")
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to decrypt password: %v", err), http.StatusBadRequest)
			return
		}
		req.Password = decrypted
	}

	if req.Username == "" || req.Password == "" {
		http.Error(w, "Username and password are required", http.StatusBadRequest)
		return
	}

	client := newWellOSLoginClient()
	resp, err := wellOSLoginDoer(client, req.Username, req.Password)
	if err != nil {
		s.handleDegradedLogin(w, r, req.Username, req.Password, err)
		return
	}
	defer resp.Body.Close()

	var loginResp WellOSLoginResponse
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf(`{"status":"error", "message":"Failed to read auth response body: %v"}`, err)))
		return
	}
	log.Printf("[DEBUG] WellOS login raw response body (redacted): %s", redactSensitiveLoginPayload(bodyBytes))

	if err := json.Unmarshal(bodyBytes, &loginResp); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf(`{"status":"error", "message":"Failed to decode auth response: %v"}`, err)))
		return
	}

	token := loginResp.Token
	name := loginResp.Name
	avatar := loginResp.Avatar
	if avatar == "" {
		avatar = loginResp.AvatarURL
	}

	if token == "" && loginResp.Data != nil {
		token = extractString(loginResp.Data, "token")
		if name == "" {
			name = extractString(loginResp.Data, "name")
		}
		if avatar == "" {
			avatar = extractString(loginResp.Data, "avatar")
		}
		if avatar == "" {
			avatar = extractString(loginResp.Data, "avatar_url")
		}
	}

	// 智能拼接 WellOS 头像绝对路径
	if avatar != "" && !strings.HasPrefix(avatar, "http://") && !strings.HasPrefix(avatar, "https://") {
		if strings.HasPrefix(avatar, "/") {
			avatar = "https://wellos.westwell-lab.com" + avatar
		} else {
			avatar = "https://wellos.westwell-lab.com/" + avatar
		}
	}

	if loginResp.Code != 0 || token == "" {
		errMsg := loginResp.Msg
		if errMsg == "" {
			errMsg = "Authentication failed"
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(fmt.Sprintf(`{"status":"error", "message":%q}`, errMsg)))
		return
	}

	// Extract department from login response
	dept := extractDeptCandidate(loginResp.Department)
	if dept == "" {
		dept = extractDeptCandidate(loginResp.Dept)
	}
	if dept == "" {
		dept = extractDeptCandidate(loginResp.DeptIdData)
	}
	if dept == "" && loginResp.Data != nil {
		dept = extractDept(loginResp.Data)
	}

	// Create or update user in our database
	var dbUser userdb.User
	var userCount int64
	db.DB.Model(&userdb.User{}).Count(&userCount)

	err = db.DB.Where("username = ?", req.Username).First(&dbUser).Error
	if err != nil {
		// New User
		dbUser = userdb.User{
			Username:   req.Username,
			Email:      req.Username, // In App.svelte username prefix is email prefix, req.Username is email
			Name:       name,
			Avatar:     avatar,
			Department: dept,
		}
		if err := db.DB.Create(&dbUser).Error; err != nil {
			log.Printf("Error creating user %s: %v", req.Username, err)
		}

		// Determine group assignment (first user gets super_admin, others get member)
		groupName := "member"
		if userCount == 0 {
			groupName = "super_admin"
		}

		var defaultGroup userdb.UserGroup
		if err := db.DB.Where("name = ?", groupName).First(&defaultGroup).Error; err == nil {
			membership := userdb.UserGroupMembership{
				UserID:      dbUser.ID,
				UserGroupID: defaultGroup.ID,
				Scope:       "global",
				ScopeID:     "",
			}
			db.DB.Create(&membership)
		}
	} else {
		// Existing User, refresh info
		dbUser.Name = name
		dbUser.Avatar = avatar
		if dept != "" {
			dbUser.Department = dept
		}
	}

	passwordHash, hashErr := deriveLocalPasswordHash(req.Password)
	if hashErr != nil {
		log.Printf("Failed to derive local maintenance password hash for %s: %v", req.Username, hashErr)
	} else {
		now := time.Now()
		dbUser.LocalPasswordHash = passwordHash
		dbUser.LocalPasswordUpdatedAt = &now
		db.DB.Save(&dbUser)
	}

	groupNames, permCodes := loadUserAccessSnapshot(dbUser.ID)
	role := roleFromGroups(groupNames)

	// Generate our own JWT token (valid for 2 hours)
	ourToken, err := GenerateJWT(req.Username, name, token, avatar, groupNames, permCodes, dbUser.Department)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"status":"error", "message":"Failed to generate session token"}`))
		return
	}

	// Record audit log for successful login
	userdb.RecordAuditLog(db.DB, req.Username, "user_login", "user", fmt.Sprintf("%d", dbUser.ID), "用户成功登录系统并获取 Session", r.RemoteAddr)

	// Return response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"token":  ourToken,
		"user": map[string]interface{}{
			"username":    req.Username,
			"name":        name,
			"avatar":      avatar,
			"role":        role,
			"groups":      groupNames,
			"permissions": permCodes,
			"department":  dbUser.Department,
		},
	})
}
