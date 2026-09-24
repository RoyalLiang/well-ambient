// Package confluence synchronizes a report into a direct child of a configured
// Confluence Server/Data Center page using a personal access token.
package confluence

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
)

type Config struct {
	ParentPageURL string
	Token         string
}

type Attachment struct {
	Filename    string
	Data        []byte
	ContentType string
}

type Page struct {
	ID    string
	URL   string
	Title string
}

// Client permits an injected transport for tests. Each operation copies the
// HTTP client and disables redirects and cookies without mutating the caller.
// The zero value is ready to use.
type Client struct {
	HTTPClient *http.Client
}

const (
	requestTimeout = 30 * time.Second
	syncTimeout    = 2 * time.Minute
	maxResponse    = 2 << 20
	maxStorage     = 8 << 20
	maxAttachment  = 16 << 20
	maxAttachments = 64
	maxTotalData   = 64 << 20
	placeholder    = "<p>Report synchronization in progress.</p>"
)

type safeError struct {
	message string
	cause   error
	status  int
}

func (e *safeError) Error() string { return e.message }
func (e *safeError) Unwrap() error { return e.cause }

// ErrorMessage exposes only diagnostics created by this client. Unknown errors
// may contain tokens, request URLs or response bodies and must not be surfaced.
func ErrorMessage(err error) string {
	var safe *safeError
	if errors.As(err, &safe) {
		return safe.message
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "Confluence 请求超时"
	}
	if errors.Is(err, context.Canceled) {
		return "Confluence 请求已取消"
	}
	return "Confluence 同步失败，请检查连接、令牌和页面写入权限后重试"
}

func fail(message string) error { return &safeError{message: message} }

func transportError(ctx context.Context, err error) error {
	if errors.Is(ctx.Err(), context.Canceled) || errors.Is(err, context.Canceled) {
		return &safeError{message: "Confluence 请求已取消", cause: context.Canceled}
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
		return &safeError{message: "Confluence 请求超时", cause: context.DeadlineExceeded}
	}
	return fail("Confluence 请求失败，请检查网络或服务配置")
}

func statusError(code int) error {
	message := "Confluence 服务请求失败"
	switch code {
	case http.StatusBadRequest:
		message = "Confluence 拒绝请求内容，请检查页面格式与参数"
	case http.StatusRequestEntityTooLarge:
		message = "Confluence 请求内容或附件超过服务限制"
	case http.StatusTooManyRequests:
		message = "Confluence 请求受到限流，请稍后重试"
	case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		message = "Confluence 服务或网关暂时不可用，请稍后重试"
	case http.StatusUnauthorized:
		message = "Confluence 认证失败，请检查访问令牌"
	case http.StatusForbidden:
		message = "Confluence 拒绝访问，请检查权限"
	case http.StatusNotFound:
		message = "Confluence 页面不存在或不可访问"
	case http.StatusConflict:
		message = "Confluence 页面存在并发修改，请稍后重试"
	default:
		if code >= 300 && code < 400 {
			message = "Confluence 不允许重定向，请检查父页地址"
		}
	}
	return &safeError{message: message + "（HTTP " + strconv.Itoa(code) + "）", status: code}
}

func hasStatus(err error, statuses ...int) bool {
	var e *safeError
	if errors.As(err, &e) {
		for _, status := range statuses {
			if e.status == status {
				return true
			}
		}
	}
	return false
}

type location struct {
	base  string
	id    string
	space string
	title string
}

func invalidConfig() error { return fail("Confluence 父页地址或访问令牌无效") }

func numericID(s string) bool {
	if s == "" || s[0] == '0' {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	_, err := strconv.ParseUint(s, 10, 64)
	return err == nil
}

func control(s string) bool { return strings.IndexFunc(s, unicode.IsControl) >= 0 }

// Validate checks the URL and token without sending a network request.
// Production URLs require HTTPS; HTTP is accepted only for loopback testing.
func Validate(cfg Config) error {
	_, err := parseConfig(cfg)
	return err
}

func parseConfig(cfg Config) (location, error) {
	bad := func() (location, error) { return location{}, invalidConfig() }
	if cfg.Token == "" || cfg.Token == "__configured__" || strings.TrimSpace(cfg.Token) != cfg.Token || control(cfg.Token) ||
		strings.ContainsAny(cfg.Token, " \t") || len(cfg.Token) > 64<<10 {
		return bad()
	}
	raw := cfg.ParentPageURL
	if raw == "" || strings.TrimSpace(raw) != raw || control(raw) || strings.ContainsAny(raw, "\\#") {
		return bad()
	}
	u, err := url.Parse(raw)
	if err != nil || u.Opaque != "" || u.User != nil || u.Host == "" || u.Fragment != "" {
		return bad()
	}
	host := u.Hostname()
	ip := net.ParseIP(host)
	if host == "" || strings.ContainsAny(host, "% \t") {
		return bad()
	}
	if ip == nil {
		for _, label := range strings.Split(host, ".") {
			if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
				return bad()
			}
			for _, r := range label {
				if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-') {
					return bad()
				}
			}
		}
	}
	if u.Scheme != "https" && !(u.Scheme == "http" && (strings.EqualFold(host, "localhost") || ip != nil && ip.IsLoopback())) {
		return bad()
	}
	if strings.HasSuffix(u.Host, ":") {
		return bad()
	}
	if port := u.Port(); port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return bad()
		}
	}
	escaped := strings.Split(strings.TrimPrefix(u.EscapedPath(), "/"), "/")
	parts := make([]string, len(escaped))
	for i, part := range escaped {
		decoded, err := url.PathUnescape(part)
		if err != nil || decoded == "" || decoded == "." || decoded == ".." ||
			strings.ContainsAny(decoded, "/\\") || control(decoded) {
			return bad()
		}
		parts[i] = decoded
	}
	query, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return bad()
	}
	loc := location{}
	n := len(parts)
	prefix := -1
	switch {
	case n >= 3 && parts[n-3] == "display":
		prefix = n - 3
		loc.space = parts[n-2]
		// Confluence's display route uses form-style '+' for spaces.
		loc.title, err = url.QueryUnescape(escaped[n-1])
		if err != nil || strings.TrimSpace(loc.title) == "" || control(loc.title) {
			return bad()
		}
	case n >= 2 && parts[n-2] == "pages" && parts[n-1] == "viewpage.action":
		prefix = n - 2
		values := query["pageId"]
		if len(values) != 1 || !numericID(values[0]) {
			return bad()
		}
		loc.id = values[0]
	case n >= 5 && parts[n-5] == "spaces" && parts[n-3] == "pages" && numericID(parts[n-2]):
		prefix = n - 5
		loc.id = parts[n-2]
	default:
		return bad()
	}
	base := &url.URL{Scheme: u.Scheme, Host: u.Host}
	if prefix > 0 {
		base.Path = "/" + strings.Join(parts[:prefix], "/")
	}
	loc.base = base.String()
	return loc, nil
}

type content struct {
	ID     string `json:"id"`
	Type   string `json:"type"`
	Status string `json:"status"`
	Title  string `json:"title"`
	Space  struct {
		Key string `json:"key"`
	} `json:"space"`
	Version struct {
		Number int `json:"number"`
	} `json:"version"`
	Ancestors []struct {
		ID string `json:"id"`
	} `json:"ancestors"`
}

type results struct {
	Results []content `json:"results"`
	Links   struct {
		Next string `json:"next"`
	} `json:"_links"`
}

type session struct {
	loc   location
	token string
	http  http.Client
}

func (c Client) session(cfg Config) (*session, error) {
	loc, err := parseConfig(cfg)
	if err != nil {
		return nil, err
	}
	s := &session{loc: loc, token: cfg.Token}
	if c.HTTPClient != nil {
		s.http = *c.HTTPClient
	}
	if s.http.Timeout <= 0 || s.http.Timeout > requestTimeout {
		s.http.Timeout = requestTimeout
	}
	s.http.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	s.http.Jar = nil
	return s, nil
}

func (s *session) request(ctx context.Context, method, path string, query url.Values, body []byte, contentType string, out any) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	endpoint := s.loc.base + "/rest/api" + path
	if len(query) != 0 {
		endpoint += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
	if err != nil {
		return fail("Confluence 请求配置无效")
	}
	req.Header.Set("Authorization", "Bearer "+s.token)
	req.Header.Set("Accept", "application/json")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if strings.HasPrefix(contentType, "multipart/") {
		req.Header.Set("X-Atlassian-Token", "no-check")
	}
	resp, err := s.http.Do(req)
	if err != nil {
		return transportError(ctx, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return statusError(resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponse+1))
	if err != nil {
		return transportError(ctx, err)
	}
	if len(data) > maxResponse {
		return fail("Confluence 响应过大")
	}
	if out != nil && json.Unmarshal(data, out) != nil {
		return fail("Confluence 返回了无效响应")
	}
	return nil
}

func (s *session) json(ctx context.Context, method, path string, body any, out any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return fail("Confluence 请求内容无效")
	}
	return s.request(ctx, method, path, nil, data, "application/json", out)
}

func (s *session) page(c content) Page {
	return Page{ID: c.ID, Title: c.Title, URL: s.loc.base + "/pages/viewpage.action?pageId=" + c.ID}
}

func validPage(c content) bool {
	return numericID(c.ID) && c.Type == "page" && c.Status == "current" && c.Title != "" && c.Space.Key != ""
}

func (s *session) byID(ctx context.Context, id string) (content, error) {
	var c content
	err := s.request(ctx, http.MethodGet, "/content/"+id, url.Values{"expand": {"space,ancestors,version"}}, nil, "", &c)
	if err != nil {
		return c, err
	}
	if !validPage(c) || c.ID != id {
		return c, fail("Confluence 页面信息无效")
	}
	return c, nil
}

func (s *session) byTitle(ctx context.Context, space, title string) ([]content, error) {
	var list results
	err := s.request(ctx, http.MethodGet, "/content", url.Values{
		"spaceKey": {space}, "title": {title}, "type": {"page"}, "status": {"current"},
		"expand": {"space,ancestors,version"}, "limit": {"2"},
	}, nil, "", &list)
	if err != nil {
		return nil, err
	}
	if len(list.Results) > 1 || list.Links.Next != "" {
		return nil, fail("Confluence 存在同名页面冲突")
	}
	for _, c := range list.Results {
		if !validPage(c) || c.Space.Key != space || c.Title != title {
			return nil, fail("Confluence 页面信息不匹配")
		}
	}
	return list.Results, nil
}

func (s *session) parent(ctx context.Context) (content, error) {
	if s.loc.id != "" {
		return s.byID(ctx, s.loc.id)
	}
	list, err := s.byTitle(ctx, s.loc.space, s.loc.title)
	if err != nil {
		return content{}, err
	}
	if len(list) == 0 {
		return content{}, fail("Confluence 父页不存在或不可访问")
	}
	return list[0], nil
}

// Check resolves the parent using GET requests only. Success confirms read
// access, not permission to create pages, edit pages, or upload attachments.
func Check(ctx context.Context, cfg Config) (Page, error) { return (Client{}).Check(ctx, cfg) }

func (c Client) Check(ctx context.Context, cfg Config) (Page, error) {
	s, err := c.session(cfg)
	if err != nil {
		return Page{}, err
	}
	parent, err := s.parent(ctx)
	if err != nil {
		return Page{}, err
	}
	return s.page(parent), nil
}

func directChild(c, parent content, title string) error {
	if !validPage(c) || c.ID == parent.ID || c.Space.Key != parent.Space.Key ||
		c.Title != title || len(c.Ancestors) == 0 || c.Ancestors[len(c.Ancestors)-1].ID != parent.ID {
		return fail("Confluence 同名页面不属于指定父页，已停止同步")
	}
	return nil
}

func (s *session) child(ctx context.Context, parent content, title string) (content, bool, error) {
	list, err := s.byTitle(ctx, parent.Space.Key, title)
	if err != nil || len(list) == 0 {
		return content{}, false, err
	}
	return list[0], true, directChild(list[0], parent, title)
}

func pageBody(parent content, title, storage string) map[string]any {
	return map[string]any{
		"type": "page", "title": title,
		"space":     map[string]string{"key": parent.Space.Key},
		"ancestors": []map[string]string{{"id": parent.ID}},
		"body":      map[string]any{"storage": map[string]string{"value": storage, "representation": "storage"}},
	}
}

func (s *session) ensureChild(ctx context.Context, parent content, title string) (content, error) {
	existing, found, err := s.child(ctx, parent, title)
	if err != nil || found {
		return existing, err
	}
	var created content
	err = s.json(ctx, http.MethodPost, "/content", pageBody(parent, title, placeholder), &created)
	if hasStatus(err, http.StatusConflict, http.StatusBadRequest) {
		// Confluence can return either 400 or 409 for concurrent title creation.
		// Re-query once, and only adopt the winner if its direct parent matches.
		winner, found, lookupErr := s.child(ctx, parent, title)
		if lookupErr != nil {
			return content{}, lookupErr
		}
		if found {
			return winner, nil
		}
	}
	if err != nil {
		return content{}, err
	}
	if !numericID(created.ID) || created.ID == parent.ID {
		return content{}, fail("Confluence 新建页面信息无效")
	}
	// Read back the canonical page, including hierarchy and version.
	created, err = s.byID(ctx, created.ID)
	if err != nil {
		return content{}, err
	}
	return created, directChild(created, parent, title)
}

func validateAttachments(attachments []Attachment) error {
	if len(attachments) > maxAttachments {
		return fail("Confluence 附件数量过多")
	}
	seen := make(map[string]bool)
	total := 0
	for _, a := range attachments {
		if a.Filename == "" || len(a.Filename) > 255 || strings.TrimSpace(a.Filename) != a.Filename ||
			strings.ContainsAny(a.Filename, "/\\") || control(a.Filename) || a.Filename == "." ||
			a.Filename == ".." || seen[a.Filename] || len(a.Data) == 0 || len(a.Data) > maxAttachment {
			return fail("Confluence 附件名称或内容无效")
		}
		seen[a.Filename] = true
		total += len(a.Data)
		if total > maxTotalData {
			return fail("Confluence 附件内容过大")
		}
		if a.ContentType != "" {
			if _, _, err := mime.ParseMediaType(a.ContentType); err != nil || control(a.ContentType) {
				return fail("Confluence 附件类型无效")
			}
		}
	}
	return nil
}

func (s *session) findAttachment(ctx context.Context, pageID, filename string) (content, bool, error) {
	var list results
	err := s.request(ctx, http.MethodGet, "/content/"+pageID+"/child/attachment",
		url.Values{"filename": {filename}, "limit": {"2"}}, nil, "", &list)
	if err != nil {
		return content{}, false, err
	}
	if len(list.Results) > 1 || list.Links.Next != "" {
		return content{}, false, fail("Confluence 存在同名附件冲突")
	}
	if len(list.Results) == 0 {
		return content{}, false, nil
	}
	a := list.Results[0]
	if !attachmentID(a.ID) || a.Type != "attachment" || a.Title != filename {
		return content{}, false, fail("Confluence 附件信息无效")
	}
	return a, true, nil
}

func attachmentID(id string) bool {
	return numericID(strings.TrimPrefix(id, "att"))
}

func (s *session) upload(ctx context.Context, pageID string, a Attachment) error {
	existing, found, err := s.findAttachment(ctx, pageID, a.Filename)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(a.Data)
	contentNamed := strings.Contains(strings.ToLower(a.Filename), hex.EncodeToString(sum[:]))
	if found && contentNamed {
		return nil
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	contentType := a.ContentType
	if contentType == "" {
		contentType = http.DetectContentType(a.Data)
	}
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", mime.FormatMediaType("form-data", map[string]string{"name": "file", "filename": a.Filename}))
	header.Set("Content-Type", contentType)
	part, err := writer.CreatePart(header)
	if err != nil {
		return fail("Confluence 附件编码失败")
	}
	if _, err = part.Write(a.Data); err != nil {
		return fail("Confluence 附件编码失败")
	}
	if err = writer.Close(); err != nil {
		return fail("Confluence 附件编码失败")
	}
	path := "/content/" + pageID + "/child/attachment"
	if found {
		path += "/" + existing.ID + "/data"
	}
	var response json.RawMessage
	err = s.request(ctx, http.MethodPost, path, nil, body.Bytes(), writer.FormDataContentType(), &response)
	if hasStatus(err, http.StatusConflict, http.StatusBadRequest) && !found && contentNamed {
		_, found, lookupErr := s.findAttachment(ctx, pageID, a.Filename)
		if lookupErr != nil {
			return lookupErr
		}
		if found {
			return nil
		}
	}
	if err != nil {
		return err
	}
	// Creation returns a result collection; data updates may return content.
	var uploaded content
	var list results
	if json.Unmarshal(response, &list) == nil && len(list.Results) == 1 {
		uploaded = list.Results[0]
	} else if json.Unmarshal(response, &uploaded) != nil {
		return fail("Confluence 附件上传响应无效")
	}
	if !attachmentID(uploaded.ID) || uploaded.Type != "attachment" || uploaded.Title != a.Filename ||
		found && uploaded.ID != existing.ID {
		return fail("Confluence 附件上传响应无效")
	}
	return nil
}

// Sync upserts a title in the parent's space and requires the page's direct
// parent to match. The caller supplies a stable dated title and storage XHTML,
// including ri:attachment references to Attachment.Filename.
//
// New pages first receive a placeholder. The final storage body is submitted
// only after every attachment succeeds. Failures can leave a placeholder or
// uploaded attachments, allowing the same title to be retried safely.
//
// Filenames containing the full hex SHA-256 of Data reuse matching attachments.
// Other stable names replace existing attachment data (a new attachment version,
// not another attachment). Content-addressed names also keep earlier page bodies
// intact if a later attachment fails. Confluence keeps old page/attachment
// versions; this client does not delete history.
func Sync(ctx context.Context, cfg Config, title, storage string, attachments []Attachment) (Page, error) {
	return (Client{}).Sync(ctx, cfg, title, storage, attachments)
}

func (c Client) Sync(ctx context.Context, cfg Config, title, storage string, attachments []Attachment) (Page, error) {
	if strings.TrimSpace(title) == "" || len(title) > 1024 || control(title) || len(storage) > maxStorage {
		return Page{}, fail("Confluence 页面标题或内容无效")
	}
	if err := validateAttachments(attachments); err != nil {
		return Page{}, stageError("校验同步内容", err)
	}
	s, err := c.session(cfg)
	if err != nil {
		return Page{}, stageError("校验连接配置", err)
	}
	ctx, cancel := context.WithTimeout(ctx, syncTimeout)
	defer cancel()
	parent, err := s.parent(ctx)
	if err != nil {
		return Page{}, stageError("读取父页面", err)
	}
	if title == parent.Title {
		return Page{}, fail("Confluence 报告标题不能与父页相同")
	}
	child, err := s.ensureChild(ctx, parent, title)
	if err != nil {
		return Page{}, stageError("查找或创建日期页面", err)
	}
	for _, a := range attachments {
		if err := s.upload(ctx, child.ID, a); err != nil {
			return Page{}, stageError("上传图表附件", err)
		}
	}
	for attempt := 0; attempt < 2; attempt++ {
		// Re-check hierarchy and version after potentially slow uploads.
		current, err := s.byID(ctx, child.ID)
		if err != nil {
			return Page{}, stageError("读取日期页面版本", err)
		}
		if err = directChild(current, parent, title); err != nil {
			return Page{}, stageError("校验日期页面层级", err)
		}
		if current.Version.Number < 1 || current.Version.Number == int(^uint(0)>>1) {
			return Page{}, fail("Confluence 页面版本无效")
		}
		body := pageBody(parent, title, storage)
		body["id"] = current.ID
		body["version"] = map[string]int{"number": current.Version.Number + 1}
		var updated content
		err = s.json(ctx, http.MethodPut, "/content/"+current.ID, body, &updated)
		if hasStatus(err, http.StatusConflict) && attempt == 0 {
			continue
		}
		if err != nil {
			return Page{}, stageError("更新日期页面正文", err)
		}
		if updated.ID != current.ID || updated.Title != title || updated.Type != "page" {
			return Page{}, fail("Confluence 更新页面信息无效")
		}
		return s.page(updated), nil
	}
	return Page{}, fail("Confluence 页面存在并发修改")
}

// Use fixed operation names only; never include titles, URLs, bodies or tokens.
func stageError(stage string, err error) error {
	return &safeError{message: "Confluence " + stage + "失败：" + ErrorMessage(err), cause: err}
}
