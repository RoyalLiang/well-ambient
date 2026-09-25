package codereview

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"well-ambient/internal/config"
)

var shaPattern = regexp.MustCompile(`^[a-fA-F0-9]{40}([a-fA-F0-9]{24})?$`)

type GitLab struct {
	Config config.GitLabConfig
	Client *http.Client
}

type gitLabHTTPError struct {
	StatusCode int
}

func (e *gitLabHTTPError) Error() string {
	return fmt.Sprintf("GitLab HTTP %d", e.StatusCode)
}

func (g GitLab) request(ctx context.Context, method, path string, payload any, out any) (http.Header, error) {
	base, err := url.Parse(strings.TrimRight(g.Config.BaseURL, "/"))
	if err != nil || base.Host == "" || base.User != nil || base.RawQuery != "" || base.Fragment != "" || (base.Scheme != "https" && !(base.Scheme == "http" && (base.Hostname() == "127.0.0.1" || base.Hostname() == "localhost"))) {
		return nil, errors.New("GitLab 地址无效")
	}
	var body []byte
	if payload != nil {
		body, err = json.Marshal(payload)
		if err != nil {
			return nil, err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(base.String(), "/")+"/api/v4"+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("PRIVATE-TOKEN", g.Config.APIToken)
	req.Header.Set("Content-Type", "application/json")
	client := http.Client{Timeout: 30 * time.Second}
	if g.Client != nil {
		client = *g.Client
	}
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.New("GitLab 请求失败或结果未知")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &gitLabHTTPError{StatusCode: resp.StatusCode}
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20+1))
	if err != nil {
		return nil, err
	}
	if len(data) > 2<<20 {
		return nil, errors.New("GitLab 响应超出限制")
	}
	if out != nil {
		if err = json.Unmarshal(data, out); err != nil {
			return nil, errors.New("GitLab 响应无法解析")
		}
	}
	return resp.Header, nil
}
func projectPath(id string) string { return "/projects/" + url.PathEscape(id) }

type File struct {
	New       bool   `json:"new_file"`
	OldPath   string `json:"old_path"`
	NewPath   string `json:"new_path"`
	Diff      string `json:"diff"`
	Deleted   bool   `json:"deleted_file"`
	TooLarge  bool   `json:"too_large"`
	Collapsed bool   `json:"collapsed"`
	Source    string `json:"source"`
	Before    string `json:"before"`
}
type Knowledge struct {
	Confidence float64   `json:"confidence"`
	Freshness  float64   `json:"freshness"`
	UpdatedAt  time.Time `json:"updated_at"`
	ID         uint      `json:"id"`
	Version    int       `json:"version"`
	Scope      string    `json:"scope"`
	Source     string    `json:"source"`
	Summary    string    `json:"summary"`
	Content    string    `json:"content"`
	Hash       string    `json:"hash"`
}
type Snapshot struct {
	Author      string      `json:"author"`
	Head        string      `json:"head"`
	Base        string      `json:"base"`
	Title       string      `json:"title"`
	Description string      `json:"description"`
	URL         string      `json:"url"`
	Files       []File      `json:"files"`
	Knowledge   []Knowledge `json:"knowledge"`
	Complete    bool        `json:"complete"`
	Gaps        []string    `json:"gaps"`
}
type mr struct {
	IID         int    `json:"iid"`
	Author struct {
		Name     string `json:"name"`
		Username string `json:"username"`
	} `json:"author"`
	SHA         string `json:"sha"`
	State       string `json:"state"`
	Title       string `json:"title"`
	Description string `json:"description"`
	URL         string `json:"web_url"`
	UpdatedAt   string `json:"updated_at"`
	DiffRefs    struct {
		Base string `json:"base_sha"`
		Head string `json:"head_sha"`
	} `json:"diff_refs"`
}

func (g GitLab) mr(ctx context.Context, project, ref string) (mr, error) {
	var x mr
	_, err := g.request(ctx, http.MethodGet, projectPath(project)+"/merge_requests/"+ref, nil, &x)
	return x, err
}

func (g GitLab) OpenMRsForBranch(ctx context.Context, project, branch string) ([]mr, error) {
	if branch == "" {
		return nil, nil
	}
	var list []mr
	path := projectPath(project) + "/merge_requests?source_branch=" + url.QueryEscape(branch) + "&state=opened&per_page=5"
	_, err := g.request(ctx, http.MethodGet, path, nil, &list)
	return list, err
}
func (g GitLab) Snapshot(ctx context.Context, project, kind, ref string) (Snapshot, error) {
	s := Snapshot{Complete: true, Files: []File{}, Knowledge: []Knowledge{}, Gaps: []string{}}
	path := projectPath(project)
	if kind == "mr" {
		m, err := g.mr(ctx, project, ref)
		if err != nil {
			return s, err
		}
		s.Head = m.DiffRefs.Head
		s.Base = m.DiffRefs.Base
		s.Author = m.Author.Name
		if s.Author == "" {
			s.Author = m.Author.Username
		}
		s.Title = m.Title
		s.Description = m.Description
		s.URL = m.URL
		path += "/merge_requests/" + ref + "/diffs"
	} else {
		var c struct {
			AuthorName string   `json:"author_name"`
			ID         string   `json:"id"`
			Title      string   `json:"title"`
			Message    string   `json:"message"`
			URL        string   `json:"web_url"`
			Parents    []string `json:"parent_ids"`
		}
		_, err := g.request(ctx, http.MethodGet, path+"/repository/commits/"+ref, nil, &c)
		if err != nil {
			return s, err
		}
		if !strings.EqualFold(c.ID, ref) {
			return s, errors.New("Commit SHA 不匹配")
		}
		s.Head = c.ID
		s.Author = c.AuthorName
		s.Title = c.Title
		s.Description = c.Message
		s.URL = c.URL
		if len(c.Parents) > 0 {
			s.Base = c.Parents[0]
		}
		if len(c.Parents) > 1 {
			s.Gaps = append(s.Gaps, "合并提交仅与第一父提交比较；请优先评审对应 MR")
		}
		path += "/repository/commits/" + ref + "/diff"
	}
	if (kind == "mr" && s.Base == "") || !shaPattern.MatchString(s.Head) || (s.Base != "" && !shaPattern.MatchString(s.Base)) {
		return s, errors.New("代码版本不完整，无法冻结评审")
	}
	bytesUsed := 0
	for page := 1; page <= 20; page++ {
		var files []File
		headers, err := g.request(ctx, http.MethodGet, path+"?per_page=100&page="+strconv.Itoa(page), nil, &files)
		if err != nil {
			return s, err
		}
		for _, f := range files {
			if len(s.Files) >= 24 || bytesUsed+len(f.Diff) > 160000 {
				s.Complete = false
				s.Gaps = append(s.Gaps, "变更超出本次文件或上下文预算；部分文件未评审")
				break
			}
			if f.TooLarge || f.Collapsed || f.Diff == "" {
				s.Complete = false
				s.Gaps = append(s.Gaps, "diff 不完整或非文本文件："+f.NewPath)
			}
			if !f.Deleted {
				f.Source, err = g.source(ctx, project, f.NewPath, s.Head)
				if err != nil {
					s.Complete = false
					s.Gaps = append(s.Gaps, "无法读取当前源码："+f.NewPath)
				}
			}
			if s.Base != "" && f.OldPath != "" {
				if !f.New {
					f.Before, err = g.source(ctx, project, f.OldPath, s.Base)
					if err != nil {
						s.Complete = false
						s.Gaps = append(s.Gaps, "无法读取变更前源码："+f.OldPath)
					}
				}
			}
			if bytesUsed+len(f.Source)+len(f.Before)+len(f.Diff) > 160000 {
				f.Source = ""
				f.Before = ""
				s.Complete = false
				s.Gaps = append(s.Gaps, "完整源码超出上下文预算："+f.NewPath)
			}
			bytesUsed += len(f.Source) + len(f.Before) + len(f.Diff)
			s.Files = append(s.Files, f)
		}
		if len(s.Files) >= 24 || bytesUsed >= 160000 {
			if headers.Get("X-Next-Page") != "" {
				s.Complete = false
				s.Gaps = append(s.Gaps, "预算限制：后续 diff 分页未评审")
			}
			break
		}
		next := headers.Get("X-Next-Page")
		if next == "" {
			if len(files) >= 100 {
				s.Complete = false
				s.Gaps = append(s.Gaps, "分页完整性无法确认")
			}
			break
		}
		if next != strconv.Itoa(page+1) {
			return s, errors.New("GitLab diff 分页不完整")
		}
		if page == 20 {
			s.Complete = false
			s.Gaps = append(s.Gaps, "diff 分页达到限制")
		}
	}
	if len(s.Files) == 0 {
		s.Complete = false
		s.Gaps = append(s.Gaps, "没有可读取的代码差异")
	}
	if kind == "mr" {
		m, err := g.mr(ctx, project, ref)
		if err != nil {
			return s, err
		}
		if m.DiffRefs.Head != s.Head || m.DiffRefs.Base != s.Base {
			return s, errors.New("读取期间 MR 版本发生变化，请重新评审")
		}
	}
	return s, nil
}
func (g GitLab) source(ctx context.Context, project, path, sha string) (string, error) {
	var file struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	_, err := g.request(ctx, http.MethodGet, projectPath(project)+"/repository/files/"+url.PathEscape(path)+"?ref="+url.QueryEscape(sha), nil, &file)
	if err != nil {
		return "", err
	}
	if file.Encoding != "base64" {
		return "", errors.New("未知源码编码")
	}
	return decodeSource(file.Content)
}
