package server

import "testing"

func TestTmpDebugSanitize(t *testing.T) {
	input := `<div><h1>团队早报</h1><p>{{introduction}}</p><p>{{date}} · {{timezone}}</p>{{yesterday}}{{unresolved}}<p>{{closing}}</p></div>`
	out, err := sanitizeEmailTemplateHTML(input)
	t.Logf("out=%q err=%v", out, err)
}
