package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// This fixture only listens on loopback and accepts an invented test credential.
type emailConfluenceBrowserStub struct {
	mu          sync.Mutex
	server      *httptest.Server
	fail        bool
	slow        bool
	reads       int
	creates     int
	updates     int
	uploads     int
	sends       int
	title       string
	storage     string
	sentHTML    string
	version     int
	attachments map[string]string
}

func newEmailConfluenceBrowserStub(t *testing.T) *emailConfluenceBrowserStub {
	t.Helper()
	stub := &emailConfluenceBrowserStub{attachments: map[string]string{}}
	page := func(id, title string, version int, child bool) map[string]any {
		ancestors := []map[string]string{}
		if child {
			ancestors = append(ancestors, map[string]string{"id": "100"})
		}
		return map[string]any{"id": id, "type": "page", "status": "current", "title": title,
			"space": map[string]string{"key": "~fixture"}, "version": map[string]int{"number": version}, "ancestors": ancestors}
	}
	stub.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fixture-confluence-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		stub.mu.Lock()
		slow := stub.slow
		stub.mu.Unlock()
		if slow {
			select {
			case <-r.Context().Done():
				return
			case <-time.After(350 * time.Millisecond):
			}
		}
		stub.mu.Lock()
		defer stub.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == "GET" && r.URL.Path == "/rest/api/content":
			stub.reads++
			found := []map[string]any{}
			title := r.URL.Query().Get("title")
			if title == "well-infra" {
				found = append(found, page("100", title, 1, false))
			} else if stub.title != "" && title == stub.title {
				found = append(found, page("101", stub.title, stub.version, true))
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"results": found})
		case r.Method == "GET" && r.URL.Path == "/rest/api/content/101":
			stub.reads++
			_ = json.NewEncoder(w).Encode(page("101", stub.title, stub.version, true))
		case r.Method == "POST" && r.URL.Path == "/rest/api/content":
			var body struct {
				Title string `json:"title"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, "invalid body", 400)
				return
			}
			if stub.title != "" {
				http.Error(w, "duplicate", 409)
				return
			}
			stub.creates++
			stub.title, stub.version = body.Title, 1
			_ = json.NewEncoder(w).Encode(page("101", stub.title, stub.version, true))
		case r.Method == "PUT" && r.URL.Path == "/rest/api/content/101":
			if stub.fail {
				http.Error(w, "fixture forced failure", 503)
				return
			}
			var body struct {
				Version struct {
					Number int `json:"number"`
				} `json:"version"`
				Body struct {
					Storage struct {
						Value string `json:"value"`
					} `json:"storage"`
				} `json:"body"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Version.Number != stub.version+1 {
				http.Error(w, "version conflict", 409)
				return
			}
			stub.updates++
			stub.version, stub.storage = body.Version.Number, body.Body.Storage.Value
			_ = json.NewEncoder(w).Encode(page("101", stub.title, stub.version, true))
		case r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/child/attachment"):
			stub.reads++
			found := []map[string]string{}
			name := r.URL.Query().Get("filename")
			if id := stub.attachments[name]; id != "" {
				found = append(found, map[string]string{"id": id, "type": "attachment", "title": name})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"results": found})
		case r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/child/attachment"):
			if r.Header.Get("X-Atlassian-Token") != "no-check" || r.ParseMultipartForm(8<<20) != nil {
				http.Error(w, "invalid upload", 400)
				return
			}
			defer r.MultipartForm.RemoveAll()
			file, header, err := r.FormFile("file")
			if err != nil {
				http.Error(w, "missing file", 400)
				return
			}
			file.Close()
			stub.uploads++
			id := fmt.Sprintf("att%d", stub.uploads+200)
			stub.attachments[header.Filename] = id
			_ = json.NewEncoder(w).Encode(map[string]any{"results": []map[string]string{{"id": id, "type": "attachment", "title": header.Filename}}})
		default:
			http.Error(w, "unsupported fixture operation", 404)
		}
	}))
	t.Cleanup(stub.server.Close)
	return stub
}

func (s *emailConfluenceBrowserStub) inspect(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r.Method == http.MethodPost {
		var settings struct {
			Fail bool `json:"fail"`
			Slow bool `json:"slow"`
		}
		if json.NewDecoder(r.Body).Decode(&settings) != nil {
			http.Error(w, "invalid fixture settings", 400)
			return
		}
		s.fail, s.slow = settings.Fail, settings.Slow
	}
	emailJSON(w, 200, map[string]any{
		"reads": s.reads, "creates": s.creates, "updates": s.updates, "uploads": s.uploads, "sends": s.sends,
		"title": s.title, "storage": s.storage, "sent_html": s.sentHTML,
	})
}
