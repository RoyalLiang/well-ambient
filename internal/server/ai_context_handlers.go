package server

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"well-ambient/internal/db"

	"gorm.io/gorm"
)

const maxAIContextUploadBytes = 16 << 20

const (
	implementationImplemented    = "已实现"
	implementationPartial        = "部分可用"
	implementationNotImplemented = "未实现"
	implementationUnknown        = "未标注"
)

type AIContextFeature struct {
	Feature      string `json:"feature"`
	Type         string `json:"type"`
	Status       string `json:"status"`
	Configurable string `json:"configurable"`
	Scenario     string `json:"scenario"`
	Description  string `json:"description"`
}

type AIContextDiff struct {
	AgainstID      uint     `json:"against_id,omitempty"`
	AgainstVersion int      `json:"against_version,omitempty"`
	Added          []string `json:"added"`
	Removed        []string `json:"removed"`
	Changed        []string `json:"changed"`
	AddedCount     int      `json:"added_count"`
	RemovedCount   int      `json:"removed_count"`
	ChangedCount   int      `json:"changed_count"`
	Summary        string   `json:"summary"`
}

type AIContextProfileDTO struct {
	ID                      uint               `json:"id"`
	ModuleName              string             `json:"module_name"`
	SourceFilename          string             `json:"source_filename"`
	SourceSheet             string             `json:"source_sheet"`
	Summary                 string             `json:"summary"`
	PromptSummary           string             `json:"prompt_summary"`
	FeatureCount            int                `json:"feature_count"`
	ImplementedCount        int                `json:"implemented_count"`
	PartialCount            int                `json:"partial_count"`
	NotImplementedCount     int                `json:"not_implemented_count"`
	UnknownCount            int                `json:"unknown_count"`
	ConfigurableCount       int                `json:"configurable_count"`
	NonConfigurableCount    int                `json:"non_configurable_count"`
	ImplementationBreakdown map[string]int     `json:"implementation_breakdown"`
	StatusBreakdown         map[string]int     `json:"status_breakdown"`
	TypeBreakdown           map[string]int     `json:"type_breakdown"`
	FeatureSnapshot         []AIContextFeature `json:"feature_snapshot"`
	Enabled                 bool               `json:"enabled"`
	Version                 int                `json:"version"`
	Diff                    *AIContextDiff     `json:"diff,omitempty"`
	CreatedAt               time.Time          `json:"created_at"`
	UpdatedAt               time.Time          `json:"updated_at"`
}

type worksheetTextRun struct {
	Text string `xml:"t"`
}

type worksheetInlineString struct {
	Text string             `xml:"t"`
	Runs []worksheetTextRun `xml:"r"`
}

type aiContextImportResponse struct {
	Success bool                `json:"success"`
	Message string              `json:"message"`
	Profile AIContextProfileDTO `json:"profile"`
	Diff    AIContextDiff       `json:"diff"`
}

func (s *Server) handleListAIContextProfiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	if db.DB == nil {
		http.Error(w, "Database not initialized", http.StatusInternalServerError)
		return
	}

	var profiles []db.AIContextProfile
	if err := db.DB.Order("module_name asc, version desc, created_at desc").Find(&profiles).Error; err != nil {
		http.Error(w, fmt.Sprintf("Failed to query AI context profiles: %v", err), http.StatusInternalServerError)
		return
	}

	dtos := make([]AIContextProfileDTO, 0, len(profiles))
	for _, profile := range profiles {
		dto := aiContextProfileDTO(profile)
		if previous, ok := findPreviousAIContextProfile(profile); ok {
			diff := diffAIContextProfiles(previous, profile)
			dto.Diff = &diff
		}
		dtos = append(dtos, dto)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(dtos); err != nil {
		log.Printf("Error encoding AI context profiles: %v", err)
	}
}

func (s *Server) handleImportAIContext(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	if db.DB == nil {
		http.Error(w, "Database not initialized", http.StatusInternalServerError)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxAIContextUploadBytes)
	if err := r.ParseMultipartForm(maxAIContextUploadBytes); err != nil {
		http.Error(w, fmt.Sprintf("Bad Request: failed to parse upload: %v", err), http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Bad Request: missing upload file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	fileBytes, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to read upload file: %v", err), http.StatusBadRequest)
		return
	}
	if len(fileBytes) == 0 {
		http.Error(w, "Bad Request: upload file is empty", http.StatusBadRequest)
		return
	}

	sourceSheet, rows, err := parseAIContextSpreadsheet(header.Filename, fileBytes)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to parse spreadsheet: %v", err), http.StatusBadRequest)
		return
	}

	moduleName := strings.TrimSpace(r.FormValue("module_name"))
	if moduleName == "" {
		moduleName = deriveModuleNameFromFilename(header.Filename)
	}

	features := extractAIContextFeatures(rows)
	if len(features) == 0 {
		http.Error(w, "Bad Request: no usable feature rows found in first sheet", http.StatusBadRequest)
		return
	}

	var previous db.AIContextProfile
	previousFound := false
	if err := db.DB.Where("module_name = ?", moduleName).Order("version desc, created_at desc").First(&previous).Error; err == nil {
		previousFound = true
	} else if err != nil && err != gorm.ErrRecordNotFound {
		http.Error(w, fmt.Sprintf("Failed to query previous profile: %v", err), http.StatusInternalServerError)
		return
	}

	nextVersion := 1
	if previousFound {
		nextVersion = previous.Version + 1
	}

	statusBreakdown, typeBreakdown, configurableCount := summarizeAIContextFeatures(features)
	snapshotJSON, _ := json.Marshal(features)
	statusJSON, _ := json.Marshal(statusBreakdown)
	typeJSON, _ := json.Marshal(typeBreakdown)
	summary := buildAIContextSummary(moduleName, header.Filename, sourceSheet, nextVersion, features, statusBreakdown, typeBreakdown, configurableCount)
	promptSummary := buildAIContextPromptSummary(moduleName, header.Filename, sourceSheet, nextVersion, features, statusBreakdown, typeBreakdown, configurableCount)

	now := time.Now()
	profile := db.AIContextProfile{
		ModuleName:           moduleName,
		SourceFilename:       header.Filename,
		SourceSheet:          sourceSheet,
		Summary:              summary,
		PromptSummary:        promptSummary,
		FeatureCount:         len(features),
		ConfigurableCount:    configurableCount,
		NonConfigurableCount: len(features) - configurableCount,
		StatusBreakdownJSON:  string(statusJSON),
		TypeBreakdownJSON:    string(typeJSON),
		FeatureSnapshotJSON:  string(snapshotJSON),
		Enabled:              true,
		Version:              nextVersion,
		CreatedAt:            now,
		UpdatedAt:            now,
	}

	tx := db.DB.Begin()
	if tx.Error != nil {
		http.Error(w, fmt.Sprintf("Failed to start transaction: %v", tx.Error), http.StatusInternalServerError)
		return
	}
	if err := tx.Model(&db.AIContextProfile{}).Where("module_name = ?", moduleName).Update("enabled", false).Error; err != nil {
		tx.Rollback()
		http.Error(w, fmt.Sprintf("Failed to disable previous module contexts: %v", err), http.StatusInternalServerError)
		return
	}
	if err := tx.Create(&profile).Error; err != nil {
		tx.Rollback()
		http.Error(w, fmt.Sprintf("Failed to save AI context profile: %v", err), http.StatusInternalServerError)
		return
	}
	if err := tx.Commit().Error; err != nil {
		http.Error(w, fmt.Sprintf("Failed to commit AI context profile: %v", err), http.StatusInternalServerError)
		return
	}

	diff := AIContextDiff{
		Added:   featureNames(features),
		Summary: "首次导入，已生成模块上下文画像。",
	}
	if previousFound {
		diff = diffAIContextProfiles(previous, profile)
	}
	diff.AddedCount = len(diff.Added)
	diff.RemovedCount = len(diff.Removed)
	diff.ChangedCount = len(diff.Changed)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(aiContextImportResponse{
		Success: true,
		Message: "AI context profile imported and enabled",
		Profile: aiContextProfileDTO(profile),
		Diff:    diff,
	}); err != nil {
		log.Printf("Error encoding AI context import response: %v", err)
	}
}

func (s *Server) handleToggleAIContextProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	if db.DB == nil {
		http.Error(w, "Database not initialized", http.StatusInternalServerError)
		return
	}

	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		http.Error(w, "Bad Request: invalid profile id", http.StatusBadRequest)
		return
	}

	var payload struct {
		Enabled *bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && err != io.EOF {
		http.Error(w, fmt.Sprintf("Bad Request: %v", err), http.StatusBadRequest)
		return
	}

	var profile db.AIContextProfile
	if err := db.DB.First(&profile, id).Error; err != nil {
		http.Error(w, "AI context profile not found", http.StatusNotFound)
		return
	}

	nextEnabled := !profile.Enabled
	if payload.Enabled != nil {
		nextEnabled = *payload.Enabled
	}

	tx := db.DB.Begin()
	if tx.Error != nil {
		http.Error(w, fmt.Sprintf("Failed to start transaction: %v", tx.Error), http.StatusInternalServerError)
		return
	}
	if nextEnabled {
		if err := tx.Model(&db.AIContextProfile{}).
			Where("module_name = ? AND id <> ?", profile.ModuleName, profile.ID).
			Update("enabled", false).Error; err != nil {
			tx.Rollback()
			http.Error(w, fmt.Sprintf("Failed to disable older module versions: %v", err), http.StatusInternalServerError)
			return
		}
	}
	profile.Enabled = nextEnabled
	profile.UpdatedAt = time.Now()
	if err := tx.Save(&profile).Error; err != nil {
		tx.Rollback()
		http.Error(w, fmt.Sprintf("Failed to update AI context profile: %v", err), http.StatusInternalServerError)
		return
	}
	if err := tx.Commit().Error; err != nil {
		http.Error(w, fmt.Sprintf("Failed to commit AI context toggle: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"profile": aiContextProfileDTO(profile),
	}); err != nil {
		log.Printf("Error encoding AI context toggle response: %v", err)
	}
}

func loadEnabledAIContextPrompt(limit int) string {
	if db.DB == nil {
		return ""
	}
	if limit <= 0 {
		limit = 8
	}

	var profiles []db.AIContextProfile
	if err := db.DB.Where("enabled = ?", true).Order("module_name asc, version desc").Limit(limit).Find(&profiles).Error; err != nil {
		log.Printf("Failed to load enabled AI context profiles: %v", err)
		return ""
	}

	blocks := make([]string, 0, len(profiles))
	for _, profile := range profiles {
		if trimmed := strings.TrimSpace(profile.PromptSummary); trimmed != "" {
			blocks = append(blocks, trimmed)
		}
	}
	return strings.Join(blocks, "\n\n---\n\n")
}

func parseAIContextSpreadsheet(filename string, data []byte) (string, [][]string, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".csv":
		reader := csv.NewReader(bytes.NewReader(data))
		reader.FieldsPerRecord = -1
		rows, err := reader.ReadAll()
		return "CSV", normalizeSheetRows(rows), err
	case ".xlsx":
		return parseXLSXFirstSheet(data)
	default:
		return "", nil, fmt.Errorf("unsupported file type %q, please upload .xlsx or .csv", ext)
	}
}

func parseXLSXFirstSheet(data []byte) (string, [][]string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", nil, err
	}

	files := make(map[string][]byte)
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			return "", nil, err
		}
		content, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return "", nil, err
		}
		files[f.Name] = content
	}

	type workbookSheet struct {
		Name string `xml:"name,attr"`
		RID  string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr"`
	}
	type workbookXML struct {
		Sheets []workbookSheet `xml:"sheets>sheet"`
	}
	var workbook workbookXML
	if content, ok := files["xl/workbook.xml"]; ok {
		if err := xml.Unmarshal(content, &workbook); err != nil {
			return "", nil, fmt.Errorf("parse workbook.xml: %w", err)
		}
	}

	sheetName := "Sheet1"
	sheetPath := "xl/worksheets/sheet1.xml"
	if len(workbook.Sheets) > 0 {
		sheetName = workbook.Sheets[0].Name
		if resolved := resolveWorksheetPath(files, workbook.Sheets[0].RID); resolved != "" {
			sheetPath = resolved
		}
	}

	sheetXML, ok := files[sheetPath]
	if !ok {
		return "", nil, fmt.Errorf("worksheet %s not found", sheetPath)
	}

	sharedStrings := parseSharedStrings(files["xl/sharedStrings.xml"])
	rows, err := parseWorksheetRows(sheetXML, sharedStrings)
	if err != nil {
		return "", nil, err
	}
	return sheetName, normalizeSheetRows(rows), nil
}

func resolveWorksheetPath(files map[string][]byte, relationshipID string) string {
	if relationshipID == "" {
		return ""
	}
	type relationship struct {
		ID     string `xml:"Id,attr"`
		Target string `xml:"Target,attr"`
	}
	type relationshipsXML struct {
		Relationships []relationship `xml:"Relationship"`
	}
	var rels relationshipsXML
	if content, ok := files["xl/_rels/workbook.xml.rels"]; ok {
		if err := xml.Unmarshal(content, &rels); err != nil {
			return ""
		}
	}
	for _, rel := range rels.Relationships {
		if rel.ID == relationshipID {
			target := strings.TrimPrefix(rel.Target, "/")
			if strings.HasPrefix(target, "xl/") {
				return path.Clean(target)
			}
			return path.Clean(path.Join("xl", target))
		}
	}
	return ""
}

func parseSharedStrings(data []byte) []string {
	if len(data) == 0 {
		return nil
	}
	type textRun struct {
		Text string `xml:"t"`
	}
	type sharedItem struct {
		Text string    `xml:"t"`
		Runs []textRun `xml:"r"`
	}
	type sharedTable struct {
		Items []sharedItem `xml:"si"`
	}
	var table sharedTable
	if err := xml.Unmarshal(data, &table); err != nil {
		return nil
	}
	values := make([]string, 0, len(table.Items))
	for _, item := range table.Items {
		if item.Text != "" {
			values = append(values, item.Text)
			continue
		}
		var builder strings.Builder
		for _, run := range item.Runs {
			builder.WriteString(run.Text)
		}
		values = append(values, builder.String())
	}
	return values
}

func parseWorksheetRows(data []byte, sharedStrings []string) ([][]string, error) {
	type sheetCell struct {
		Ref    string                `xml:"r,attr"`
		Type   string                `xml:"t,attr"`
		Value  string                `xml:"v"`
		Inline worksheetInlineString `xml:"is"`
	}
	type sheetRow struct {
		Cells []sheetCell `xml:"c"`
	}
	type worksheetXML struct {
		Rows []sheetRow `xml:"sheetData>row"`
	}

	var worksheet worksheetXML
	if err := xml.Unmarshal(data, &worksheet); err != nil {
		return nil, fmt.Errorf("parse worksheet XML: %w", err)
	}

	rows := make([][]string, 0, len(worksheet.Rows))
	for _, row := range worksheet.Rows {
		values := []string{}
		for fallbackIndex, cell := range row.Cells {
			columnIndex := cellRefColumnIndex(cell.Ref)
			if columnIndex < 0 {
				columnIndex = fallbackIndex
			}
			for len(values) <= columnIndex {
				values = append(values, "")
			}
			values[columnIndex] = sheetCellValue(cell.Type, cell.Value, cell.Inline, sharedStrings)
		}
		rows = append(rows, values)
	}
	return rows, nil
}

func sheetCellValue(cellType string, raw string, inline worksheetInlineString, sharedStrings []string) string {
	if cellType == "s" {
		idx, err := strconv.Atoi(raw)
		if err == nil && idx >= 0 && idx < len(sharedStrings) {
			return strings.TrimSpace(sharedStrings[idx])
		}
		return ""
	}
	if cellType == "inlineStr" {
		if inline.Text != "" {
			return strings.TrimSpace(inline.Text)
		}
		var builder strings.Builder
		for _, run := range inline.Runs {
			builder.WriteString(run.Text)
		}
		return strings.TrimSpace(builder.String())
	}
	return strings.TrimSpace(raw)
}

func cellRefColumnIndex(ref string) int {
	if ref == "" {
		return -1
	}
	col := 0
	seen := false
	for _, r := range ref {
		if r < 'A' || r > 'Z' {
			if r >= 'a' && r <= 'z' {
				r -= 'a' - 'A'
			} else {
				break
			}
		}
		seen = true
		col = col*26 + int(r-'A'+1)
	}
	if !seen {
		return -1
	}
	return col - 1
}

func normalizeSheetRows(rows [][]string) [][]string {
	maxCols := 0
	trimmedRows := make([][]string, 0, len(rows))
	for _, row := range rows {
		clean := make([]string, len(row))
		hasValue := false
		for i, value := range row {
			clean[i] = strings.TrimSpace(value)
			if clean[i] != "" {
				hasValue = true
			}
		}
		if !hasValue {
			continue
		}
		for len(clean) > 0 && clean[len(clean)-1] == "" {
			clean = clean[:len(clean)-1]
		}
		if len(clean) > maxCols {
			maxCols = len(clean)
		}
		trimmedRows = append(trimmedRows, clean)
	}
	for i := range trimmedRows {
		for len(trimmedRows[i]) < maxCols {
			trimmedRows[i] = append(trimmedRows[i], "")
		}
	}
	return trimmedRows
}

func extractAIContextFeatures(rows [][]string) []AIContextFeature {
	if len(rows) == 0 {
		return nil
	}
	headerIndex := detectHeaderRow(rows)
	headers := []string{}
	if headerIndex >= 0 {
		headers = rows[headerIndex]
	}
	headerMap := buildAIContextHeaderMap(headers)
	startIndex := headerIndex + 1
	if headerIndex < 0 {
		startIndex = 0
	}

	featureCol := pickColumn(headerMap, []string{"feature", "name", "title"}, 0)
	typeCol := pickColumn(headerMap, []string{"type", "category"}, 1)
	statusCol := pickColumn(headerMap, []string{"status", "state"}, 2)
	configurableCol := pickColumn(headerMap, []string{"configurable"}, 3)
	scenarioCol := pickColumn(headerMap, []string{"scenario", "project", "scene"}, 4)
	descriptionCol := pickColumn(headerMap, []string{"description", "remark", "detail"}, maxInt(0, len(headers)-1))

	features := []AIContextFeature{}
	seen := map[string]bool{}
	for _, row := range rows[startIndex:] {
		feature := cellAt(row, featureCol)
		if feature == "" && len(row) > 0 {
			feature = cellAt(row, 0)
		}
		feature = strings.TrimSpace(feature)
		if feature == "" {
			continue
		}
		key := featureMapKey(feature)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		features = append(features, AIContextFeature{
			Feature:      feature,
			Type:         cellAt(row, typeCol),
			Status:       cellAt(row, statusCol),
			Configurable: cellAt(row, configurableCol),
			Scenario:     cellAt(row, scenarioCol),
			Description:  cellAt(row, descriptionCol),
		})
	}
	return features
}

func detectHeaderRow(rows [][]string) int {
	bestIndex := -1
	bestScore := 0
	for i, row := range rows {
		if i > 8 {
			break
		}
		score := 0
		for _, cell := range row {
			normalized := normalizeHeader(cell)
			switch {
			case strings.Contains(normalized, "功能"), strings.Contains(normalized, "能力"), strings.Contains(normalized, "需求"):
				score += 2
			case strings.Contains(normalized, "状态"), strings.Contains(normalized, "类型"), strings.Contains(normalized, "可配置"):
				score += 2
			case strings.Contains(normalized, "场景"), strings.Contains(normalized, "描述"), strings.Contains(normalized, "说明"):
				score++
			}
		}
		if score > bestScore {
			bestScore = score
			bestIndex = i
		}
	}
	if bestScore >= 3 {
		return bestIndex
	}
	return -1
}

func buildAIContextHeaderMap(headers []string) map[string]int {
	result := map[string]int{}
	for index, header := range headers {
		normalized := normalizeHeader(header)
		if normalized == "" {
			continue
		}
		switch {
		case containsAny(normalized, "功能类型", "类型", "类别", "分类"):
			setColumnIfMissing(result, "type", index)
		case containsAny(normalized, "实现状态", "当前状态", "状态"):
			setColumnIfMissing(result, "status", index)
		case containsAny(normalized, "是否可配置", "可配置", "配置项", "配置"):
			setColumnIfMissing(result, "configurable", index)
		case containsAny(normalized, "应用现场项目", "验证场景", "业务场景", "适用场景", "场景", "现场项目"):
			setColumnIfMissing(result, "scenario", index)
		case containsAny(normalized, "功能说明", "能力说明", "描述", "说明", "备注", "详情", "文本3"):
			setColumnIfMissing(result, "description", index)
		case containsAny(normalized, "功能名称", "功能点", "功能", "能力名称", "能力", "文本"):
			setColumnIfMissing(result, "feature", index)
		}
	}
	return result
}

func summarizeAIContextFeatures(features []AIContextFeature) (map[string]int, map[string]int, int) {
	statusBreakdown := map[string]int{}
	typeBreakdown := map[string]int{}
	configurableCount := 0
	for _, feature := range features {
		status := normalizedBucket(feature.Status, "未标注状态")
		featureType := normalizedBucket(feature.Type, "未标注类型")
		statusBreakdown[status]++
		typeBreakdown[featureType]++
		if isConfigurableValue(feature.Configurable) {
			configurableCount++
		}
	}
	return statusBreakdown, typeBreakdown, configurableCount
}

func summarizeImplementationFeatures(features []AIContextFeature) (map[string]int, int, int, int, int) {
	breakdown := map[string]int{
		implementationImplemented:    0,
		implementationPartial:        0,
		implementationNotImplemented: 0,
		implementationUnknown:        0,
	}
	for _, feature := range features {
		breakdown[implementationBucket(feature.Status)]++
	}
	return breakdown,
		breakdown[implementationImplemented],
		breakdown[implementationPartial],
		breakdown[implementationNotImplemented],
		breakdown[implementationUnknown]
}

func implementationBucket(status string) string {
	normalized := strings.ToLower(strings.TrimSpace(status))
	normalized = strings.NewReplacer(" ", "", "_", "", "-", "", "/", "").Replace(normalized)
	switch {
	case normalized == "":
		return implementationUnknown
	case containsAny(normalized, "未标注", "未知", "不明确", "待确认", "unknown", "na", "n/a"):
		return implementationUnknown
	case containsAny(normalized, "未实现", "未开发", "待开发", "不可用", "规划中", "计划中", "backlog", "planned", "todo", "notimplemented", "notstarted"):
		return implementationNotImplemented
	case containsAny(normalized, "部分", "开发中", "进行中", "验证中", "待测试", "灰度", "已初步验证", "初步验证", "partial", "inprogress", "testing", "beta", "wip"):
		return implementationPartial
	case containsAny(normalized, "已实现", "已耐久", "已完成", "开发完成", "已开发完成", "已上线", "已发布", "可用", "done", "implemented", "complete", "completed", "production", "ga", "available"):
		return implementationImplemented
	default:
		return implementationUnknown
	}
}

func buildAIContextSummary(moduleName string, filename string, sheetName string, version int, features []AIContextFeature, statusBreakdown map[string]int, typeBreakdown map[string]int, configurableCount int) string {
	_, implementedCount, partialCount, notImplementedCount, unknownCount := summarizeImplementationFeatures(features)
	return fmt.Sprintf("%s v%d：沉淀 %d 项模块能力实现状态；已实现 %d 项，部分可用 %d 项，未实现 %d 项，未标注 %d 项。来源仅作为摄取记录：%s/%s。",
		moduleName,
		version,
		len(features),
		implementedCount,
		partialCount,
		notImplementedCount,
		unknownCount,
		filename,
		sheetName,
	)
}

func buildAIContextPromptSummary(moduleName string, filename string, sheetName string, version int, features []AIContextFeature, statusBreakdown map[string]int, typeBreakdown map[string]int, configurableCount int) string {
	var builder strings.Builder
	implementationBreakdown, implementedCount, partialCount, notImplementedCount, unknownCount := summarizeImplementationFeatures(features)
	builder.WriteString(fmt.Sprintf("模块能力实现画像：%s v%d\n", moduleName, version))
	builder.WriteString("用途：仅用于判断需求是否命中已实现能力，以及由此校准工时；不要把来源文件中的类型、场景或备注当作完整系统架构。\n")
	builder.WriteString(fmt.Sprintf("实现状态：共 %d 项；已实现 %d 项；部分可用 %d 项；未实现 %d 项；未标注 %d 项。\n",
		len(features), implementedCount, partialCount, notImplementedCount, unknownCount))
	builder.WriteString(fmt.Sprintf("状态分布：%s。\n", formatImplementationBreakdown(implementationBreakdown)))
	builder.WriteString(fmt.Sprintf("已实现能力：%s。\n", formatFeatureNamesByImplementation(features, implementationImplemented, 24)))
	builder.WriteString(fmt.Sprintf("部分可用/需验证：%s。\n", formatFeatureNamesByImplementation(features, implementationPartial, 18)))
	builder.WriteString(fmt.Sprintf("未实现/待开发：%s。\n", formatFeatureNamesByImplementation(features, implementationNotImplemented, 18)))
	builder.WriteString("估算使用规则：命中已实现能力时，只计算配置、复用接入、局部改造、联调和验证成本；命中部分可用能力时，补充完成度验证、缺口实现和回归成本；命中未实现或未标注能力时，按新增设计、实现、集成、验收和风险缓冲估算，并在 missing_info 中列出需人工确认的问题。")
	return strings.TrimSpace(builder.String())
}

func aiContextProfileDTO(profile db.AIContextProfile) AIContextProfileDTO {
	features := decodeAIContextFeatures(profile.FeatureSnapshotJSON)
	implementationBreakdown, implementedCount, partialCount, notImplementedCount, unknownCount := summarizeImplementationFeatures(features)
	return AIContextProfileDTO{
		ID:                      profile.ID,
		ModuleName:              profile.ModuleName,
		SourceFilename:          profile.SourceFilename,
		SourceSheet:             profile.SourceSheet,
		Summary:                 profile.Summary,
		PromptSummary:           profile.PromptSummary,
		FeatureCount:            profile.FeatureCount,
		ImplementedCount:        implementedCount,
		PartialCount:            partialCount,
		NotImplementedCount:     notImplementedCount,
		UnknownCount:            unknownCount,
		ConfigurableCount:       profile.ConfigurableCount,
		NonConfigurableCount:    profile.NonConfigurableCount,
		ImplementationBreakdown: implementationBreakdown,
		StatusBreakdown:         decodeIntMap(profile.StatusBreakdownJSON),
		TypeBreakdown:           decodeIntMap(profile.TypeBreakdownJSON),
		FeatureSnapshot:         features,
		Enabled:                 profile.Enabled,
		Version:                 profile.Version,
		CreatedAt:               profile.CreatedAt,
		UpdatedAt:               profile.UpdatedAt,
	}
}

func findPreviousAIContextProfile(profile db.AIContextProfile) (db.AIContextProfile, bool) {
	var previous db.AIContextProfile
	err := db.DB.Where("module_name = ? AND version < ?", profile.ModuleName, profile.Version).
		Order("version desc, created_at desc").
		First(&previous).Error
	return previous, err == nil
}

func diffAIContextProfiles(previous db.AIContextProfile, current db.AIContextProfile) AIContextDiff {
	before := decodeAIContextFeatures(previous.FeatureSnapshotJSON)
	after := decodeAIContextFeatures(current.FeatureSnapshotJSON)
	diff := diffAIContextFeatures(before, after)
	diff.AgainstID = previous.ID
	diff.AgainstVersion = previous.Version
	diff.Summary = fmt.Sprintf("相对 v%d：新增 %d 项，移除 %d 项，变化 %d 项。",
		previous.Version, diff.AddedCount, diff.RemovedCount, diff.ChangedCount)
	return diff
}

func diffAIContextFeatures(before []AIContextFeature, after []AIContextFeature) AIContextDiff {
	beforeMap := map[string]AIContextFeature{}
	afterMap := map[string]AIContextFeature{}
	for _, feature := range before {
		beforeMap[featureMapKey(feature.Feature)] = feature
	}
	for _, feature := range after {
		afterMap[featureMapKey(feature.Feature)] = feature
	}

	diff := AIContextDiff{}
	for key, feature := range afterMap {
		previous, ok := beforeMap[key]
		if !ok {
			diff.Added = append(diff.Added, feature.Feature)
			continue
		}
		if !sameAIContextFeature(previous, feature) {
			diff.Changed = append(diff.Changed, fmt.Sprintf("%s：%s -> %s",
				feature.Feature,
				implementationBucket(previous.Status),
				implementationBucket(feature.Status),
			))
		}
	}
	for key, feature := range beforeMap {
		if _, ok := afterMap[key]; !ok {
			diff.Removed = append(diff.Removed, feature.Feature)
		}
	}
	sort.Strings(diff.Added)
	sort.Strings(diff.Removed)
	sort.Strings(diff.Changed)
	diff.AddedCount = len(diff.Added)
	diff.RemovedCount = len(diff.Removed)
	diff.ChangedCount = len(diff.Changed)
	return diff
}

func sameAIContextFeature(a AIContextFeature, b AIContextFeature) bool {
	return implementationBucket(a.Status) == implementationBucket(b.Status)
}

func decodeIntMap(raw string) map[string]int {
	result := map[string]int{}
	if raw == "" {
		return result
	}
	_ = json.Unmarshal([]byte(raw), &result)
	return result
}

func decodeAIContextFeatures(raw string) []AIContextFeature {
	var result []AIContextFeature
	if raw == "" {
		return result
	}
	_ = json.Unmarshal([]byte(raw), &result)
	return result
}

func deriveModuleNameFromFilename(filename string) string {
	base := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	base = strings.TrimSpace(base)
	if base == "" {
		return "未命名模块"
	}
	return base
}

func normalizeHeader(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.ReplaceAll(value, " ", "")
	value = strings.ReplaceAll(value, "_", "")
	value = strings.ReplaceAll(value, "-", "")
	return value
}

func setColumnIfMissing(columns map[string]int, key string, index int) {
	if _, exists := columns[key]; !exists {
		columns[key] = index
	}
}

func pickColumn(columns map[string]int, keys []string, fallback int) int {
	for _, key := range keys {
		if index, ok := columns[key]; ok {
			return index
		}
	}
	return fallback
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}

func cellAt(row []string, index int) string {
	if index < 0 || index >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[index])
}

func normalizedBucket(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func isConfigurableValue(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return false
	}
	if strings.Contains(normalized, "不可") || strings.Contains(normalized, "否") || strings.Contains(normalized, "no") || strings.Contains(normalized, "false") {
		return false
	}
	return strings.Contains(normalized, "可") ||
		strings.Contains(normalized, "是") ||
		strings.Contains(normalized, "支持") ||
		strings.Contains(normalized, "yes") ||
		strings.Contains(normalized, "true") ||
		normalized == "y" ||
		normalized == "1"
}

func formatBreakdown(values map[string]int, limit int) string {
	type pair struct {
		Key   string
		Value int
	}
	pairs := make([]pair, 0, len(values))
	for key, value := range values {
		pairs = append(pairs, pair{Key: key, Value: value})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].Value == pairs[j].Value {
			return pairs[i].Key < pairs[j].Key
		}
		return pairs[i].Value > pairs[j].Value
	})
	if limit > 0 && len(pairs) > limit {
		pairs = pairs[:limit]
	}
	parts := make([]string, 0, len(pairs))
	for _, pair := range pairs {
		parts = append(parts, fmt.Sprintf("%s %d", pair.Key, pair.Value))
	}
	if len(parts) == 0 {
		return "无"
	}
	return strings.Join(parts, " / ")
}

func formatImplementationBreakdown(values map[string]int) string {
	parts := []string{}
	for _, key := range []string{implementationImplemented, implementationPartial, implementationNotImplemented, implementationUnknown} {
		parts = append(parts, fmt.Sprintf("%s %d", key, values[key]))
	}
	return strings.Join(parts, " / ")
}

func formatFeatureNamesByImplementation(features []AIContextFeature, bucket string, limit int) string {
	names := make([]string, 0, len(features))
	for _, feature := range features {
		if implementationBucket(feature.Status) == bucket {
			names = append(names, feature.Feature)
		}
	}
	sort.Strings(names)
	if len(names) == 0 {
		return "无"
	}
	if limit > 0 && len(names) > limit {
		return strings.Join(names[:limit], "、") + fmt.Sprintf("、+%d", len(names)-limit)
	}
	return strings.Join(names, "、")
}

func limitFeatures(features []AIContextFeature, limit int) []AIContextFeature {
	if limit <= 0 || len(features) <= limit {
		return features
	}
	return features[:limit]
}

func featureNames(features []AIContextFeature) []string {
	names := make([]string, 0, len(features))
	for _, feature := range features {
		names = append(names, feature.Feature)
	}
	sort.Strings(names)
	return names
}

func featureMapKey(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = regexp.MustCompile(`\s+`).ReplaceAllString(value, "")
	return value
}

func truncateString(value string, maxLen int) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) <= maxLen {
		return value
	}
	return string(runes[:maxLen]) + "..."
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}
