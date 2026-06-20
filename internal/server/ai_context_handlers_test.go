package server

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractAIContextFeaturesFromRows(t *testing.T) {
	rows := [][]string{
		{"文本", "功能类型", "状态", "是否可配置", "应用现场项目", "文本 3"},
		{"全局路径规划", "领航能力", "已实现", "是", "研发协同", "支持跨模块路径拆解"},
		{"红区卡点诊断", "决策能力", "规划中", "否", "决策面板", "识别风险卡点"},
	}

	features := extractAIContextFeatures(rows)
	if len(features) != 2 {
		t.Fatalf("expected 2 features, got %d", len(features))
	}
	if features[0].Feature != "全局路径规划" {
		t.Fatalf("unexpected feature name: %q", features[0].Feature)
	}
	if features[0].Type != "领航能力" {
		t.Fatalf("unexpected type: %q", features[0].Type)
	}
	if !isConfigurableValue(features[0].Configurable) {
		t.Fatalf("expected first feature to be configurable")
	}

	statusBreakdown, typeBreakdown, configurableCount := summarizeAIContextFeatures(features)
	if statusBreakdown["已实现"] != 1 || statusBreakdown["规划中"] != 1 {
		t.Fatalf("unexpected status breakdown: %#v", statusBreakdown)
	}
	if typeBreakdown["领航能力"] != 1 || typeBreakdown["决策能力"] != 1 {
		t.Fatalf("unexpected type breakdown: %#v", typeBreakdown)
	}
	if configurableCount != 1 {
		t.Fatalf("expected one configurable feature, got %d", configurableCount)
	}
}

func TestDiffAIContextFeatures(t *testing.T) {
	before := []AIContextFeature{
		{Feature: "能力 A", Status: "已实现"},
		{Feature: "能力 B", Status: "规划中"},
	}
	after := []AIContextFeature{
		{Feature: "能力 A", Status: "已实现"},
		{Feature: "能力 B", Status: "已实现"},
		{Feature: "能力 C", Status: "已实现"},
	}

	diff := diffAIContextFeatures(before, after)
	if diff.AddedCount != 1 || diff.Added[0] != "能力 C" {
		t.Fatalf("unexpected added diff: %#v", diff)
	}
	if diff.RemovedCount != 0 {
		t.Fatalf("unexpected removed diff: %#v", diff)
	}
	if diff.ChangedCount != 1 || diff.Changed[0] != "能力 B" {
		t.Fatalf("unexpected changed diff: %#v", diff)
	}
}

func TestParseUploadedPathPlanningWorkbookIfPresent(t *testing.T) {
	workbookPath := filepath.Join("..", "..", "全局领航能力汇总.xlsx")
	data, err := os.ReadFile(workbookPath)
	if os.IsNotExist(err) {
		t.Skip("uploaded path planning workbook is not present")
	}
	if err != nil {
		t.Fatalf("read workbook: %v", err)
	}

	sheetName, rows, err := parseAIContextSpreadsheet(filepath.Base(workbookPath), data)
	if err != nil {
		t.Fatalf("parse workbook: %v", err)
	}
	features := extractAIContextFeatures(rows)
	if sheetName == "" {
		t.Fatalf("expected first sheet name")
	}
	if len(features) == 0 {
		t.Fatalf("expected uploaded workbook to yield module features")
	}
	t.Logf("parsed sheet %q with %d feature rows", sheetName, len(features))
}
