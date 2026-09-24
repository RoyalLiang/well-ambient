package confluence

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

const testToken = "test-secret-PAT-do-not-leak"
const reportTitle = "Daily 2026-09-17"
const finalStorage = `<p>Report</p><ac:image><ri:attachment ri:filename="chart.png"/></ac:image>`

type fixture struct {
	t           *testing.T
	server      *httptest.Server
	mu          sync.Mutex
	parent      content
	child       *content
	attachments map[string]content
	data        map[string][]byte
	events      []string
	writes      []map[string]any
	conflicts   int
	createRace  int
	wrongWinner bool
	hook        func(http.ResponseWriter, *http.Request) bool
}

func pageContent(id, title, parentID string, version int) content {
	c := content{ID: id, Type: "page", Status: "current", Title: title}
	c.Space.Key = "~zhiyuan_liang"
	c.Version.Number = version
	if parentID != "" {
		c.Ancestors = append(c.Ancestors, struct {
			ID string `json:"id"`
		}{ID: parentID})
	}
	return c
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{
		t: t, parent: pageContent("1", "well infra", "", 1),
		attachments: make(map[string]content), data: make(map[string][]byte),
	}
	f.server = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.server.Close)
	return f
}

func (f *fixture) cfg() Config {
	return Config{ParentPageURL: f.server.URL + "/confluence/display/~zhiyuan_liang/well+infra?tracking=discard", Token: testToken}
}

func (f *fixture) existing(parent string) {
	c := pageContent("2", reportTitle, parent, 3)
	f.child = &c
}

func respond(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}

func (f *fixture) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, r.Method+" "+r.URL.Path)
	if r.Header.Get("Authorization") != "Bearer "+testToken {
		f.t.Errorf("missing Bearer auth on %s %s", r.Method, r.URL.Path)
	}
	if r.URL.Query().Get("tracking") != "" {
		f.t.Error("tracking query forwarded")
	}
	if f.hook != nil && f.hook(w, r) {
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/confluence/rest/api")
	if path == r.URL.Path {
		f.t.Errorf("missing context path: %s", r.URL.Path)
		http.NotFound(w, r)
		return
	}
	switch {
	case r.Method == http.MethodGet && path == "/content":
		if r.URL.Query().Get("spaceKey") != f.parent.Space.Key || r.URL.Query().Get("expand") != "space,ancestors,version" {
			f.t.Errorf("unexpected query: %v", r.URL.Query())
		}
		list := []content{}
		switch r.URL.Query().Get("title") {
		case f.parent.Title:
			list = append(list, f.parent)
		case reportTitle:
			if f.child != nil {
				list = append(list, *f.child)
			}
		}
		respond(w, map[string]any{"results": list})
	case r.Method == http.MethodGet && path == "/content/1":
		respond(w, f.parent)
	case r.Method == http.MethodGet && path == "/content/2":
		if f.child == nil {
			http.NotFound(w, r)
			return
		}
		respond(w, f.child)
	case r.Method == http.MethodPost && path == "/content":
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			f.t.Errorf("decode creation: %v", err)
		}
		f.writes = append(f.writes, body)
		f.existing("1")
		f.child.Version.Number = 1
		if f.createRace != 0 {
			if f.wrongWinner {
				f.child.Ancestors[0].ID = "999"
			}
			w.WriteHeader(f.createRace)
			return
		}
		w.WriteHeader(http.StatusCreated)
		respond(w, f.child)
	case r.Method == http.MethodPut && path == "/content/2":
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			f.t.Errorf("decode update: %v", err)
		}
		f.writes = append(f.writes, body)
		if f.conflicts > 0 {
			f.conflicts--
			f.child.Version.Number++
			w.WriteHeader(http.StatusConflict)
			return
		}
		version := int(body["version"].(map[string]any)["number"].(float64))
		if version != f.child.Version.Number+1 {
			f.t.Errorf("version = %d; want %d", version, f.child.Version.Number+1)
		}
		f.child.Version.Number = version
		respond(w, map[string]any{"id": "2", "type": "page", "title": reportTitle, "_links": map[string]string{"webui": "https://evil.invalid/stolen"}})
	case r.Method == http.MethodGet && path == "/content/2/child/attachment":
		list := []content{}
		if a, ok := f.attachments[r.URL.Query().Get("filename")]; ok {
			list = append(list, a)
		}
		respond(w, map[string]any{"results": list})
	case r.Method == http.MethodPost && strings.HasPrefix(path, "/content/2/child/attachment"):
		if r.Header.Get("X-Atlassian-Token") != "no-check" {
			f.t.Error("missing attachment CSRF bypass header")
		}
		if err := r.ParseMultipartForm(maxAttachment + 1024); err != nil {
			f.t.Errorf("parse multipart: %v", err)
			w.WriteHeader(400)
			return
		}
		if r.MultipartForm != nil {
			defer r.MultipartForm.RemoveAll()
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			f.t.Errorf("file part: %v", err)
			w.WriteHeader(400)
			return
		}
		defer file.Close()
		if header.Header.Get("Content-Type") != "image/png" {
			f.t.Errorf("content type: %s", header.Header.Get("Content-Type"))
		}
		data, _ := io.ReadAll(file)
		f.data[header.Filename] = data
		a := content{ID: "att10", Type: "attachment", Title: header.Filename}
		if existing, ok := f.attachments[header.Filename]; ok {
			a = existing
			if path != "/content/2/child/attachment/"+a.ID+"/data" {
				f.t.Errorf("expected attachment update, got %s", path)
			}
		}
		f.attachments[header.Filename] = a
		if strings.HasSuffix(path, "/data") {
			respond(w, a)
		} else {
			respond(w, map[string]any{"results": []content{a}})
		}
	default:
		f.t.Errorf("unexpected request: %s %s", r.Method, path)
		http.NotFound(w, r)
	}
}

func storageOf(body map[string]any) string {
	return body["body"].(map[string]any)["storage"].(map[string]any)["value"].(string)
}

func TestCheckURLFormsAndReadOnly(t *testing.T) {
	for _, path := range []string{
		"/confluence/display/~zhiyuan_liang/well+infra?tracking=discard",
		"/confluence/display/~zhiyuan_liang/well%20infra",
		"/confluence/pages/viewpage.action?pageId=1&tracking=discard",
		"/confluence/spaces/IGNORED/pages/1/title",
	} {
		t.Run(path, func(t *testing.T) {
			f := newFixture(t)
			cfg := f.cfg()
			cfg.ParentPageURL = f.server.URL + path
			page, err := Check(context.Background(), cfg)
			if err != nil {
				t.Fatal(err)
			}
			if page.ID != "1" || page.Title != f.parent.Title || page.URL != f.server.URL+"/confluence/pages/viewpage.action?pageId=1" {
				t.Fatalf("unexpected page: %+v", page)
			}
			for _, event := range f.events {
				if !strings.HasPrefix(event, "GET ") {
					t.Fatalf("Check wrote: %s", event)
				}
			}
		})
	}
}

func TestLiteralPlusInDisplayTitle(t *testing.T) {
	f := newFixture(t)
	f.parent.Title = "C++ guide"
	cfg := f.cfg()
	cfg.ParentPageURL = f.server.URL + "/confluence/display/~zhiyuan_liang/C%2B%2B+guide"
	if _, err := Check(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
}

func TestValidateURLAndToken(t *testing.T) {
	valid := []string{
		"https://confluence.westwell-lab.com/display/~zhiyuan_liang/well-infra?src=test",
		"https://host.example/confluence/pages/viewpage.action?pageId=123",
		"https://host.example/confluence/spaces/KEY/pages/123/title",
		"http://127.0.0.1:1234/pages/viewpage.action?pageId=123",
		"http://[::1]:1234/display/KEY/Page",
		"https://host.example/context%20path/display/KEY/Page",
	}
	for _, raw := range valid {
		if err := Validate(Config{ParentPageURL: raw, Token: testToken}); err != nil {
			t.Errorf("valid URL %q: %v", raw, err)
		}
	}
	invalid := []string{
		"", "https:///display/K/T", "/display/K/T", "http://remote.example/display/K/T",
		"http://127.0.0.1.evil.example/display/K/T", "ftp://host.example/display/K/T",
		"https://user:pass@host.example/display/K/T", "https://host.example/display/K/T#fragment",
		"https://host.example/display/K/T#", "https://host.example/display/K/T/",
		"https://host.example/display/K/a%2fb", "https://host.example/display/K/a%5cb",
		"https://host.example/../display/K/T", "https://host.example/%2e%2e/display/K/T",
		"https://host.example//display/K/T", "https://host.example/display/K/%0a",
		"https://host.example/display/K/T?bad=%XX", "https://host.example/pages/viewpage.action",
		"https://host.example/pages/viewpage.action?pageId=-1", "https://host.example/pages/viewpage.action?pageId=0",
		"https://host.example/pages/viewpage.action?pageId=1&pageId=2",
		"https://host.example/pages/viewpage.action?pageId=1%2f2", "https://host.example/unknown",
		"https://host.example:65536/display/K/T", "https://host.example:/display/K/T",
		"https://host.example\\evil/display/K/T",
	}
	for _, raw := range invalid {
		if err := Validate(Config{ParentPageURL: raw, Token: testToken}); err == nil {
			t.Errorf("accepted invalid URL %q", raw)
		}
	}
	for _, token := range []string{"", "__configured__", " token", "token ", "a\nb", "a\rb", "a b", strings.Repeat("x", (64<<10)+1)} {
		if err := Validate(Config{ParentPageURL: valid[0], Token: token}); err == nil {
			t.Error("accepted invalid token")
		}
	}
}

func TestSyncCreateUploadThenFinalContent(t *testing.T) {
	f := newFixture(t)
	data := []byte("\x89PNG\r\n\x1a\nexample")
	page, err := Sync(context.Background(), f.cfg(), reportTitle, finalStorage, []Attachment{{Filename: "chart.png", Data: data, ContentType: "image/png"}})
	if err != nil {
		t.Fatal(err)
	}
	if page.ID != "2" || page.URL != f.server.URL+"/confluence/pages/viewpage.action?pageId=2" {
		t.Fatalf("untrusted or incorrect result: %+v", page)
	}
	if len(f.writes) != 2 || storageOf(f.writes[0]) != placeholder || storageOf(f.writes[1]) != finalStorage {
		t.Fatalf("expected placeholder then final content: %+v", f.writes)
	}
	create := f.writes[0]
	if create["space"].(map[string]any)["key"] != f.parent.Space.Key ||
		create["ancestors"].([]any)[0].(map[string]any)["id"] != "1" ||
		create["body"].(map[string]any)["storage"].(map[string]any)["representation"] != "storage" {
		t.Fatalf("creation contract: %+v", create)
	}
	if !bytes.Equal(f.data["chart.png"], data) {
		t.Fatal("uploaded bytes changed")
	}
	upload, update := -1, -1
	for i, event := range f.events {
		if event == "POST /confluence/rest/api/content/2/child/attachment" {
			upload = i
		}
		if event == "PUT /confluence/rest/api/content/2" {
			update = i
		}
	}
	if upload < 0 || update <= upload {
		t.Fatalf("bad request order: %v", f.events)
	}
}

func TestSyncExistingAttachmentDataUpdate(t *testing.T) {
	f := newFixture(t)
	f.existing("1")
	f.attachments["chart.png"] = content{ID: "10", Type: "attachment", Title: "chart.png"}
	_, err := Sync(context.Background(), f.cfg(), reportTitle, finalStorage,
		[]Attachment{{Filename: "chart.png", Data: []byte("new png bytes"), ContentType: "image/png"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(f.writes) != 1 || f.child.Version.Number != 4 {
		t.Fatalf("unexpected updates: %+v", f.writes)
	}
	if string(f.data["chart.png"]) != "new png bytes" {
		t.Fatal("attachment data not updated")
	}
}

func TestContentNamedAttachmentsAreReused(t *testing.T) {
	f := newFixture(t)
	data := []byte("stable png bytes")
	sum := sha256.Sum256(data)
	filename := "daily-jira-chart-" + hex.EncodeToString(sum[:]) + ".png"
	attachments := []Attachment{{Filename: filename, Data: data, ContentType: "image/png"}}
	for i := 0; i < 2; i++ {
		if _, err := Sync(context.Background(), f.cfg(), reportTitle, finalStorage, attachments); err != nil {
			t.Fatal(err)
		}
	}
	uploads := 0
	for _, event := range f.events {
		if strings.HasPrefix(event, "POST ") && strings.Contains(event, "/attachment") {
			uploads++
		}
	}
	if uploads != 1 {
		t.Fatalf("uploads = %d; want one", uploads)
	}
}

func TestAttachmentFailureDoesNotSubmitFinalContent(t *testing.T) {
	f := newFixture(t)
	f.hook = func(w http.ResponseWriter, r *http.Request) bool {
		if r.Method == "POST" && strings.Contains(r.URL.Path, "/attachment") {
			w.WriteHeader(403)
			_, _ = io.WriteString(w, testToken)
			return true
		}
		return false
	}
	page, err := Sync(context.Background(), f.cfg(), reportTitle, finalStorage,
		[]Attachment{{Filename: "chart.png", Data: []byte("png"), ContentType: "image/png"}})
	if err == nil || page != (Page{}) || len(f.writes) != 1 || storageOf(f.writes[0]) != placeholder {
		t.Fatalf("failure reported success: page=%+v err=%v writes=%v", page, err, f.writes)
	}
	if strings.Contains(err.Error(), testToken) {
		t.Fatal("remote body leaked")
	}
}

func TestSafeHTTPFailures(t *testing.T) {
	for _, status := range []int{401, 403, 404, 302, 500} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			f := newFixture(t)
			f.hook = func(w http.ResponseWriter, r *http.Request) bool {
				w.Header().Set("Location", "https://evil.invalid/?token="+testToken)
				w.WriteHeader(status)
				_, _ = io.WriteString(w, testToken+" remote-private-body")
				return true
			}
			page, err := Check(context.Background(), f.cfg())
			if err == nil || page != (Page{}) {
				t.Fatalf("unexpected success: %+v %v", page, err)
			}
			for _, forbidden := range []string{testToken, "remote-private-body", "evil.invalid"} {
				if strings.Contains(err.Error(), forbidden) {
					t.Errorf("error leaked %q: %v", forbidden, err)
				}
			}
			if len(f.events) != 1 {
				t.Fatalf("unexpected retries: %v", f.events)
			}
		})
	}
}

func TestRedirectNeverForwardsBearer(t *testing.T) {
	targetHits := 0
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { targetHits++ }))
	defer target.Close()
	f := newFixture(t)
	f.hook = func(w http.ResponseWriter, r *http.Request) bool {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
		return true
	}
	called := false
	injected := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { called = true; return nil }}
	_, err := (Client{HTTPClient: injected}).Check(context.Background(), f.cfg())
	if err == nil || targetHits != 0 || called {
		t.Fatalf("redirect followed: hits=%d hook=%v err=%v", targetHits, called, err)
	}
	if injected.CheckRedirect == nil || injected.Timeout != 0 {
		t.Fatal("injected client mutated")
	}
}

func TestMissingAndWrongParent(t *testing.T) {
	t.Run("unknown parent", func(t *testing.T) {
		f := newFixture(t)
		cfg := f.cfg()
		cfg.ParentPageURL = strings.Replace(cfg.ParentPageURL, "well+infra", "missing", 1)
		if _, err := Sync(context.Background(), cfg, reportTitle, finalStorage, nil); err == nil {
			t.Fatal("unknown parent accepted")
		}
		if len(f.writes) != 0 {
			t.Fatal("wrote without parent")
		}
	})
	t.Run("same title different parent", func(t *testing.T) {
		f := newFixture(t)
		f.existing("999")
		if _, err := Sync(context.Background(), f.cfg(), reportTitle, finalStorage, nil); err == nil {
			t.Fatal("overwrote unrelated page")
		}
		if len(f.writes) != 0 {
			t.Fatal("wrote unrelated page")
		}
	})
	t.Run("parent title", func(t *testing.T) {
		f := newFixture(t)
		if _, err := Sync(context.Background(), f.cfg(), f.parent.Title, finalStorage, nil); err == nil || len(f.writes) != 0 {
			t.Fatal("parent title accepted")
		}
	})
	t.Run("parent id", func(t *testing.T) {
		f := newFixture(t)
		f.existing("1")
		f.child.ID = "1"
		if _, err := Sync(context.Background(), f.cfg(), reportTitle, finalStorage, nil); err == nil || len(f.writes) != 0 {
			t.Fatal("parent id accepted")
		}
	})
	t.Run("ancestor but not direct parent", func(t *testing.T) {
		f := newFixture(t)
		f.existing("1")
		f.child.Ancestors = append(f.child.Ancestors, struct {
			ID string `json:"id"`
		}{ID: "9"})
		if _, err := Sync(context.Background(), f.cfg(), reportTitle, finalStorage, nil); err == nil || len(f.writes) != 0 {
			t.Fatal("indirect parent accepted")
		}
	})
}

func TestVersionConflictRetriesOnce(t *testing.T) {
	for _, conflicts := range []int{1, 2} {
		t.Run(strconv.Itoa(conflicts), func(t *testing.T) {
			f := newFixture(t)
			f.existing("1")
			f.conflicts = conflicts
			_, err := Sync(context.Background(), f.cfg(), reportTitle, finalStorage, nil)
			if (err != nil) != (conflicts == 2) {
				t.Fatalf("unexpected result: %v", err)
			}
			if len(f.writes) != 2 {
				t.Fatalf("updates = %d; want 2", len(f.writes))
			}
			got := []float64{f.writes[0]["version"].(map[string]any)["number"].(float64), f.writes[1]["version"].(map[string]any)["number"].(float64)}
			if !reflect.DeepEqual(got, []float64{4, 5}) {
				t.Fatalf("versions: %v", got)
			}
		})
	}
}

func TestConcurrentCreateAdoptsOnlyMatchingParent(t *testing.T) {
	for _, status := range []int{400, 409} {
		for _, wrong := range []bool{false, true} {
			t.Run(strconv.Itoa(status)+"/wrong="+strconv.FormatBool(wrong), func(t *testing.T) {
				f := newFixture(t)
				f.createRace, f.wrongWinner = status, wrong
				_, err := Sync(context.Background(), f.cfg(), reportTitle, finalStorage, nil)
				if (err != nil) != wrong {
					t.Fatalf("unexpected result: %v", err)
				}
				want := 2
				if wrong {
					want = 1
				}
				if len(f.writes) != want {
					t.Fatalf("writes = %d; want %d", len(f.writes), want)
				}
			})
		}
	}
}

func TestContextCancellationAndTimeout(t *testing.T) {
	for _, cancelImmediately := range []bool{false, true} {
		t.Run(strconv.FormatBool(cancelImmediately), func(t *testing.T) {
			started := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				close(started)
				<-r.Context().Done()
			}))
			defer server.Close()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			cfg := Config{ParentPageURL: server.URL + "/pages/viewpage.action?pageId=1", Token: testToken}
			if cancelImmediately {
				cancel()
			} else {
				go func() { <-started; cancel() }()
			}
			_, err := Check(ctx, cfg)
			if !errors.Is(err, context.Canceled) || strings.Contains(err.Error(), server.URL) {
				t.Fatalf("cancellation: %v", err)
			}
		})
	}
	t.Run("injected timeout", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
		defer server.Close()
		cfg := Config{ParentPageURL: server.URL + "/pages/viewpage.action?pageId=1", Token: testToken}
		_, err := (Client{HTTPClient: &http.Client{Timeout: 10 * time.Millisecond}}).Check(context.Background(), cfg)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("timeout: %v", err)
		}
	})
}

func TestMalformedAndOversizeResponses(t *testing.T) {
	for _, body := range []string{
		"<html>" + testToken + "</html>",
		`{"results":[{"id":"1","type":"page","title":"bad"}]}`,
		strings.Repeat("x", maxResponse+1),
		`{"results":[],"_links":{"next":"https://evil.invalid/"}}`,
	} {
		f := newFixture(t)
		f.hook = func(w http.ResponseWriter, r *http.Request) bool {
			_, _ = io.WriteString(w, body)
			return true
		}
		if _, err := Check(context.Background(), f.cfg()); err == nil || strings.Contains(err.Error(), testToken) {
			t.Fatalf("invalid response accepted or leaked: %v", err)
		}
	}
}

func TestInvalidInputsSendNoRequests(t *testing.T) {
	f := newFixture(t)
	badAttachments := [][]Attachment{
		{{Filename: "../chart.png", Data: []byte("x")}},
		{{Filename: "chart.png", Data: []byte("x"), ContentType: "image/png\r\nAuthorization: secret"}},
		{{Filename: "chart.png", Data: []byte("x")}, {Filename: "chart.png", Data: []byte("y")}},
		{{Filename: "chart.png", Data: make([]byte, maxAttachment+1)}},
	}
	for _, attachments := range badAttachments {
		if _, err := Sync(context.Background(), f.cfg(), reportTitle, finalStorage, attachments); err == nil {
			t.Fatal("accepted bad attachment")
		}
	}
	for _, title := range []string{"", " ", "title\n"} {
		if _, err := Sync(context.Background(), f.cfg(), title, finalStorage, nil); err == nil {
			t.Fatal("accepted bad title")
		}
	}
	if len(f.events) != 0 {
		t.Fatalf("invalid inputs sent requests: %v", f.events)
	}
}

func TestRootContextPath(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		respond(w, pageContent("1", "Parent", "", 1))
	}))
	defer server.Close()
	cfg := Config{ParentPageURL: server.URL + "/pages/viewpage.action?pageId=1&tracking=ignore", Token: testToken}
	page, err := Check(context.Background(), cfg)
	if err != nil || gotPath != "/rest/api/content/1" {
		t.Fatalf("root path: %q %v", gotPath, err)
	}
	u, _ := url.Parse(page.URL)
	if u.RawQuery != "pageId=1" {
		t.Fatalf("query not canonical: %s", page.URL)
	}
}

func TestConcurrentCreateWithoutWinnerIsBounded(t *testing.T) {
	f := newFixture(t)
	creates := 0
	f.hook = func(w http.ResponseWriter, r *http.Request) bool {
		if r.Method == "POST" && r.URL.Path == "/confluence/rest/api/content" {
			creates++
			w.WriteHeader(http.StatusConflict)
			return true
		}
		return false
	}
	if _, err := Sync(context.Background(), f.cfg(), reportTitle, finalStorage, nil); err == nil {
		t.Fatal("conflict without child reported success")
	}
	if creates != 1 || len(f.events) != 4 {
		t.Fatalf("unbounded create/requery: %d %v", creates, f.events)
	}
}

func TestMovedChildAfterUploadIsNotUpdated(t *testing.T) {
	f := newFixture(t)
	f.existing("1")
	f.hook = func(w http.ResponseWriter, r *http.Request) bool {
		if r.Method == "GET" && r.URL.Path == "/confluence/rest/api/content/2" {
			f.child.Ancestors[0].ID = "999"
		}
		return false
	}
	_, err := Sync(context.Background(), f.cfg(), reportTitle, finalStorage,
		[]Attachment{{Filename: "chart.png", Data: []byte("png"), ContentType: "image/png"}})
	if err == nil || len(f.writes) != 0 {
		t.Fatalf("moved page updated: %v %+v", err, f.writes)
	}
}

func TestAllAttachmentsMustSucceedBeforeFinalContent(t *testing.T) {
	for _, invalidResponse := range []bool{false, true} {
		t.Run(strconv.FormatBool(invalidResponse), func(t *testing.T) {
			f := newFixture(t)
			uploads := 0
			f.hook = func(w http.ResponseWriter, r *http.Request) bool {
				if r.Method == "POST" && strings.Contains(r.URL.Path, "/attachment") {
					uploads++
					if uploads == 2 {
						if invalidResponse {
							respond(w, map[string]any{"results": []any{}})
						} else {
							w.WriteHeader(http.StatusInternalServerError)
						}
						return true
					}
				}
				return false
			}
			attachments := []Attachment{
				{Filename: "one.png", Data: []byte("one"), ContentType: "image/png"},
				{Filename: "two.png", Data: []byte("two"), ContentType: "image/png"},
			}
			page, err := Sync(context.Background(), f.cfg(), reportTitle, finalStorage, attachments)
			if err == nil || page != (Page{}) || len(f.writes) != 1 || len(f.data) != 1 {
				t.Fatalf("partial upload reported success: %+v %v", page, err)
			}
			// Retrying the same title reuses the child and can complete.
			f.hook = nil
			if _, err := Sync(context.Background(), f.cfg(), reportTitle, finalStorage, attachments); err != nil {
				t.Fatal(err)
			}
			if len(f.writes) != 2 {
				t.Fatalf("retry created another page: %+v", f.writes)
			}
		})
	}
}

func TestSHA256AttachmentCreateRace(t *testing.T) {
	f := newFixture(t)
	data := []byte("concurrent png")
	sum := sha256.Sum256(data)
	filename := "chart-" + hex.EncodeToString(sum[:]) + ".png"
	uploads := 0
	f.hook = func(w http.ResponseWriter, r *http.Request) bool {
		if r.Method == "POST" && strings.Contains(r.URL.Path, "/attachment") {
			uploads++
			f.attachments[filename] = content{ID: "att12", Type: "attachment", Title: filename}
			w.WriteHeader(http.StatusConflict)
			return true
		}
		return false
	}
	_, err := Sync(context.Background(), f.cfg(), reportTitle, finalStorage,
		[]Attachment{{Filename: filename, Data: data, ContentType: "image/png"}})
	if err != nil || uploads != 1 || len(f.writes) != 2 {
		t.Fatalf("attachment race: uploads=%d err=%v", uploads, err)
	}
}

func TestCancellationDuringSyncDoesNotReportSuccess(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f := newFixture(t)
	f.existing("1")
	f.hook = func(w http.ResponseWriter, r *http.Request) bool {
		if r.Method == "PUT" {
			_, _ = io.Copy(io.Discard, r.Body)
			cancel()
			select {
			case <-r.Context().Done():
			case <-time.After(time.Second):
				t.Error("server did not observe canceled update")
			}
			return true
		}
		return false
	}
	page, err := Sync(ctx, f.cfg(), reportTitle, finalStorage, nil)
	if !errors.Is(err, context.Canceled) || page != (Page{}) {
		t.Fatalf("canceled update: %+v %v", page, err)
	}
}
