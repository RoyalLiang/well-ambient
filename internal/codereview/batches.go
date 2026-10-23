package codereview

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"well-ambient/internal/config"
	"well-ambient/internal/llm"
)

type InputOptimization struct {
	Reason      string `json:"reason"`
	InputBytes  int    `json:"input_bytes"`
	OutputBytes int    `json:"output_bytes"`
}

func evidenceAtLine(lines []string, line int, evidence string) bool {
	if line < 1 || line > len(lines) {
		return false
	}
	evidence = strings.TrimSpace(evidence)
	if evidence == "" {
		return false
	}
	if !strings.Contains(evidence, "\n") {
		return strings.Contains(lines[line-1], evidence)
	}
	// The excerpt may start partway through the claimed line, but never on a
	// later line (including after blank lines).
	offset := strings.Index(strings.Join(lines[line-1:], "\n"), evidence)
	return offset >= 0 && offset < len(lines[line-1])
}

// Preserve complete files and absolute line numbers across bounded model calls.
func reviewBatches(snapshot Snapshot) []Snapshot {
	const sourceBudget = 120000
	batches := []Snapshot{}
	current := snapshot
	current.Files = nil
	bytes := 0
	for _, file := range snapshot.Files {
		size := len(encode(file))
		if len(current.Files) > 0 && bytes+size > sourceBudget {
			batches = append(batches, current)
			current.Files = nil
			bytes = 0
		}
		current.Files = append(current.Files, file)
		bytes += size
	}
	if len(current.Files) > 0 || len(batches) == 0 {
		batches = append(batches, current)
	}
	return batches
}

func (s *Service) generateObserved(ctx context.Context, ai config.AIConfig, system, user string, observations *[]InputOptimization) (string, error) {
	observe := func(reason string, before, after int) {
		if observations != nil {
			*observations = append(*observations, InputOptimization{reason, before, after})
		}
	}
	if s.Generate != nil {
		observe("custom_generator", len(user), len(user))
		return s.Generate(ctx, system, user)
	}
	c := llm.Client{Config: ai}
	return c.Generate(ctx, llm.Request{SystemPrompt: system, UserPrompt: user, MaxOutputTokens: 10000, OnInputOptimization: observe})
}

func (s *Service) reviewSnapshot(ctx context.Context, ai config.AIConfig, prompt string, snapshot *Snapshot) (Report, error) {
	batches := reviewBatches(*snapshot)
	combined := Report{EvidenceComplete: true, Findings: []Finding{}, Questions: []string{}}
	summaries := []string{}
	seenSummaries := map[string]bool{}
	for i, batch := range batches {
		if err := ctx.Err(); err != nil {
			return Report{}, err
		}
		input := encode(batch)
		batchPrompt := prompt
		if len(batches) > 1 {
			batchPrompt += fmt.Sprintf("\n这是第 %d/%d 批，仅包含本批变更文件。不得把其他批次未出现在输入中的实现判为缺失；跨批次依赖只能列为待确认。", i+1, len(batches))
		}
		raw, err := s.generateObserved(ctx, ai, batchPrompt, input, &snapshot.InputOptimization)
		if err != nil {
			if ctx.Err() != nil {
				return Report{}, fmt.Errorf("第 %d/%d 批 AI 初审中断：%w", i+1, len(batches), ctx.Err())
			}
			return Report{}, fmt.Errorf("第 %d/%d 批 AI 初审失败，请检查模型服务后重试", i+1, len(batches))
		}
		draft, err := parseReport(raw, batch)
		if err != nil {
			return Report{}, fmt.Errorf("第 %d/%d 批初审：%w", i+1, len(batches), err)
		}
		raw, err = s.generateObserved(ctx, ai, batchPrompt+"\n本轮是独立复核。先从代码重新判断，再核对初审。主动寻找反证，删除不成立或已经被其他代码保护的问题；不能把初审当作证据。输出相同 JSON 结构。", input+"\n<untrusted_draft>"+encode(draft)+"</untrusted_draft>", &snapshot.InputOptimization)
		if err != nil {
			if ctx.Err() != nil {
				return Report{}, fmt.Errorf("第 %d/%d 批 AI 复核中断：%w", i+1, len(batches), ctx.Err())
			}
			return Report{}, fmt.Errorf("第 %d/%d 批 AI 复核失败，请重新评审", i+1, len(batches))
		}
		report, err := parseReport(raw, batch)
		if err != nil {
			return Report{}, fmt.Errorf("第 %d/%d 批复核：%w", i+1, len(batches), err)
		}
		combined.EvidenceComplete = combined.EvidenceComplete && report.EvidenceComplete
		combined.Findings = append(combined.Findings, report.Findings...)
		combined.IntentChecks = append(combined.IntentChecks, report.IntentChecks...)
		combined.Questions = append(combined.Questions, report.Questions...)
		if !seenSummaries[report.Summary] {
			summaries = append(summaries, report.Summary)
			seenSummaries[report.Summary] = true
		}
		combined.Validation = report.Validation
	}
	combined.Summary = strings.Join(summaries, "\n\n")
	priority := map[string]int{"P0": 0, "P1": 1, "P2": 2, "P3": 3}
	sort.SliceStable(combined.Findings, func(i, j int) bool {
		return priority[combined.Findings[i].Priority] < priority[combined.Findings[j].Priority]
	})
	if len(batches) > 1 {
		combined.Validation += fmt.Sprintf(" 分 %d 批完成 %d 个文件的两轮分析；未验证跨批次调用链。", len(batches), len(snapshot.Files))
	}
	return combined, nil
}
