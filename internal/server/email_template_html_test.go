package server

import (
	"strings"
	"testing"
)

func TestSanitizeEmailTemplateHTML(t *testing.T) {
	input := `<div style="color:#333"><p>{{introduction}}</p><script>alert(1)</script><a href="javascript:alert(2)" onclick="alert(3)">链接</a><img src="data:image/png;base64,AAAA" onerror="alert(4)"></div>`
	got, err := sanitizeEmailTemplateHTML(input)
	if err != nil {
		t.Fatal(err)
	}
	for _, unsafe := range []string{"<script", "javascript:", "onclick", "onerror"} {
		if strings.Contains(strings.ToLower(got), unsafe) {
			t.Fatalf("unsafe markup survived: %s", unsafe)
		}
	}
	if !strings.Contains(got, "{{introduction}}") || !strings.Contains(got, "data:image/png;base64,AAAA") {
		t.Fatalf("safe markup lost: %s", got)
	}
}

func TestSanitizeEmailTemplateHTMLRejectsEmptyBody(t *testing.T) {
	if _, err := sanitizeEmailTemplateHTML(``); err == nil {
		t.Fatal("empty html accepted")
	}
}
