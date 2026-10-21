package jiraissuesync

import (
	"errors"
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
	"well-ambient/internal/db"
)

type RepoEvidence struct {
	RepoID      string    `json:"repo_id"`
	Repository  string    `json:"repository"`
	IssueURL    string    `json:"issue_url,omitempty"`
	MRURL       string    `json:"mr_url,omitempty"`
	SHA         string    `json:"sha,omitempty"`
	PipelineURL string    `json:"pipeline_url,omitempty"`
	PipelineID  int64     `json:"pipeline_id,omitempty"`
	Merged      bool      `json:"merged"`
	Verified    bool      `json:"verified"`
	Message     string    `json:"message"`
	CollectedAt time.Time `json:"collected_at"`
}
type SyncItem struct {
	JiraKey      string         `json:"jira_key"`
	JiraURL      string         `json:"jira_url"`
	Repositories []RepoEvidence `json:"repositories"`
	Commented    bool           `json:"commented"`
	Closed       bool           `json:"closed"`
	Message      string         `json:"message"`
}
type gitIssue struct {
	ID          int64  `json:"id"`
	IID         int64  `json:"iid"`
	ProjectID   int64  `json:"project_id"`
	Description string `json:"description"`
	WebURL      string `json:"web_url"`
	State       string `json:"state"`
}
type mergeRequest struct {
	IID             int64      `json:"iid"`
	ProjectID       int64      `json:"project_id"`
	State           string     `json:"state"`
	TargetBranch    string     `json:"target_branch"`
	MergeCommitSHA  string     `json:"merge_commit_sha"`
	SquashCommitSHA string     `json:"squash_commit_sha"`
	WebURL          string     `json:"web_url"`
	MergedAt        *time.Time `json:"merged_at"`
}
type pipeline struct {
	Source string `json:"source"`
	ID     int64  `json:"id"`
	SHA    string `json:"sha"`
	Ref    string `json:"ref"`
	Status string `json:"status"`
	WebURL string `json:"web_url"`
}

func (o *operation) sync() error {
	projects := []string{}
	for _, m := range o.config.Mappings {
		if !hasProject(o.catalog, m.Project) {
			return invalid("项目目录已变化，请重新读取并修正映射")
		}
		projects = append(projects, m.Project)
	}
	var pending []string
	if err := o.s.DB.WithContext(o.ctx).Model(&db.JiraIssueLink{}).Distinct("jira_key").Where("jira_origin = ? AND git_lab_origin = ? AND project_key IN ? AND closed = ?", o.remote.jira, o.remote.gitlab, projects, false).Limit(1001).Pluck("jira_key", &pending).Error; err != nil {
		return err
	}
	if len(pending) > 1000 {
		return errors.New("待核对任务超过 1000 个，请缩小项目范围")
	}
	predicates := []string{}
	for _, m := range o.config.Mappings {
		term := "project = " + m.Project
		if len(m.TaskKeys) > 0 {
			term += " AND key in (" + strings.Join(m.TaskKeys, ",") + ")"
		} else {
			term += " AND statusCategory != Done"
		}
		predicates = append(predicates, "("+term+")")
	}
	// Pending known identities survive a lost transition acknowledgement even after
	// Jira moves to Done and disappears from the ordinary active-issue search.
	eligiblePending := []string{}
	for _, key := range pending {
		if !taskPattern.MatchString(key) {
			continue
		}
		for _, m := range o.config.Mappings {
			if strings.HasPrefix(key, m.Project+"-") && (len(m.TaskKeys) == 0 || contains(m.TaskKeys, key)) {
				eligiblePending = append(eligiblePending, key)
				break
			}
		}
	}
	if len(eligiblePending) > 0 {
		predicates = append(predicates, "key in ("+strings.Join(eligiblePending, ",")+")")
	}
	tasks, err := o.remote.tasks(o.ctx, projects, strings.Join(predicates, " OR "))
	if err != nil {
		return err
	}

	failures := 0
	for _, task := range tasks {
		ids := []string{}
		for _, m := range o.config.Mappings {
			if strings.HasPrefix(task.Key, m.Project+"-") && (len(m.TaskKeys) == 0 || contains(m.TaskKeys, task.Key)) {
				ids = append(ids, m.RepoIDs...)
			}
		}
		sort.Strings(ids)
		ids = unique(ids)
		if len(ids) == 0 {
			continue
		}
		if err = o.progress("正在同步 " + task.Key); err != nil {
			return err
		}
		item := SyncItem{JiraKey: task.Key, JiraURL: o.remote.jira + "/browse/" + url.PathEscape(task.Key), Repositories: []RepoEvidence{}}
		if task.Fields.Status.Category.Key == "done" && !contains(eligiblePending, task.Key) {
			item.Closed = true
			item.Message = "Jira 已完成，本次无需创建或关闭"
			o.result.Items = append(o.result.Items, item)
			continue
		}
		if task.Fields.Status.Category.Key != "done" {
			if err = o.s.DB.WithContext(o.ctx).Model(&db.JiraIssueLink{}).Where("jira_key = ? AND jira_origin = ? AND git_lab_origin = ?", task.Key, o.remote.jira, o.remote.gitlab).Update("closed", false).Error; err != nil {
				return err
			}
		}
		complete := true
		for _, id := range ids {
			repo, ok := findRepo(o.catalog, id)
			if !ok {
				return invalid("映射仓库不在当前群组，请重新读取")
			}
			evidence, e := o.syncRepo(task, repo)
			item.Repositories = append(item.Repositories, evidence)
			if e != nil {
				complete = false
				failures++
			} else if !evidence.Verified {
				complete = false
			}
		}
		if complete {
			if err = o.closeJira(&item); err != nil {
				failures++
				item.Message = err.Error()
			}
		} else {
			item.Message = "等待所有关联仓库合并并通过对应提交的验证"
		}
		o.result.Items = append(o.result.Items, item)
		if err = o.progress("已核对 " + task.Key); err != nil {
			return err
		}
	}
	if len(o.result.Items) == 0 {
		o.run.Message = "本次无待处理任务"
		return nil
	}
	if failures > 0 {
		return fmt.Errorf("%d 项操作未完成，请查看逐项证据后重试", failures)
	}
	return nil
}
func contains(items []string, value string) bool {
	for _, v := range items {
		if v == value {
			return true
		}
	}
	return false
}
func (o *operation) syncRepo(task jiraTask, repo Repo) (result RepoEvidence, err error) {
	id := fmt.Sprint(repo.ID)
	result = RepoEvidence{RepoID: id, Repository: repo.Path, CollectedAt: o.s.now()}
	defer func() {
		if err != nil {
			result.Message = err.Error()
		}
	}()
	// Verify live namespace/default branch before sending any repository write.
	var live Repo
	if _, err = o.remote.request(o.ctx, false, "GET", "/projects/"+id, nil, &live); err != nil {
		return
	}
	if live.ID != repo.ID || live.Archived || !live.IssuesEnabled || !strings.HasPrefix(live.Path, o.remote.group+"/") || live.DefaultBranch == "" {
		err = errors.New("仓库范围、权限或默认分支已变化")
		return
	}
	marker := "<!-- well-ambient:jira-issue:" + digest(o.remote.jira+"|"+task.Key+"|"+o.remote.gitlab+"|"+id) + " -->"
	end := "<!-- /well-ambient:jira-issue -->"
	scopeKey := digest(o.remote.jira + "|" + task.Key + "|" + o.remote.gitlab + "|" + id)
	var known db.JiraIssueLink
	lookup := o.s.DB.WithContext(o.ctx).Where("scope_key = ?", scopeKey).First(&known).Error
	matches := []gitIssue{}
	if lookup == nil {
		var linked gitIssue
		if _, err = o.remote.request(o.ctx, false, "GET", "/projects/"+id+"/issues/"+fmt.Sprint(known.IssueIID), nil, &linked); err != nil {
			return
		}
		if linked.ID != known.IssueID || linked.ProjectID != repo.ID || !strings.Contains(linked.Description, marker) {
			err = errors.New("已关联 Issue 被删除、移动或移除了同步标识，未创建替代项")
			return
		}
		matches = append(matches, linked)
	} else if errors.Is(lookup, gorm.ErrRecordNotFound) {
		issues, e := glList[gitIssue](o.ctx, o.remote, "/projects/"+id+"/issues?scope=all&state=all&search="+url.QueryEscape(scopeKey), 1000)
		if e != nil {
			err = e
			return
		}
		for _, candidate := range issues {
			if candidate.ProjectID == repo.ID && strings.Contains(candidate.Description, marker) {
				matches = append(matches, candidate)
			}
		}
	} else {
		err = lookup
		return
	}
	if len(matches) > 1 {
		err = errors.New("存在多个相同同步标识的 Issue，需先处理重复关联")
		return
	}

	block := marker + "\nJira: " + o.remote.jira + "/browse/" + task.Key + "\n状态: " + task.Fields.Status.Name + "\n\n" + managedDescription(workspaceClip(task.Fields.Description, 12000)) + "\n" + end
	title := workspaceClip("["+task.Key+"] "+task.Fields.Summary, 250)
	var issue gitIssue
	if len(matches) == 0 {
		_, err = o.remote.request(o.ctx, false, "POST", "/projects/"+id+"/issues", map[string]any{"title": title, "description": block}, &issue)
		if err != nil {
			return
		}
		if issue.IID <= 0 || issue.ID <= 0 {
			err = errors.New("Issue 创建结果缺少标识，请重试核对")
			return
		}
	} else {
		issue = matches[0]
		start := strings.Index(issue.Description, marker)
		suffix := strings.Index(issue.Description[start:], end)
		if suffix < 0 {
			err = errors.New("Issue 同步区域标识不完整，请人工核对")
			return
		}
		description := issue.Description[:start] + block + issue.Description[start+suffix+len(end):]
		if description != issue.Description {
			_, err = o.remote.request(o.ctx, false, "PUT", "/projects/"+id+"/issues/"+fmt.Sprint(issue.IID), map[string]any{"description": description}, nil)
			if err != nil {
				return
			}
		}
	}
	result.IssueURL = evidenceURL(issue.WebURL, o.remote.gitlab)
	// Read back created/updated issue, including after successful acknowledgement.
	var confirmed gitIssue
	if _, err = o.remote.request(o.ctx, false, "GET", "/projects/"+id+"/issues/"+fmt.Sprint(issue.IID), nil, &confirmed); err != nil {
		return
	}
	if confirmed.ID != issue.ID || confirmed.ProjectID != repo.ID || !strings.Contains(confirmed.Description, marker) {
		err = errors.New("Issue 关联回读不一致")
		return
	}
	link := db.JiraIssueLink{ScopeKey: scopeKey, IssueID: confirmed.ID, IssueIID: confirmed.IID, JiraKey: task.Key, JiraOrigin: o.remote.jira, GitLabOrigin: o.remote.gitlab, ProjectKey: strings.SplitN(task.Key, "-", 2)[0]}
	if err = o.s.DB.WithContext(o.ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&link).Error; err != nil {
		return
	}
	var persisted db.JiraIssueLink
	if err = o.s.DB.WithContext(o.ctx).Where("scope_key = ?", scopeKey).First(&persisted).Error; err != nil {
		return
	}
	if persisted.IssueID != confirmed.ID || persisted.IssueIID != confirmed.IID {
		err = errors.New("Issue 关联身份冲突，已停止回写")
		return
	}
	closing, e := glList[mergeRequest](o.ctx, o.remote, "/projects/"+id+"/issues/"+fmt.Sprint(issue.IID)+"/closed_by", 100)
	if e != nil {
		err = e
		return
	}
	if len(closing) == 0 {
		result.Message = fmt.Sprintf("等待关闭该 Issue 的 MR 合并（Closes #%d）", issue.IID)
		return
	}
	// More than one merged closing MR is ambiguous: do not cherry-pick convenient evidence.
	candidates := []mergeRequest{}
	for _, mr := range closing {
		if mr.State == "merged" && mr.ProjectID == repo.ID {
			candidates = append(candidates, mr)
		}
	}
	if len(candidates) != 1 {
		result.Message = "需有且仅有一个已合并的关闭 MR"
		return
	}
	var mr mergeRequest
	if _, err = o.remote.request(o.ctx, false, "GET", "/projects/"+id+"/merge_requests/"+fmt.Sprint(candidates[0].IID), nil, &mr); err != nil {
		return
	}
	if mr.State != "merged" || mr.MergedAt == nil || mr.TargetBranch != live.DefaultBranch || mr.ProjectID != repo.ID {
		result.Message = "MR 未合并到当前默认分支"
		return
	}
	// Verify the MR explicitly closes this exact Issue, not merely a matching Jira title.
	closed, e := glList[gitIssue](o.ctx, o.remote, "/projects/"+id+"/merge_requests/"+fmt.Sprint(mr.IID)+"/closes_issues", 100)
	if e != nil {
		err = e
		return
	}
	found := false
	for _, v := range closed {
		if v.ID == issue.ID && v.ProjectID == repo.ID {
			found = true
		}
	}
	if !found {
		result.Message = "MR 缺少该 Issue 的关闭关联"
		return
	}
	sha := mr.MergeCommitSHA
	if sha == "" {
		sha = mr.SquashCommitSHA
	}
	if len(sha) != 40 && len(sha) != 64 {
		result.Message = "缺少完整合并提交 SHA"
		return
	}
	result.Merged = true
	result.MRURL = evidenceURL(mr.WebURL, o.remote.gitlab)
	result.SHA = sha
	var pipelines []pipeline
	_, err = o.remote.request(o.ctx, false, "GET", "/projects/"+id+"/pipelines?sha="+url.QueryEscape(sha)+"&ref="+url.QueryEscape(live.DefaultBranch)+"&source=push&order_by=id&sort=desc&per_page=1", nil, &pipelines)
	if err != nil {
		return
	}
	if len(pipelines) == 0 {
		result.Message = "等待合并提交在默认分支的验证"
		return
	}
	p := pipelines[0]
	result.PipelineURL = evidenceURL(p.WebURL, o.remote.gitlab)
	result.PipelineID = p.ID
	if p.Source != "push" || p.SHA != sha || p.Ref != live.DefaultBranch || p.Status != "success" || p.ID <= 0 {
		result.Message = "最近验证未通过或 SHA 不匹配"
		return
	}
	result.Verified = true
	result.Message = "已合并 · 默认分支 Push 验证通过"
	return
}
func (o *operation) hasComment(key, marker string, evidenceRows []string) (bool, error) {
	for start := 0; start < 2000; {
		var page struct {
			StartAt  int `json:"startAt"`
			Total    int `json:"total"`
			Comments []struct {
				Body string `json:"body"`
			} `json:"comments"`
		}
		_, err := o.remote.request(o.ctx, true, "GET", "/issue/"+url.PathEscape(key)+"/comment?maxResults=100&startAt="+strconv.Itoa(start), nil, &page)
		if err != nil {
			return false, err
		}
		if page.StartAt != start || page.Total > 2000 {
			return false, errors.New("Jira 评论分页不完整或超过 2000 条，未执行重复写入")
		}
		for _, c := range page.Comments {
			if strings.Contains(c.Body, marker) {
				for _, required := range evidenceRows {
					if !strings.Contains(c.Body, required) {
						return false, errors.New("同标识评论的证据内容已变化，未关闭 Jira")
					}
				}
				return true, nil
			}
		}
		start += len(page.Comments)
		if start >= page.Total {
			return false, nil
		}
		if len(page.Comments) == 0 {
			return false, errors.New("Jira 评论分页中断")
		}
	}
	return false, errors.New("Jira 评论读取超出上限")
}
func (o *operation) closeJira(item *SyncItem) error {
	rows := []string{}
	for _, e := range item.Repositories {
		if !e.Merged || !e.Verified || e.SHA == "" || e.PipelineURL == "" || e.MRURL == "" {
			return errors.New("关闭证据不完整")
		}
		rows = append(rows, fmt.Sprintf("仓库 %s\nIssue: %s\n已合并 MR: %s\n合并提交: %s\n通过验证: %s (pipeline %d, SHA %s)", e.Repository, e.IssueURL, e.MRURL, e.SHA, e.PipelineURL, e.PipelineID, e.SHA))
	}
	codeRows := []string{}
	for _, e := range item.Repositories {
		codeRows = append(codeRows, e.RepoID+"|"+e.SHA)
	}
	sort.Strings(codeRows)
	codeHash := digest(strings.Join(codeRows, "\n"))
	var issue jiraTask
	if _, err := o.remote.request(o.ctx, true, "GET", "/issue/"+url.PathEscape(item.JiraKey)+"?fields=status", nil, &issue); err != nil {
		return err
	}
	var previous db.JiraIssueLink
	lookup := o.s.DB.WithContext(o.ctx).Where("jira_key = ? AND jira_origin = ? AND git_lab_origin = ? AND closed_code_hash <> ''", item.JiraKey, o.remote.jira, o.remote.gitlab).First(&previous).Error
	if lookup != nil && !errors.Is(lookup, gorm.ErrRecordNotFound) {
		return lookup
	}
	if issue.Fields.Status.Category.Key != "done" && previous.ClosedCodeHash == codeHash {
		return errors.New("Jira 已重新打开；等待新的合并提交与验证证据，未复用旧证据关闭")
	}
	sort.Strings(rows)
	marker := "[well-ambient:verified:" + digest(item.JiraKey+"|"+strings.Join(rows, "\n")) + "]"
	exists, err := o.hasComment(item.JiraKey, marker, rows)
	if err != nil {
		return err
	}
	if !exists {
		body := "代码合并与验证证据\n\n" + strings.Join(rows, "\n\n") + "\n\n采集时间: " + o.s.now().Format(time.RFC3339) + "\n" + marker
		if _, err = o.remote.request(o.ctx, true, "POST", "/issue/"+url.PathEscape(item.JiraKey)+"/comment", map[string]string{"body": body}, nil); err != nil {
			return fmt.Errorf("证据评论未确认：%w", err)
		}
		exists, err = o.hasComment(item.JiraKey, marker, rows)
		if err != nil {
			return err
		}
		if !exists {
			return errors.New("证据评论回读未确认，未关闭 Jira")
		}
	}
	item.Commented = true
	if err = o.progress("证据已评论 · " + item.JiraKey); err != nil {
		return err
	}
	if _, err = o.remote.request(o.ctx, true, "GET", "/issue/"+url.PathEscape(item.JiraKey)+"?fields=status", nil, &issue); err != nil {
		return err
	}
	if issue.Fields.Status.Category.Key == "done" {
		item.Closed = true
		if err = o.markJiraClosed(item.JiraKey, codeHash); err != nil {
			return err
		}
		item.Message = "证据已评论 · Jira 已关闭"
		return nil
	}
	var transitions struct {
		Transitions []struct {
			ID string `json:"id"`
			To struct {
				Category struct {
					Key string `json:"key"`
				} `json:"statusCategory"`
			} `json:"to"`
		} `json:"transitions"`
	}
	if _, err = o.remote.request(o.ctx, true, "GET", "/issue/"+url.PathEscape(item.JiraKey)+"/transitions", nil, &transitions); err != nil {
		return err
	}
	ids := []string{}
	for _, t := range transitions.Transitions {
		if t.To.Category.Key == "done" && t.ID != "" {
			ids = append(ids, t.ID)
		}
	}
	if len(ids) != 1 {
		return errors.New("证据已评论；无法唯一确定 Jira 完成状态流转，请检查工作流")
	}
	_, transitionErr := o.remote.request(o.ctx, true, "POST", "/issue/"+url.PathEscape(item.JiraKey)+"/transitions", map[string]any{"transition": map[string]string{"id": ids[0]}}, nil)
	// A timed-out POST may already have succeeded; read back before reporting/retrying.
	if _, err = o.remote.request(o.ctx, true, "GET", "/issue/"+url.PathEscape(item.JiraKey)+"?fields=status", nil, &issue); err != nil {
		return errors.New("证据已评论；Jira 关闭结果未确认，下次运行将核对")
	}
	if issue.Fields.Status.Category.Key != "done" {
		if transitionErr != nil {
			return errors.New("证据已评论；Jira 关闭请求未确认")
		}
		return errors.New("证据已评论；Jira 回读尚未完成")
	}
	item.Closed = true
	if err = o.markJiraClosed(item.JiraKey, codeHash); err != nil {
		return err
	}
	item.Message = "证据已评论 · Jira 已关闭"
	return nil
}

func managedDescription(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(value, "<!-- /well-ambient:jira-issue -->", "&lt;!-- /well-ambient:jira-issue -->"), "<!-- well-ambient:jira-issue:", "&lt;!-- well-ambient:jira-issue:")
}
func (o *operation) markJiraClosed(key string, codeHash string) error {
	return o.s.DB.WithContext(o.ctx).Model(&db.JiraIssueLink{}).Where("jira_key = ? AND jira_origin = ? AND git_lab_origin = ?", key, o.remote.jira, o.remote.gitlab).Updates(map[string]any{"closed": true, "closed_code_hash": codeHash}).Error
}

func evidenceURL(value, origin string) string {
	if len(value) > 1024 {
		return ""
	}
	u, err := url.Parse(value)
	if err != nil || u.User != nil || u.Scheme+"://"+u.Host != origin {
		return ""
	}
	return value
}
