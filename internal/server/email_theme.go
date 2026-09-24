package server

import "strings"

// Email clients vary in CSS support. Inline light styles remain the fallback;
// capable clients share one dark palette through media queries or OGSC markers.
var emailThemeTemplate = buildEmailThemeTemplate()

func buildEmailThemeTemplate() string {
	rules := []struct{ selectors, declarations string }{
		{"body,.email-body", "background-color:#111827!important;color:#e6edf5!important"},
		{".email-container", "background-color:#172131!important;border-color:#3c4c63!important;box-shadow:none!important"},
		{".email-card,.email-focus-card", "background-color:#202c3e!important;border-color:#3c4c63!important;box-shadow:none!important"},
		{".email-card-inner", "background-color:#253247!important;border-color:#3c4c63!important"},
		{".email-card-inner td", "border-right-color:#3c4c63!important"},
		{".email-h1,.email-h2,.email-h3,.email-td.email-h3,.email-focus-card-title", "color:#f1f5f9!important"},
		{".email-heading-h2", "color:#f1f5f9!important;border-top-color:#3c4c63!important"},
		{".email-divider,.email-border-subtle,.email-table", "border-color:#3c4c63!important"},
		{".email-thead-tr", "background-color:#202c3e!important"},
		{".email-tbody-tr", "background-color:#172131!important"},
		{".email-th", "color:#b7c4d6!important;border-bottom-color:#3c4c63!important"},
		{".email-td", "color:#e6edf5!important;border-bottom-color:#3c4c63!important"},
		{".email-text-main", "color:#e6edf5!important"},
		{".email-text-muted,.email-td.email-text-muted,.email-focus-card-meta", "color:#b7c4d6!important"},
		{".email-progress-bg", "background-color:#3c4c63!important"},
		{".email-progress-fill", "background-color:#0f766e!important;color:#ffffff!important"},
		{".email-badge-neutral,.chart-badge", "background:#253247!important;color:#e6edf5!important;border-color:#52647c!important"},
		{".email-chart-legend .email-status-label", "background-color:#253247!important;color:#f1f5f9!important;border-color:#64748b!important"},
		{".email-badge-blue", "background:#20364c!important;color:#93c5fd!important;border-color:#405d7c!important"},
		{".email-badge-green", "background:#1c3a35!important;color:#6ee7b7!important;border-color:#3c6a5b!important"},
		{".email-badge-amber", "background:#3a3021!important;color:#fcd34d!important;border-color:#6b5738!important"},
		{".email-badge-purple", "background:#342b48!important;color:#d8b4fe!important;border-color:#62517f!important"},
		{".email-badge-teal", "background:#1c393e!important;color:#5eead4!important;border-color:#3c6269!important"},
		{".email-jira-key,.email-brand-tag", "color:#5eead4!important"},
		{".email-text-amber,.email-value-amber", "color:#fcd34d!important"},
		{".email-value-blue", "color:#93c5fd!important"},
		{".email-value-green", "color:#6ee7b7!important"},
		{".email-value-purple", "color:#d8b4fe!important"},
		{".email-warning-box", "background:#312a1f!important;border-color:#6b5738!important;color:#fde68a!important"},
		{".email-focus-box", "background:#29271f!important;border-color:#6b5a35!important"},
		{".email-focus-tag,.email-focus-sub", "color:#fcd34d!important"},
		{".email-focus-title", "color:#fef3c7!important"},
		{".email-focus-sub-text", "color:#d6d3c4!important"},
		{".email-focus-count", "color:#fdba74!important"},
		{".chart-track", "stroke:#52647c!important"},
		{".chart-empty-surface", "fill:#202c3e!important;stroke:#52647c!important"},
		{".chart-arc-good", "stroke:#6ee7b7!important"},
		{".chart-arc-warn", "stroke:#fcd34d!important"},
		{".chart-arc-neutral", "stroke:#5eead4!important"},
		{".chart-text-main", "fill:#f1f5f9!important"},
		{".chart-text-sub", "fill:#b7c4d6!important"},
		{".chart-text-highlight", "fill:#5eead4!important"},
		{".chart-grid,.chart-base-line", "stroke:#52647c!important"},
		{".chart-dot", "fill:#202c3e!important;stroke:#5eead4!important"},
		{".chart-dot-last", "fill:#5eead4!important;stroke:#202c3e!important"},
		{".chart-slice", "stroke:#202c3e!important"},
		{".chart-curve-path", "stroke:#5eead4!important"},
		{".chart-grad-stop1", "stop-color:#5eead4!important;stop-opacity:0.24!important"},
		{".chart-grad-stop2", "stop-color:#5eead4!important;stop-opacity:0!important"},
		{".chart-series-0", "fill:#5eead4!important;background-color:#5eead4!important"},
		{".chart-series-1", "fill:#93c5fd!important;background-color:#93c5fd!important"},
		{".chart-series-2", "fill:#fcd34d!important;background-color:#fcd34d!important"},
		{".chart-series-3", "fill:#c4b5fd!important;background-color:#c4b5fd!important"},
		{".chart-series-4", "fill:#6ee7b7!important;background-color:#6ee7b7!important"},
		{".chart-series-5", "fill:#f9a8d4!important;background-color:#f9a8d4!important"},
		{".chart-series-6", "fill:#b7c4d6!important;background-color:#b7c4d6!important"},
		{".chart-bar-track", "fill:#253247!important"},
	}
	var out strings.Builder
	out.WriteString(`{{define "theme"}}<style>:root{color-scheme:light dark;supported-color-schemes:light dark}.email-brand-tag,.email-jira-key,.email-badge-teal{color:#00666c!important}.email-value-green{color:#047857!important}.email-value-amber{color:#92400e!important}.email-badge-blue,.email-value-blue{color:#075985!important}.email-text-muted,.email-td.email-text-muted{color:#536479!important}.chart-text-sub{fill:#536479!important}.chart-text-highlight{fill:#00666c!important}.email-chart-legend{overflow-wrap:normal;word-break:normal}.email-chart-legend .chart-badge{white-space:nowrap}.email-chart-description{white-space:normal;overflow:visible}@media(max-width:560px){.email-chart-cell,.email-category-cell{display:block!important;width:100%!important}.email-chart-gap,.email-category-gap{display:block!important;width:100%!important;height:12px!important}.email-content{padding:20px 14px!important}.email-body{padding:12px 0!important}}@media(prefers-color-scheme:dark){`)
	for _, rule := range rules {
		out.WriteString(rule.selectors + "{" + rule.declarations + "}")
	}
	out.WriteString("}")
	for _, rule := range rules {
		var selectors []string
		for _, selector := range strings.Split(rule.selectors, ",") {
			selectors = append(selectors, "[data-ogsc] "+selector, "[data-ogsb] "+selector)
			if selector == "body" || selector == ".email-body" {
				selectors = append(selectors, selector+"[data-ogsc]", selector+"[data-ogsb]")
			}
		}
		out.WriteString(strings.Join(selectors, ",") + "{" + rule.declarations + "}")
	}
	out.WriteString("</style>{{end}}")
	return out.String()
}
