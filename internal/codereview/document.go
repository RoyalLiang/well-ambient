package codereview

import (
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"path"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"well-ambient/internal/db"
)

// EvidenceContract freezes the runtime rules actually supplied to the model.
type EvidenceContract struct {
	Version    string   `json:"version"`
	PromptHash string   `json:"prompt_hash"`
	Rules      []string `json:"rules"`
}

type ReviewDocument struct {
	Filename string            `json:"filename"`
	Sections map[string]string `json:"sections"`
	Markdown string            `json:"markdown"`
}

func mdText(s string) string {
	s = strings.ReplaceAll(strings.ReplaceAll(s, "\r", " "), "\n", " ")
	return strings.NewReplacer("\\", "\\\\", "`", "\\`", "*", "\\*", "_", "\\_", "[", "\\[", "]", "\\]", "<", "&lt;", ">", "&gt;", "#", "\\#", "|", "\\|").Replace(s)
}

func mdCode(s, language string) string {
	longest, current := 0, 0
	for _, ch := range s {
		if ch == '`' {
			current++
			if current > longest {
				longest = current
			}
		} else {
			current = 0
		}
	}
	length := longest + 1
	if length < 3 {
		length = 3
	}
	fence := strings.Repeat("`", length)
	return fence + language + "\n" + s + "\n" + fence
}

func sourceLink(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return "未记录可用的原始变更链接"
	}
	return "[原始变更](<" + strings.NewReplacer("<", "%3C", ">", "%3E", "\n", "", "\r", "").Replace(u.String()) + ">)"
}

var dimensionNames = map[string]string{"business": "业务与需求", "robustness": "健壮性", "reusability": "复用性", "abstraction": "抽象", "encapsulation": "封装", "concurrency": "并发", "security": "安全", "performance": "性能", "testing": "测试", "delivery": "交付"}

// BuildDocument only uses evidence frozen with this run, never current settings.
func BuildDocument(run db.CodeReviewRun) (ReviewDocument, error) {
	doc := ReviewDocument{Filename: reviewFilename(run), Sections: map[string]string{}}
	var report Report
	reportOK := json.Unmarshal([]byte(run.ReportJSON), &report) == nil && strings.TrimSpace(report.Summary) != ""
	hasResult := (run.Status == "completed" || run.Status == "partial") && reportOK
	if !hasResult {
		// A retry or failed run may retain earlier output. Keep its frozen input
		// evidence accessible without presenting stale output as this result.
		report = Report{}
	}
	var snapshot Snapshot
	snapshotOK := json.Unmarshal([]byte(run.SnapshotJSON), &snapshot) == nil && strings.TrimSpace(run.SnapshotJSON) != "null"
	var policy frozenReviewPolicy
	policyOK := json.Unmarshal([]byte(run.PolicyJSON), &policy) == nil && strings.TrimSpace(run.PolicyJSON) != "null"
	var review strings.Builder
	review.WriteString("**评审结论：" + documentConclusion(run, report, snapshot, snapshotOK, hasResult) + "**\n\n")
	if hasResult {
		fmt.Fprintf(&review, "本次记录 %d 项问题与建议。评审状态与源码覆盖范围见事实依据；验证情况见下文。\n\n", len(report.Findings))
	}
	if hasResult && (run.Status == "partial" || !report.EvidenceComplete || !snapshotOK || !snapshot.Complete) {
		review.WriteString("**覆盖限制：证据或覆盖不足，结果仅适用于已读取范围。**\n\n")
	}
	var details strings.Builder
	validation := report.Validation
	if validation == "" {
		validation = "历史记录未保存执行验证信息；不能确认测试、构建、静态分析或外部调用链已验证。"
	}
	details.WriteString("### 验证记录\n\n" + mdText(validation) + "\n\n")
	if len(report.IntentChecks) > 0 {
		details.WriteString("### 变更声明核对\n\n")
		for _, check := range report.IntentChecks {
			fmt.Fprintf(&details, "- **%s**：%s；%s\n", mdText(check.Declaration), mdText(check.Status), mdText(check.Analysis))
			for _, ref := range check.Code {
				fmt.Fprintf(&details, "  - 代码依据：%s:%d\n", mdText(ref.File), ref.Line)
			}
		}
	} else {
		details.WriteString("### 变更声明核对\n\n未保存逐条声明核对，不能确认 MR 描述或提交承诺均已实现。\n\n")
	}
	if len(report.Findings) > 0 {
		review.WriteString("## 问题列表\n\n<table class=\"cr-findings-overview\"><thead><tr><th scope=\"col\">等级</th><th scope=\"col\">问题</th><th scope=\"col\">来源</th></tr></thead><tbody>")
		for _, priority := range []string{"P0", "P1", "P2", "P3", "P4", "P5"} {
			for _, f := range report.Findings {
				if documentPriority(f) != priority {
					continue
				}
				location := htmlText(fmt.Sprintf("%s:%d", f.File, f.Line))
				if target := findingSourceURL(run, snapshot, f); target != "" {
					location = "<a href=\"" + html.EscapeString(target) + "\" target=\"_blank\" rel=\"noopener noreferrer\">" + location + "</a>"
				}
				fmt.Fprintf(&review, "<tr><td><strong>%s</strong></td><td>%s</td><td>%s</td></tr>", priority, htmlText(f.Title), location)
			}
		}
		review.WriteString("</tbody></table>\n\n## 问题详情\n\n")
	}
	for _, priority := range []string{"P0", "P1", "P2", "P3", "P4", "P5"} {
		for _, f := range report.Findings {
			if documentPriority(f) != priority {
				continue
			}
			location := mdText(fmt.Sprintf("%s:%d", f.File, f.Line))
			if target := findingSourceURL(run, snapshot, f); target != "" {
				location = "[" + location + "](<" + target + ">)"
			}
			fmt.Fprintf(&review, "### [%s] %s\n\n%s\n\n**触发与影响**：%s\n\n**修改建议**：%s\n\n**验证建议（未执行）**：%s\n\n", priority, mdText(f.Title), location, mdText(f.Impact), mdText(f.Suggestion), mdText(f.Verification))
		}
	}
	if hasResult && len(report.Findings) == 0 {
		review.WriteString("\n在已读取范围内未发现具备源码证据的问题；不等同于安全或合并批准。\n")
	}
	review.WriteString(details.String())
	review.WriteString("### 待确认与覆盖缺口\n\n")
	gaps := append(append([]string{}, report.Questions...), snapshot.Gaps...)
	if !snapshotOK {
		gaps = append(gaps, "历史代码与知识快照未保存或无法解析。")
	}
	if !policyOK {
		gaps = append(gaps, "历史规则策略快照未保存或无法解析。")
	}
	if len(gaps) == 0 {
		review.WriteString("未记录额外缺口；验证范围仍以上述执行记录为限。\n")
	}
	seen := map[string]bool{}
	for _, gap := range gaps {
		if !seen[gap] {
			fmt.Fprintf(&review, "- %s\n", mdText(gap))
			seen[gap] = true
		}
	}
	doc.Sections["review"] = review.String()

	var facts strings.Builder
	facts.WriteString("## 事实依据\n\n")
	object := "Commit " + run.Ref
	if run.Kind == "mr" {
		object = "MR !" + run.Ref
	}
	for _, row := range [][2]string{{"评审对象", object}, {"标题", run.Title}, {"仓库", run.Repo}, {"Project ID", run.ProjectID}, {"作者", run.Author}, {"记录状态", run.Status}, {"基线 SHA", run.BaseSHA}, {"目标 SHA", run.HeadSHA}, {"模型", run.Model}, {"运行时版本", run.PromptVersion}, {"场景", report.Scenario}} {
		value := row[1]
		if value == "" {
			value = "未记录"
		}
		fmt.Fprintf(&facts, "- **%s**：%s\n", row[0], mdText(value))
	}
	fmt.Fprintf(&facts, "- **接收时间**：%s\n- **结果更新时间**：%s\n- **来源**：%s\n\n### 原始变更声明（不可信资料）\n\n", run.CreatedAt.Format("2006-01-02T15:04:05Z07:00"), run.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"), sourceLink(run.URL))
	if snapshot.Description != "" {
		facts.WriteString(mdCode(snapshot.Description, "text"))
	} else {
		facts.WriteString("未保存 MR 描述或提交说明；不能据此推断需求。")
	}
	facts.WriteString("\n\n### Headroom 输入优化\n\n")
	if len(snapshot.InputOptimization) == 0 {
		facts.WriteString("历史记录未保存逐次调用信息，无法据此确认当时是否使用 Headroom。\n")
	} else {
		names := map[string]string{"compressed": "已压缩", "unchanged": "已调用，输入无需压缩", "below_threshold": "输入低于压缩阈值", "disabled": "模型配置已关闭", "capability_inactive": "Headroom 插件未启用", "custom_generator": "使用测试或自定义生成器，未经过模型客户端"}
		for i, observation := range snapshot.InputOptimization {
			state := names[observation.Reason]
			if state == "" {
				state = "压缩未完成，已保留原文（" + observation.Reason + "）"
			}
			fmt.Fprintf(&facts, "- 调用 %d：%s；输入 %d 字节 → %d 字节。\n", i+1, mdText(state), observation.InputBytes, observation.OutputBytes)
		}
	}
	if hasResult {
		facts.WriteString("### 原始评审摘要\n\n")
		for _, paragraph := range strings.Split(report.Summary, "\n\n") {
			facts.WriteString(mdText(paragraph) + "\n\n")
		}
	}
	doc.Sections["facts"] = facts.String()

	var code strings.Builder
	code.WriteString("## 代码依据\n\n### 问题定位与逐字证据\n\n")
	for _, f := range report.Findings {
		fmt.Fprintf(&code, "### [%s] %s\n\n`%s:%d`\n\n%s\n\n", documentPriority(f), mdText(f.Title), mdText(f.File), f.Line, mdCode(f.Evidence, ""))
	}
	if len(report.Findings) == 0 {
		code.WriteString("未保存问题级源码引用。\n\n")
	}
	code.WriteString("### 文件覆盖与差异\n\n已读取文件与差异不代表已验证外部调用链。问题标题与正文对应。\n\n")
	if len(snapshot.Files) == 0 {
		code.WriteString("未保存文件差异；代码依据不完整。\n")
	}
	for _, file := range snapshot.Files {
		fmt.Fprintf(&code, "#### %s\n\n", mdText(file.NewPath))
		if file.OldPath != file.NewPath && file.OldPath != "" {
			fmt.Fprintf(&code, "原路径：%s\n\n", mdText(file.OldPath))
		}
		if file.Deleted || file.TooLarge || file.Collapsed || file.Diff == "" || (file.Source == "" && !file.Deleted) {
			code.WriteString("覆盖限制：删除、折叠、过大或源码/差异缺失，不能视为完整验证。\n\n")
		}
		if file.Diff != "" {
			code.WriteString(mdCode(file.Diff, "diff") + "\n\n")
		}
	}
	doc.Sections["code"] = code.String()

	var knowledge strings.Builder
	knowledge.WriteString("## 知识依据\n\n知识条目是输入资料；仅明确关联到问题的条目可称为该问题的引用依据。\n\n")
	if len(snapshot.Knowledge) == 0 {
		knowledge.WriteString("本次未保存适用知识；业务规则结论需要人工确认。\n")
	}
	for _, k := range snapshot.Knowledge {
		refs := []string{}
		for _, f := range report.Findings {
			for _, id := range f.KnowledgeIDs {
				if id == k.ID {
					refs = append(refs, fmt.Sprintf("[%s] %s", documentPriority(f), mdText(f.Title)))
					break
				}
			}
		}
		usage := "检索到的背景资料，未被问题明确引用"
		if len(refs) > 0 {
			usage = "问题引用：" + strings.Join(refs, ", ")
		}
		fmt.Fprintf(&knowledge, "### #%d · %s\n\n- 版本：%d；范围：%s\n- 来源：%s\n- 内容摘要：%s\n- 置信度：%.2f；新鲜度：%.2f；更新时间：%s\n- 使用情况：%s\n\n%s\n\n", k.ID, mdText(k.Summary), k.Version, mdText(k.Scope), mdText(k.Source), mdText(k.Hash), k.Confidence, k.Freshness, k.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"), usage, mdCode(k.Content, "text"))
	}
	doc.Sections["knowledge"] = knowledge.String()

	var rules strings.Builder
	rules.WriteString("## 规则依据\n\n仅使用本次保存的策略与技能版本。下列展示结构不证明历史评审已逐条执行规则。\n\n")
	if snapshot.EvidenceContract != nil {
		fmt.Fprintf(&rules, "### 冻结运行时规则\n\n版本：%s；基础运行时提示词 SHA-256（独立复核另加复核指令）：%s\n\n", mdText(snapshot.EvidenceContract.Version), mdText(snapshot.EvidenceContract.PromptHash))
		for _, rule := range snapshot.EvidenceContract.Rules {
			fmt.Fprintf(&rules, "- %s\n", mdText(rule))
		}
	} else {
		rules.WriteString("运行时规则快照未保存，不能用当前默认规则替代当时规则。\n\n")
	}
	fmt.Fprintf(&rules, "\n### 在线评审策略\n\n版本 ID：%d；版本：%d；名称：%s；内容 SHA-256：%s\n\n", run.SkillVersionID, run.SkillVersion, mdText(run.SkillName), mdText(run.SkillHash))
	if policy.Method != nil {
		fmt.Fprintf(&rules, "### 评审方法\n\n%s · 版本 %d（ID %d）· SHA-256：%s\n\n", mdText(policy.Method.Name), policy.Method.Version, policy.Method.VersionID, mdText(policy.Method.Hash))
		rules.WriteString("<details>\n<summary>本次冻结的方法规则全文</summary>\n\n" + mdCode(policy.Method.Instructions, "text") + "\n\n</details>\n\n")
	} else if report.ReviewMethod != nil {
		fmt.Fprintf(&rules, "评审方法元数据：%s v%d · %s；未保存方法全文。\n\n", mdText(report.ReviewMethod.Name), report.ReviewMethod.Version, mdText(report.ReviewMethod.Hash))
	} else {
		rules.WriteString("未保存版本化 merge-review 方法，不能认定本次采用了当前技能规则。\n\n")
	}
	rules.WriteString("### 本次仓库补充规则\n\n")
	if policyOK && strings.TrimSpace(policy.Rules) != "" {
		rules.WriteString(mdCode(policy.Rules, "text"))
	} else if policyOK {
		rules.WriteString("本次未设置补充规则。")
	} else {
		rules.WriteString("历史策略快照缺失。")
	}
	doc.Sections["rules"] = rules.String()
	parts := []string{}
	for _, section := range []string{"review", "facts", "code", "knowledge", "rules"} {
		parts = append(parts, doc.Sections[section])
	}
	doc.Markdown = strings.Join(parts, "\n\n") + "\n"
	return doc, nil
}

func documentPriority(f Finding) string {
	for _, p := range []string{"P0", "P1", "P2", "P3", "P4", "P5"} {
		if f.Priority == p {
			return p
		}
	}
	if p := map[string]string{"critical": "P0", "high": "P1", "medium": "P2", "low": "P3", "P0": "P0", "P1": "P1", "P2": "P2", "P3": "P3", "P4": "P4", "P5": "P5"}[f.Severity]; p != "" {
		return p
	}
	return "P2"
}

func documentConclusion(run db.CodeReviewRun, report Report, snapshot Snapshot, snapshotOK, hasResult bool) string {
	if !hasResult {
		return "当前记录没有可用的评审结论，不能据此判断是否可以合并。"
	}
	for _, f := range report.Findings {
		if p := documentPriority(f); p == "P0" || p == "P1" {
			return "存在阻塞问题，建议修复后再合并。"
		}
	}
	if run.Status == "partial" || !report.EvidenceComplete || !snapshotOK || !snapshot.Complete {
		return "证据或覆盖不足，暂不能确认可以合并。"
	}
	return "已读取范围内未发现阻塞问题。"
}

var unsafeFilename = regexp.MustCompile(`[<>:"/\\|?*\x00-\x1f\x7f]+`)

func filenamePart(s, fallback string) string {
	s = unsafeFilename.ReplaceAllString(strings.TrimSpace(s), "-")
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return -1
		}
		return r
	}, s)
	s = strings.Trim(s, " ._-")
	if s == "" {
		s = fallback
	}
	// Bound each component in bytes without cutting UTF-8 characters.
	for len(s) > 72 {
		_, n := utf8.DecodeLastRuneInString(s)
		s = s[:len(s)-n]
	}
	return s
}

func reviewFilename(run db.CodeReviewRun) string {
	var snapshot Snapshot
	_ = json.Unmarshal([]byte(run.SnapshotJSON), &snapshot)
	repo := strings.TrimSpace(snapshot.RepositorySource.FullPath)
	if repo == "" {
		repo = strings.TrimSpace(run.Repo)
	}
	repo = path.Base(strings.TrimRight(strings.ReplaceAll(repo, "\\", "/"), "/"))
	if repo == "." {
		repo = ""
	}
	branch := snapshot.SourceBranch
	ref := "mr" + run.Ref
	if run.Kind != "mr" {
		ref = "commit-" + run.Ref
		if len(ref) > 19 {
			ref = ref[:19]
		}
	}
	date := "未记录日期"
	if !run.CreatedAt.IsZero() {
		date = run.CreatedAt.In(time.FixedZone("UTC+8", 8*3600)).Format("2006-01-02")
	}
	return filenamePart(repo, "未知仓库") + "_" + filenamePart(branch, "未记录分支") + "_" + filenamePart(ref, "未记录版本") + "_" + date + ".md"
}

func htmlText(s string) string {
	return html.EscapeString(strings.ReplaceAll(strings.ReplaceAll(s, "\r", " "), "\n", " "))
}

// Source links resolve against the frozen repository and code version.
func findingSourceURL(run db.CodeReviewRun, snapshot Snapshot, f Finding) string {
	source := snapshot.RepositorySource
	if source.WebURL == "" {
		_ = json.Unmarshal([]byte(run.RepositorySourceJSON), &source)
	}
	root := source.WebURL
	if root == "" {
		raw := snapshot.URL
		if raw == "" {
			raw = run.URL
		}
		for _, marker := range []string{"/-/merge_requests/", "/-/commit/", "/merge_requests/", "/commit/"} {
			if prefix, _, ok := strings.Cut(raw, marker); ok {
				root = prefix
				break
			}
		}
	}
	head := snapshot.Head
	if head == "" {
		head = run.HeadSHA
	}
	u, err := url.Parse(root)
	if err != nil || u.Host == "" || u.User != nil || (u.Scheme != "https" && u.Scheme != "http") || !shaPattern.MatchString(head) || f.Line < 1 || f.File == "" || strings.HasPrefix(f.File, "/") {
		return ""
	}
	for _, part := range strings.Split(f.File, "/") {
		if part == ".." || part == "." || part == "" {
			return ""
		}
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/-/blob/" + head + "/" + f.File
	u.RawPath = ""
	u.RawQuery = ""
	u.Fragment = fmt.Sprintf("L%d", f.Line)
	return u.String()
}
