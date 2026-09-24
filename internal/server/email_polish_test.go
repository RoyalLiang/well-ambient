package server

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"well-ambient/internal/config"
)

func TestEmailPolishRelativeAxisAndZeroCommits(t *testing.T) {
	for _, style := range []string{"brief", "focus", "ledger", "hyperframe"} {
		for _, state := range []string{"normal", "zero", "missing", "edge", "progress"} {
			t.Run(style+"/"+state, func(t *testing.T) {
				report, err := emailTemplateDemo(config.EmailTemplate{Style: style})
				if err != nil {
					t.Fatal(err)
				}
				report.Subject = "Email polish " + style + " " + state
				switch state {
				case "zero":
					report.Commits = &emailCommitSummary{Authors: map[string]int{}}
					report.TrendPoints = []int{0, 0, 0, 0, 0, 0, 0}
				case "missing":
					report.Commits = &emailCommitSummary{Count: 4, Repositories: 2}
				case "edge":
					report.TrendPoints = []int{0, 1, 99, 3, 10000, 4, 1000}
					report.YesterdayUpdated = []emailIssue{{Status: strings.Repeat("待验收", 10)}}
				case "progress":
					report.ProjectGroupStats = nil
					for _, rate := range []int{0, 1, 49, 50, 99, 100} {
						report.ProjectGroupStats = append(report.ProjectGroupStats, emailProjectGroupStat{
							Name: fmt.Sprintf("Progress %d", rate), YesterdayCount: 100, ResolutionRate: rate,
						})
					}
				}
				if err := renderEmailReport(&report); err != nil {
					t.Fatal(err)
				}
				for i, bar := range report.TrendBars {
					if bar.Label != fmt.Sprintf("-%dd", 7-i) {
						t.Fatalf("relative day mismatch: %q", bar.Label)
					}
				}
				if style == "hyperframe" && strings.Contains(report.HTML, "HYPERFRAME · 研发全景看板") {
					t.Fatal("removed masthead returned")
				}
				if strings.Count(string(report.CurveChartSVG), `class="email-trend-tick"`) != 3 {
					t.Fatal("trend must retain three labeled integer ticks")
				}
				if state == "zero" {
					if strings.Count(report.HTML, `class="email-commit-zero-bar"`) != 3 || len(report.CommitChart.Bars) != 0 {
						t.Fatal("zero metrics require three neutral tracks without invented authors")
					}
					for _, label := range []string{"采集提交：0 次", "涉及仓库：0 个", "贡献者：0 人"} {
						if !strings.Contains(report.HTML, label) {
							t.Errorf("missing zero metric %q", label)
						}
					}
					if strings.Contains(string(report.CommitChart.SVG), "%") && strings.Contains(string(report.CommitChart.SVG), "占 ") {
						t.Fatal("zero authors have no meaningful share")
					}
				}
				if state == "missing" && (strings.Contains(report.HTML, "email-commit-zero-bar") || !strings.Contains(report.HTML, "暂无作者分布")) {
					t.Fatal("unknown authors must not become a zero-activity chart")
				}
				if dir := os.Getenv("WELL_EMAIL_POLISH_OUTPUT"); dir != "" {
					if err := os.MkdirAll(dir, 0755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(dir, "email-polish-"+style+"-"+state+".html"), []byte(report.HTML), 0644); err != nil {
						t.Fatal(err)
					}
				}
			})
		}
	}
}

func TestEmailProgressExactWidthsAndEndpoints(t *testing.T) {
	for _, rate := range []int{0, 1, 49, 50, 99, 100} {
		markup := string(renderEmailProgress(rate, 100))
		if !strings.Contains(markup, fmt.Sprintf(">%d%%</td>", rate)) {
			t.Errorf("%d: percentage must appear inside the track", rate)
		}
		if rate > 0 && !strings.Contains(markup, fmt.Sprintf(`width="%d%%" class="email-progress-fill"`, rate)) {
			t.Errorf("%d: fill width no longer matches data", rate)
		}
		if (rate == 0 || rate == 100) && strings.Count(markup, "<td ") != 1 {
			t.Errorf("%d: endpoint must not leave an empty cell", rate)
		}
	}
	if !strings.Contains(string(renderEmailProgress(0, 0)), ">—</td>") {
		t.Fatal("missing denominator must remain not applicable")
	}
}
