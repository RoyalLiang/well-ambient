package server

import (
	"bytes"
	"encoding/base64"
	"fmt"
	nethtml "golang.org/x/net/html"
	"image/png"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"well-ambient/internal/config"
)

func TestEmailCommitPNGKeepsEveryBucketAndExactShares(t *testing.T) {
	authors := map[string]int{}
	for i := 0; i < 12; i++ {
		authors[fmt.Sprintf("author-%02d", i)] = i + 1
	}
	report := emailReport{Commits: &emailCommitSummary{Count: 78, Repositories: 3, Authors: authors}}
	if err := renderEmailReport(&report); err != nil {
		t.Fatal(err)
	}
	chart := report.CommitChart
	if chart == nil || len(chart.Bars) != 9 {
		t.Fatal("expected eight authors and the remaining-author aggregate")
	}
	tokens := nethtml.NewTokenizer(strings.NewReader(string(chart.SVG)))
	var shares []float64
	for {
		kind := tokens.Next()
		if kind == nethtml.ErrorToken {
			if tokens.Err() != io.EOF {
				t.Fatal(tokens.Err())
			}
			break
		}
		if kind != nethtml.StartTagToken && kind != nethtml.SelfClosingTagToken {
			continue
		}
		token := tokens.Token()
		if token.Data != "img" {
			continue
		}
		attrs := map[string]string{}
		for _, attr := range token.Attr {
			attrs[attr.Key] = attr.Val
		}
		data, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(attrs["src"], "data:image/png;base64,"))
		if err != nil {
			t.Fatal(err)
		}
		image, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		filled := 0
		for x := 0; x < image.Bounds().Dx(); x++ {
			_, _, _, alpha := image.At(x, image.Bounds().Dy()/2).RGBA()
			if alpha == 65535 {
				filled++
			}
		}
		shares = append(shares, float64(filled)/float64(image.Bounds().Dx()))
	}
	if len(shares) != len(chart.Bars) {
		t.Fatalf("PNG has %d bars for %d buckets", len(shares), len(chart.Bars))
	}
	for i, bar := range chart.Bars {
		want := float64(bar.Count) / float64(chart.Total)
		if math.Abs(shares[i]-want) > 1.0/(640*emailChartScale) {
			t.Errorf("bar %s share=%f want=%f", bar.Label, shares[i], want)
		}
	}

	for _, marker := range []string{"email-commit-section", "email-commit-summary", "其他（合计）", "贡献者", "12 人"} {
		if !strings.Contains(report.HTML, marker) {
			t.Errorf("missing commit summary content %q", marker)
		}
	}
	if !strings.Contains(report.Text, "其他（合计）：10") {
		t.Fatal("plain-text fallback lost remaining authors")
	}
}

func TestEmailCommitPNGEmptyDisabledAndEscaping(t *testing.T) {
	report := emailReport{Commits: &emailCommitSummary{Authors: map[string]int{}}}
	if err := renderEmailReport(&report); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(report.HTML, "暂无采集到的提交") || strings.Contains(report.HTML, `class="email-commit-bar"`) {
		t.Fatal("zero commits must render an explicit empty state without activity bars")
	}
	report.Commits.Count = 4
	if err := renderEmailReport(&report); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(report.HTML, "暂无作者分布") || strings.Contains(report.HTML, "暂无采集到的提交") {
		t.Fatal("missing authors must not be reported as zero collected commits")
	}
	report.Commits = &emailCommitSummary{Count: 10001, Authors: map[string]int{`<script>alert("author")</script>`: 1, "Main": 10000}}
	if err := renderEmailReport(&report); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(report.HTML, "<script>") || !strings.Contains(string(report.CommitChart.SVG), "&lt;script&gt;") {
		t.Fatal("commit author label must be escaped")
	}
	if !strings.Contains(string(report.CommitChart.SVG), "&lt;0.1%") {
		t.Fatal("small positive commit shares must not be labeled zero")
	}
	report.Commits = nil
	if err := renderEmailReport(&report); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(report.HTML, "email-commit-section") || report.CommitChart != nil {
		t.Fatal("disabled commits must omit the entire section")
	}
}

func TestEmailCommitPreviewStates(t *testing.T) {
	many := map[string]int{}
	for i := 0; i < 12; i++ {
		many[fmt.Sprintf("author-%02d", i)] = i + 1
	}
	cases := map[string]*emailCommitSummary{
		"empty":    {Authors: map[string]int{}},
		"missing":  {Count: 4, Repositories: 2, Authors: map[string]int{}},
		"single":   {Count: 8, Repositories: 1, Authors: map[string]int{"示例作者": 8}},
		"many":     {Count: 78, Repositories: 3, Authors: many},
		"long":     {Count: 10001, Repositories: 2, Authors: map[string]int{strings.Repeat("LongAuthor", 9): 10000, `<script>alert("author")</script>`: 1}},
		"disabled": nil,
	}
	for name, commits := range cases {
		t.Run(name, func(t *testing.T) {
			report, err := emailTemplateDemo(config.EmailTemplate{Style: "brief", Subject: "Commit preview", Introduction: "各位同事，早上好。"})
			if err != nil {
				t.Fatal(err)
			}
			report.Commits = commits
			report.ProjectGroupStats = nil
			if err := renderEmailReport(&report); err != nil {
				t.Fatal(err)
			}
			if commits != nil {
				contributors := 0
				for _, count := range commits.Authors {
					if count > 0 {
						contributors++
					}
				}
				if commits.Contributors != contributors || report.CommitChart == nil {
					t.Fatal("commit statistics lost their source author counts")
				}
			}
			if dir := os.Getenv("WELL_EMAIL_COMMIT_OUTPUT"); dir != "" {
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "email-commit-"+name+".html"), []byte(report.HTML), 0644); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
