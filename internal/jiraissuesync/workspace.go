package jiraissuesync

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"well-ambient/internal/db"
	"well-ambient/internal/openaccess"
)

var projectPattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,39}$`)
var taskPattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,39}-[1-9][0-9]*$`)

type Mapping struct {
	Project  string   `json:"project"`
	RepoIDs  []string `json:"repo_ids"`
	TaskKeys []string `json:"task_keys"` // Empty means all issues of this Jira project.
}
type WorkspaceConfig struct {
	Version     int       `json:"version"`
	Enabled     bool      `json:"enabled"`
	TimeOfDay   string    `json:"time_of_day"`
	GroupURL    string    `json:"group_url"`
	JiraURL     string    `json:"jira_url"`
	Mappings    []Mapping `json:"mappings"`
	GitLabToken string    `json:"gitlab_token,omitempty"` // write-only
	JiraToken   string    `json:"jira_token,omitempty"`   // write-only, Jira Server/DC bearer PAT
}
type Repo struct {
	ID            int64  `json:"id"`
	Path          string `json:"path_with_namespace"`
	DefaultBranch string `json:"default_branch"`
	WebURL        string `json:"web_url"`
	Archived      bool   `json:"archived"`
	IssuesEnabled bool   `json:"issues_enabled"`
}
type Project struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}
type Catalog struct {
	Repositories []Repo    `json:"repositories"`
	Projects     []Project `json:"projects"`
	CheckedAt    time.Time `json:"checked_at"`
}
type SourceEvidence struct {
	RepoID string `json:"repo_id"`
	Path   string `json:"path"`
	SHA    string `json:"sha"`
}
type Proposal struct {
	Mapping
	Reason   string           `json:"reason"`
	Evidence []SourceEvidence `json:"evidence"`
}
type AnalysisTaskDetail struct {
	Key         string `json:"key"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

type RunView struct {
	GroupURL        string               `json:"group_url,omitempty"`
	JiraURL         string               `json:"jira_url,omitempty"`
	AnalysisCatalog *Catalog             `json:"analysis_catalog,omitempty"`
	TaskDetails     []AnalysisTaskDetail `json:"task_details,omitempty"`
	db.JiraIssueRun
	Proposals  []Proposal `json:"proposals,omitempty"`
	Items      []SyncItem `json:"items,omitempty"`
	ItemsTotal int        `json:"items_total"`
	NextOffset int        `json:"next_offset"`
}
type WorkspaceView struct {
	Settings         WorkspaceConfig `json:"settings"`
	Catalog          Catalog         `json:"catalog"`
	Runs             []RunView       `json:"runs"`
	GitLabConfigured bool            `json:"gitlab_configured"`
	JiraConfigured   bool            `json:"jira_configured"`
	KeyReady         bool            `json:"key_ready"`
	AIReady          bool            `json:"ai_ready"`
	Busy             bool            `json:"busy"`
}
type credentials struct {
	GitLab string `json:"gitlab"`
	Jira   string `json:"jira"`
}
type Workspace struct {
	DB     *gorm.DB
	Key    []byte
	Client *http.Client
	AI     func(context.Context, string, string) (string, error)
	Now    func() time.Time
}

func (s *Workspace) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}
func marshal(v any) string          { b, _ := json.Marshal(v); return string(b) }
func invalid(message string) error  { return openaccess.NewError("invalid_request", message) }
func conflict(message string) error { return openaccess.NewError("conflict", message) }
func (s *Workspace) row(ctx context.Context) (db.JiraIssueWorkspace, error) {
	d := db.JiraIssueWorkspace{ID: 1, Version: 1, ConfigJSON: marshal(WorkspaceConfig{Version: 1, TimeOfDay: "09:00", Mappings: []Mapping{}}), CatalogJSON: marshal(Catalog{Repositories: []Repo{}, Projects: []Project{}})}
	if err := s.DB.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&d).Error; err != nil {
		return d, err
	}
	err := s.DB.WithContext(ctx).First(&d, 1).Error
	return d, err
}
func (s *Workspace) crypt(value string, decrypt bool) (string, error) {
	if len(s.Key) != 32 {
		return "", invalid("独立凭据加密密钥未配置（WELL_JIRA_ISSUE_SYNC_KEY）")
	}
	block, err := aes.NewCipher(s.Key)
	if err != nil {
		return "", err
	}
	gcm, _ := cipher.NewGCM(block)
	if decrypt {
		b, e := base64.StdEncoding.DecodeString(value)
		if e != nil || len(b) < gcm.NonceSize() {
			return "", errors.New("独立凭据无法解密")
		}
		plain, e := gcm.Open(nil, b[:gcm.NonceSize()], b[gcm.NonceSize():], []byte("jira-issue-workspace-v1"))
		if e != nil {
			return "", errors.New("独立凭据无法解密")
		}
		return string(plain), nil
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(gcm.Seal(nonce, nonce, []byte(value), []byte("jira-issue-workspace-v1"))), nil
}
func (s *Workspace) secrets(d db.JiraIssueWorkspace) (credentials, error) {
	var c credentials
	if d.Credentials == "" {
		return c, nil
	}
	v, err := s.crypt(d.Credentials, true)
	if err == nil {
		err = json.Unmarshal([]byte(v), &c)
	}
	return c, err
}
func decodeConfig(d db.JiraIssueWorkspace) WorkspaceConfig {
	var c WorkspaceConfig
	_ = json.Unmarshal([]byte(d.ConfigJSON), &c)
	c.Version = d.Version
	return c
}
func (s *Workspace) View(ctx context.Context) (WorkspaceView, error) {
	d, err := s.row(ctx)
	if err != nil {
		return WorkspaceView{}, err
	}
	v := WorkspaceView{Settings: decodeConfig(d), KeyReady: len(s.Key) == 32, AIReady: s.AI != nil, Busy: d.Lease != "" && d.LeaseUntil.After(s.now()), Runs: []RunView{}}
	_ = json.Unmarshal([]byte(d.CatalogJSON), &v.Catalog)
	c, err := s.secrets(d)
	if err != nil && len(s.Key) == 32 {
		return v, err
	}
	v.GitLabConfigured = c.GitLab != ""
	v.JiraConfigured = c.Jira != ""
	var rows []db.JiraIssueRun
	if err = s.DB.WithContext(ctx).Select("id", "kind", "config_version", "status", "message", "item_count", "created_at", "updated_at").Order("id DESC").Limit(20).Find(&rows).Error; err != nil {
		return v, err
	}
	analysisIncluded := false
	for _, r := range rows {
		item := RunView{JiraIssueRun: r}
		if len(v.Runs) == 0 || (r.Kind == "analyze" && r.Status == "succeeded" && !analysisIncluded) {
			var payload db.JiraIssueRun
			if err := s.DB.WithContext(ctx).Select("result_json").First(&payload, r.ID).Error; err != nil {
				return v, err
			}
			_ = json.Unmarshal([]byte(payload.ResultJSON), &item)
		}
		item.JiraIssueRun = r
		item.ItemsTotal = r.ItemCount
		if len(item.Items) > 5 {
			item.Items = item.Items[:5]
			item.NextOffset = 5
		}
		if item.Kind == "analyze" && item.Status == "succeeded" && !analysisIncluded {
			analysisIncluded = true
		} else {
			item.Proposals = nil
		}
		if len(v.Runs) > 0 {
			item.Items = nil
		}
		if item.Status == "running" && (!v.Busy || r.ID != d.RunID) {
			item.Status = "interrupted"
			item.Message = "运行中断，可重试并核对远端结果"
		}
		v.Runs = append(v.Runs, item)
	}
	return v, nil
}

// URLs are administrator-provided private integrations. Keep credentials out of URLs,
// reject metadata/link-local targets, and never follow redirects with credentials.
func endpoint(raw string, group bool) (string, string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", "", invalid("请输入不含凭据、参数的 HTTP(S) 地址")
	}
	host := strings.ToLower(u.Hostname())
	if host == "metadata.google.internal" || strings.HasPrefix(host, "169.254.") || strings.HasPrefix(host, "fe80:") {
		return "", "", invalid("不允许元数据或链路本地地址")
	}
	path := strings.Trim(u.Path, "/")
	if group {
		path = strings.TrimPrefix(path, "groups/")
		path = strings.TrimSuffix(path, "/-")
		if path == "" || strings.Contains(path, "/-/") {
			return "", "", invalid("请输入 GitLab 群组地址")
		}
	}
	for _, p := range strings.Split(path, "/") {
		if p == "." || p == ".." {
			return "", "", invalid("地址路径无效")
		}
	}
	base := strings.ToLower(u.Scheme) + "://" + strings.ToLower(u.Host)
	if !group && path != "" {
		base += "/" + path
	}
	return base, path, nil
}
func (s *Workspace) Save(ctx context.Context, c WorkspaceConfig) (WorkspaceView, error) {
	d, err := s.row(ctx)
	if err != nil {
		return WorkspaceView{}, err
	}
	old := decodeConfig(d)
	if c.Version != d.Version {
		return WorkspaceView{}, conflict("配置已更新，请重新加载")
	}
	if d.Lease != "" && d.LeaseUntil.After(s.now()) {
		return WorkspaceView{}, conflict("运行中，请先停止或等待结束")
	}
	if !regexp.MustCompile(`^(?:[01][0-9]|2[0-3]):[0-5][0-9]$`).MatchString(c.TimeOfDay) {
		return WorkspaceView{}, invalid("时间应为 HH:mm")
	}
	if len(c.Mappings) > 100 {
		return WorkspaceView{}, invalid("最多配置 100 条映射")
	}
	if len(c.GroupURL) > 512 || len(c.JiraURL) > 512 {
		return WorkspaceView{}, invalid("连接地址不能超过 512 字符")
	}
	if c.GroupURL != "" {
		if _, _, err = endpoint(c.GroupURL, true); err != nil {
			return WorkspaceView{}, err
		}
	}
	if c.JiraURL != "" {
		if _, _, err = endpoint(c.JiraURL, false); err != nil {
			return WorkspaceView{}, err
		}
	}
	secret, err := s.secrets(d)
	if err != nil {
		return WorkspaceView{}, err
	}
	if old.GroupURL != c.GroupURL && secret.GitLab != "" && c.GitLabToken == "" {
		return WorkspaceView{}, invalid("变更群组地址时请重新填写 GitLab Token")
	}
	if old.JiraURL != c.JiraURL && secret.Jira != "" && c.JiraToken == "" {
		return WorkspaceView{}, invalid("变更 Jira 地址时请重新填写 Jira Token")
	}
	if len(c.GitLabToken) > 4096 || len(c.JiraToken) > 4096 {
		return WorkspaceView{}, invalid("Token 长度超出范围")
	}
	if c.GitLabToken != "" {
		secret.GitLab = c.GitLabToken
	}
	if c.JiraToken != "" {
		secret.Jira = c.JiraToken
	}
	c.GitLabToken = ""
	c.JiraToken = ""
	var cat Catalog
	_ = json.Unmarshal([]byte(d.CatalogJSON), &cat)
	if old.GroupURL != c.GroupURL || old.JiraURL != c.JiraURL {
		cat = Catalog{Repositories: []Repo{}, Projects: []Project{}}
		if len(c.Mappings) > 0 {
			return WorkspaceView{}, invalid("连接地址变更后，请清空映射并重新读取仓库")
		}
	}
	totalTasks := 0
	seen := map[string]bool{}
	for i := range c.Mappings {
		m := &c.Mappings[i]
		if !projectPattern.MatchString(m.Project) || len(m.RepoIDs) == 0 || len(m.RepoIDs) > 20 || len(m.TaskKeys) > 100 {
			return WorkspaceView{}, invalid("映射需要有效 Jira 项目和 1–20 个仓库")
		}
		if !hasProject(cat, m.Project) {
			return WorkspaceView{}, invalid("请先读取项目，再选择 Jira 项目")
		}
		sort.Strings(m.RepoIDs)
		m.RepoIDs = unique(m.RepoIDs)
		sort.Strings(m.TaskKeys)
		m.TaskKeys = unique(m.TaskKeys)
		for _, id := range m.RepoIDs {
			if _, ok := findRepo(cat, id); !ok {
				return WorkspaceView{}, invalid("仓库不在当前群组目录中")
			}
		}
		totalTasks += len(m.TaskKeys)
		if totalTasks > 1000 {
			return WorkspaceView{}, invalid("映射任务编号合计最多 1000 个")
		}
		for _, key := range m.TaskKeys {
			if len(key) > 64 || !taskPattern.MatchString(key) || !strings.HasPrefix(key, m.Project+"-") {
				return WorkspaceView{}, invalid("任务编号必须属于映射的 Jira 项目")
			}
		}
		sig := marshal(*m)
		if seen[sig] {
			return WorkspaceView{}, invalid("存在重复映射")
		}
		seen[sig] = true
	}
	if c.Enabled && (c.GroupURL == "" || c.JiraURL == "" || secret.GitLab == "" || secret.Jira == "" || len(c.Mappings) == 0) {
		return WorkspaceView{}, invalid("启用前请完成独立连接和项目映射")
	}
	encrypted := ""
	if secret.GitLab != "" || secret.Jira != "" {
		encrypted, err = s.crypt(marshal(secret), false)
		if err != nil {
			return WorkspaceView{}, err
		}
	}
	c.Version++
	q := s.DB.WithContext(ctx).Model(&db.JiraIssueWorkspace{}).Where("id = 1 AND version = ? AND (lease = '' OR lease_until <= ?)", d.Version, s.now()).Updates(map[string]any{"version": c.Version, "config_json": marshal(c), "credentials": encrypted, "catalog_json": marshal(cat), "updated_at": s.now()})
	if q.Error != nil {
		return WorkspaceView{}, q.Error
	}
	if q.RowsAffected != 1 {
		return WorkspaceView{}, conflict("配置或运行状态已变化")
	}
	return s.View(ctx)
}
func unique(in []string) []string {
	out := []string{}
	for _, v := range in {
		if len(out) == 0 || out[len(out)-1] != v {
			out = append(out, v)
		}
	}
	return out
}
func findRepo(c Catalog, id string) (Repo, bool) {
	for _, r := range c.Repositories {
		if fmt.Sprint(r.ID) == id {
			return r, true
		}
	}
	return Repo{}, false
}
func hasProject(c Catalog, key string) bool {
	for _, p := range c.Projects {
		if p.Key == key {
			return true
		}
	}
	return false
}

// Begin is a durable cross-process lease; callers can safely launch Execute asynchronously.
func (s *Workspace) Begin(ctx context.Context, version int, kind string) (db.JiraIssueRun, string, error) {
	if kind != "discover" && kind != "analyze" && kind != "sync" {
		return db.JiraIssueRun{}, "", invalid("未知操作")
	}
	d, err := s.row(ctx)
	if err != nil {
		return db.JiraIssueRun{}, "", err
	}
	if version != d.Version {
		return db.JiraIssueRun{}, "", conflict("配置已更新，请重新加载")
	}
	c := decodeConfig(d)
	secrets, err := s.secrets(d)
	if err != nil {
		return db.JiraIssueRun{}, "", err
	}
	if c.GroupURL == "" || c.JiraURL == "" || secrets.GitLab == "" || secrets.Jira == "" {
		return db.JiraIssueRun{}, "", invalid("请先保存两端独立连接")
	}
	if kind == "analyze" && s.AI == nil {
		return db.JiraIssueRun{}, "", invalid("AI 服务未启用")
	}
	if kind == "sync" && len(c.Mappings) == 0 {
		return db.JiraIssueRun{}, "", invalid("请先配置项目映射")
	}
	lease, err := openaccess.RandomID("jis")
	if err != nil {
		return db.JiraIssueRun{}, "", err
	}
	run := db.JiraIssueRun{Kind: kind, ConfigVersion: version, Status: "running", Message: "准备运行", CreatedAt: s.now(), UpdatedAt: s.now()}
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&run).Error; err != nil {
			return err
		}
		q := tx.Model(&db.JiraIssueWorkspace{}).Where("id = 1 AND version = ? AND (lease = '' OR lease_until <= ?)", version, s.now()).Updates(map[string]any{"lease": lease, "lease_until": s.now().Add(10 * time.Minute), "cancelled": false, "run_id": run.ID})
		if q.Error != nil {
			return q.Error
		}
		if q.RowsAffected != 1 {
			return conflict("已有运行，请等待或停止")
		}
		return nil
	})
	return run, lease, err
}
func (s *Workspace) Cancel(ctx context.Context, runID uint) error {
	q := s.DB.WithContext(ctx).Model(&db.JiraIssueWorkspace{}).Where("id = 1 AND run_id = ? AND lease <> '' AND lease_until > ?", runID, s.now()).Update("cancelled", true)
	if q.Error != nil {
		return q.Error
	}
	if q.RowsAffected != 1 {
		return conflict("该运行已结束或被替代")
	}
	return nil
}

type operation struct {
	s       *Workspace
	ctx     context.Context
	run     db.JiraIssueRun
	lease   string
	config  WorkspaceConfig
	catalog Catalog
	remote  *remote
	result  RunView
}

func (o *operation) guard() error {
	if err := o.ctx.Err(); err != nil {
		return errors.New("运行已停止或超时，已发出的请求需在下次运行核对")
	}
	var d db.JiraIssueWorkspace
	if err := o.s.DB.WithContext(o.ctx).Select("version", "lease", "lease_until", "cancelled", "run_id").First(&d, 1).Error; err != nil {
		return err
	}
	if d.Cancelled || d.RunID != o.run.ID || d.Lease != o.lease || d.Version != o.run.ConfigVersion || !d.LeaseUntil.After(o.s.now()) {
		return errors.New("运行已停止或配置已变化，已发出的请求将保留记录")
	}
	return nil
}
func (o *operation) progress(message string) error {
	if err := o.guard(); err != nil {
		return err
	}
	o.run.Message = message
	o.run.UpdatedAt = o.s.now()
	return o.s.DB.WithContext(o.ctx).Model(&db.JiraIssueRun{}).Where("id = ?", o.run.ID).Updates(map[string]any{"message": message, "result_json": marshal(o.result), "item_count": len(o.result.Items), "updated_at": o.run.UpdatedAt}).Error
}
func (s *Workspace) Execute(parent context.Context, run db.JiraIssueRun, lease string, repoIDs []string, projectKeys ...[]string) {
	ctx, cancel := context.WithTimeout(parent, 8*time.Minute)
	defer cancel()
	o := &operation{s: s, ctx: ctx, run: run, lease: lease}
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				var active db.JiraIssueWorkspace
				if err := s.DB.WithContext(ctx).Select("run_id", "lease", "lease_until", "cancelled").First(&active, 1).Error; err == nil && (active.Cancelled || active.RunID != run.ID || active.Lease != lease || !active.LeaseUntil.After(s.now())) {
					cancel()
					return
				}
			}
		}
	}()

	err := func() error {
		d, e := s.row(ctx)
		if e != nil {
			return e
		}
		o.config = decodeConfig(d)
		_ = json.Unmarshal([]byte(d.CatalogJSON), &o.catalog)
		sec, e := s.secrets(d)
		if e != nil {
			return e
		}
		o.remote, e = newRemote(o.config, sec, s.Client, o.guard)
		if e != nil {
			return e
		}
		if e = o.guard(); e != nil {
			return e
		}
		switch run.Kind {
		case "discover":
			return o.discover()
		case "analyze":
			var selected []string
			if len(projectKeys) > 0 {
				selected = projectKeys[0]
			}
			return o.analyze(repoIDs, selected)
		case "sync":
			return o.sync()
		}
		return invalid("未知操作")
	}()
	status := "succeeded"
	message := "运行完成"
	if strings.HasPrefix(o.run.Message, "已") || strings.HasPrefix(o.run.Message, "本次") {
		message = o.run.Message
	}
	if err != nil {
		status = "failed"
		message = err.Error()
		if ctx.Err() != nil {
			message = "运行超时或服务停止，请重试核对远端结果"
		}
	}
	if len(message) > 500 {
		message = message[:500]
	}
	// Cleanup has its own bounded context, including after cancellation/server shutdown.
	cleanup, done := context.WithTimeout(context.Background(), 5*time.Second)
	defer done()
	var last db.JiraIssueWorkspace
	if err != nil && s.DB.WithContext(cleanup).Select("run_id", "cancelled").First(&last, 1).Error == nil && last.RunID == run.ID && last.Cancelled {
		status = "cancelled"
		message = "运行已停止，已发出请求将在下次运行核对"
	}
	_ = s.DB.WithContext(cleanup).Model(&db.JiraIssueRun{}).Where("id = ?", run.ID).Updates(map[string]any{"status": status, "message": message, "result_json": marshal(o.result), "item_count": len(o.result.Items), "updated_at": s.now()}).Error
	_ = s.DB.WithContext(cleanup).Model(&db.JiraIssueWorkspace{}).Where("id = 1 AND lease = ?", lease).Updates(map[string]any{"lease": "", "lease_until": s.now(), "cancelled": false, "run_id": 0}).Error
}
func (s *Workspace) TickWorkspace(ctx context.Context) error {
	d, err := s.row(ctx)
	if err != nil {
		return err
	}
	c := decodeConfig(d)
	now := s.now().In(time.FixedZone("Asia/Shanghai", 8*3600))
	if !c.Enabled || now.Format("15:04") < c.TimeOfDay {
		return nil
	}
	key := fmt.Sprintf("%s:%d", now.Format("2006-01-02"), d.Version)
	if d.DailyKey == key {
		return nil
	}
	run, lease, err := s.Begin(ctx, d.Version, "sync")
	if err != nil {
		return err
	}
	q := s.DB.WithContext(ctx).Model(&db.JiraIssueWorkspace{}).Where("id = 1 AND lease = ? AND daily_key <> ?", lease, key).Update("daily_key", key)
	if q.Error != nil || q.RowsAffected != 1 {
		_ = s.DB.WithContext(ctx).Model(&db.JiraIssueRun{}).Where("id = ?", run.ID).Updates(map[string]any{"status": "skipped", "message": "每日运行已登记，未重复执行"}).Error
		_ = s.DB.WithContext(ctx).Model(&db.JiraIssueWorkspace{}).Where("id = 1 AND lease = ?", lease).Updates(map[string]any{"lease": "", "lease_until": s.now(), "run_id": 0}).Error
		return q.Error
	}
	s.Execute(ctx, run, lease, nil)
	return nil
}

func (s *Workspace) RunDetail(ctx context.Context, id uint, offset int) (RunView, error) {
	if id == 0 || offset < 0 || offset > 1000 {
		return RunView{}, invalid("运行或分页参数无效")
	}
	var r db.JiraIssueRun
	if err := s.DB.WithContext(ctx).First(&r, id).Error; err != nil {
		return RunView{}, err
	}
	view := RunView{}
	if r.ResultJSON != "" {
		if err := json.Unmarshal([]byte(r.ResultJSON), &view); err != nil {
			return view, err
		}
	}
	view.JiraIssueRun = r
	view.Proposals = nil
	view.ItemsTotal = len(view.Items)
	if offset > len(view.Items) {
		return view, invalid("分页超出运行结果范围")
	}
	end := offset + 20
	if end > len(view.Items) {
		end = len(view.Items)
	}
	view.Items = view.Items[offset:end]
	if end < view.ItemsTotal {
		view.NextOffset = end
	}
	return view, nil
}
