package confluence

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestErrorMessageRedaction(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want string
	}{
		{"permission", statusError(403), "Confluence 拒绝访问"},
		{"authentication", statusError(401), "Confluence 认证失败"},
		{"wrapped", fmt.Errorf("secret-token: %w", statusError(404)), "Confluence 页面不存在"},
		{"timeout", fmt.Errorf("secret-token: %w", context.DeadlineExceeded), "Confluence 请求超时"},
		{"canceled", context.Canceled, "Confluence 请求已取消"},
		{"unknown", errors.New("secret-token"), "Confluence 同步失败"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := ErrorMessage(tc.err)
			if !strings.Contains(got, tc.want) || strings.Contains(got, "secret-token") {
				t.Fatalf("unsafe or missing diagnostic: %q", got)
			}
		})
	}
}

func TestHTTPDiagnosticsKeepStatusWithoutResponseBody(t *testing.T) {
	for _, code := range []int{400, 401, 403, 404, 409, 413, 429, 500, 502, 503} {
		if message := ErrorMessage(statusError(code)); !strings.Contains(message, fmt.Sprintf("HTTP %d", code)) {
			t.Errorf("status %d lost: %s", code, message)
		}
	}
}

func TestSyncDiagnosticIdentifiesFailedStage(t *testing.T) {
	for _, tc := range []struct {
		stage, method, path string
		status              int
	}{
		{"读取父页面", "GET", "/confluence/rest/api/content", 503},
		{"查找或创建日期页面", "POST", "/confluence/rest/api/content", 403},
		{"上传图表附件", "POST", "/confluence/rest/api/content/2/child/attachment", 413},
		{"更新日期页面正文", "PUT", "/confluence/rest/api/content/2", 400},
	} {
		t.Run(tc.stage, func(t *testing.T) {
			f := newFixture(t)
			f.hook = func(w http.ResponseWriter, r *http.Request) bool {
				if r.Method == tc.method && r.URL.Path == tc.path {
					w.WriteHeader(tc.status)
					_, _ = w.Write([]byte("private upstream body " + testToken))
					return true
				}
				return false
			}
			_, err := Sync(context.Background(), f.cfg(), reportTitle, finalStorage, []Attachment{{Filename: "chart.png", Data: []byte("png"), ContentType: "image/png"}})
			msg := ErrorMessage(err)
			if !strings.Contains(msg, tc.stage) || !strings.Contains(msg, fmt.Sprintf("HTTP %d", tc.status)) {
				t.Fatalf("missing stage/status: %s", msg)
			}
			if strings.Contains(msg, testToken) || strings.Contains(msg, "private upstream") {
				t.Fatal("diagnostic leaked response body")
			}
		})
	}
}
