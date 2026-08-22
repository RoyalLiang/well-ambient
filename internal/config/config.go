package config

import (
	"net/url"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config holds all configuration details for well-ambient
type Config struct {
	Server           ServerConfig           `yaml:"server" json:"server"`
	GitLab           GitLabConfig           `yaml:"gitlab" json:"gitlab"`
	Feishu           FeishuConfig           `yaml:"feishu" json:"feishu"`
	Jira             JiraConfig             `yaml:"jira" json:"jira"`
	AI               AIConfig               `yaml:"ai" json:"ai"`
	PerformanceBrain PerformanceBrainConfig `yaml:"performance_brain" json:"performance_brain"`
	SolutionCatalog  SolutionCatalogConfig  `yaml:"solution_catalog" json:"solution_catalog"`
}

const (
	DefaultPerformanceIntervalMinutes     = 60
	DefaultPerformanceWindowDays          = 90
	DefaultPerformanceRetentionDays       = 90
	DefaultPerformanceFormulaVersion      = "v6.0"
	DefaultPerformancePublicationMode     = "shadow"
	DefaultPerformanceCoverageGate        = 0.70
	DefaultPerformanceMinimumSamples      = 5
	DefaultPerformanceMinimumExposureDays = 30
	DefaultPerformanceBusyRetryCount      = 3
	DefaultPerformanceBusyRetryDelay      = 200
	DefaultSolutionCatalogIntervalMinutes = 30
	DefaultSolutionCatalogCandidateLimit  = 8
	DefaultSolutionCatalogRecallThreshold = 0.18
)

// SolutionCatalogConfig controls the local publication reconciler and bounded
// similarity recall. Model comparison itself uses the shared AI provider.
type SolutionCatalogConfig struct {
	IntervalMinutes int     `yaml:"interval_minutes" json:"interval_minutes"`
	CandidateLimit  int     `yaml:"candidate_limit" json:"candidate_limit"`
	RecallThreshold float64 `yaml:"recall_threshold" json:"recall_threshold"`
}

func (c SolutionCatalogConfig) Normalized() SolutionCatalogConfig {
	normalized := c
	if normalized.IntervalMinutes <= 0 {
		normalized.IntervalMinutes = DefaultSolutionCatalogIntervalMinutes
	}
	if normalized.CandidateLimit <= 0 {
		normalized.CandidateLimit = DefaultSolutionCatalogCandidateLimit
	}
	if normalized.CandidateLimit > 30 {
		normalized.CandidateLimit = 30
	}
	if normalized.RecallThreshold <= 0 || normalized.RecallThreshold > 1 {
		normalized.RecallThreshold = DefaultSolutionCatalogRecallThreshold
	}
	return normalized
}

// PerformanceBrainConfig controls the silent personnel-performance runner.
// Enabled is explicit so existing deployments remain inert until configured.
type PerformanceBrainConfig struct {
	Enabled              bool    `yaml:"enabled" json:"enabled"`
	IntervalMinutes      int     `yaml:"interval_minutes" json:"interval_minutes"`
	AssessmentWindowDays int     `yaml:"assessment_window_days" json:"assessment_window_days"`
	RetentionDays        int     `yaml:"retention_days" json:"retention_days"`
	FormulaVersion       string  `yaml:"formula_version" json:"formula_version"`
	EvidenceCoverageGate float64 `yaml:"evidence_coverage_gate" json:"evidence_coverage_gate"`
	MinimumSamples       int     `yaml:"minimum_samples" json:"minimum_samples"`
	MinimumExposureDays  int     `yaml:"minimum_exposure_days" json:"minimum_exposure_days"`
	BusyRetryAttempts    int     `yaml:"busy_retry_attempts" json:"busy_retry_attempts"`
	BusyRetryDelayMS     int     `yaml:"busy_retry_delay_ms" json:"busy_retry_delay_ms"`
	PublicationMode      string  `yaml:"publication_mode" json:"publication_mode"`
	DemandMetricsEnabled *bool   `yaml:"demand_metrics_enabled" json:"demand_metrics_enabled"`
	BugMetricsEnabled    *bool   `yaml:"bug_metrics_enabled" json:"bug_metrics_enabled"`
	CodeMetricsEnabled   *bool   `yaml:"code_metrics_enabled" json:"code_metrics_enabled"`
	JiraHistoryEnabled   *bool   `yaml:"jira_history_enabled" json:"jira_history_enabled"`
	GitDedupeEnabled     *bool   `yaml:"git_dedupe_enabled" json:"git_dedupe_enabled"`
}

// Normalized supplies safe runtime values without mutating the persisted config.
// The scorecard governs the formula contract; deployments may configure the
// scheduler and retention, but may not silently drift the published gates.
func (c PerformanceBrainConfig) Normalized() PerformanceBrainConfig {
	normalized := c
	if normalized.IntervalMinutes <= 0 {
		normalized.IntervalMinutes = DefaultPerformanceIntervalMinutes
	}
	if normalized.AssessmentWindowDays <= 0 {
		normalized.AssessmentWindowDays = DefaultPerformanceWindowDays
	}
	if normalized.RetentionDays <= 0 {
		normalized.RetentionDays = DefaultPerformanceRetentionDays
	}
	normalized.FormulaVersion = DefaultPerformanceFormulaVersion
	normalized.EvidenceCoverageGate = DefaultPerformanceCoverageGate
	normalized.MinimumSamples = DefaultPerformanceMinimumSamples
	normalized.MinimumExposureDays = DefaultPerformanceMinimumExposureDays
	normalized.PublicationMode = strings.ToLower(strings.TrimSpace(normalized.PublicationMode))
	if normalized.PublicationMode != "formal" && normalized.PublicationMode != "shadow" {
		normalized.PublicationMode = DefaultPerformancePublicationMode
	}
	normalized.DemandMetricsEnabled = defaultTrue(normalized.DemandMetricsEnabled)
	normalized.BugMetricsEnabled = defaultTrue(normalized.BugMetricsEnabled)
	normalized.CodeMetricsEnabled = defaultTrue(normalized.CodeMetricsEnabled)
	normalized.JiraHistoryEnabled = defaultTrue(normalized.JiraHistoryEnabled)
	normalized.GitDedupeEnabled = defaultTrue(normalized.GitDedupeEnabled)
	if normalized.BusyRetryAttempts <= 0 {
		normalized.BusyRetryAttempts = DefaultPerformanceBusyRetryCount
	}
	if normalized.BusyRetryDelayMS <= 0 {
		normalized.BusyRetryDelayMS = DefaultPerformanceBusyRetryDelay
	}
	return normalized
}

func defaultTrue(value *bool) *bool {
	if value != nil {
		copy := *value
		return &copy
	}
	value = new(bool)
	*value = true
	return value
}

// ServerConfig holds HTTP server settings
type ServerConfig struct {
	Port          int    `yaml:"port" json:"port"`
	Host          string `yaml:"host" json:"host"`
	PublicURL     string `yaml:"public_url,omitempty" json:"public_url,omitempty"`
	AttachmentDir string `yaml:"attachment_dir" json:"attachment_dir"`
}

// GitLabConfig holds connection settings for self-hosted GitLab
type GitLabConfig struct {
	Enabled  bool          `yaml:"enabled" json:"enabled"`
	BaseURL  string        `yaml:"base_url" json:"base_url"`
	Secret   string        `yaml:"secret_token" json:"secret_token"` // For webhook validation
	APIToken string        `yaml:"api_token" json:"api_token"`       // For GitLab API requests
	Repos    []RepoMapping `yaml:"repos" json:"repos"`
}

// RepoMapping maps GitLab repositories to internal projects or tasks
type RepoMapping struct {
	Name      string `yaml:"name" json:"name"`
	Path      string `yaml:"path" json:"path"`
	ProjectID string `yaml:"project_id" json:"project_id"`
}

// FeishuConfig holds credentials for Feishu/Lark Integration
type FeishuConfig struct {
	Enabled   bool          `yaml:"enabled" json:"enabled"`
	AppID     string        `yaml:"app_id" json:"app_id"`
	AppSecret string        `yaml:"app_secret" json:"app_secret"`
	Bot       BotConfig     `yaml:"bot" json:"bot"`
	Bitable   BitableConfig `yaml:"bitable" json:"bitable"`
}

// BotConfig holds chatbot settings
type BotConfig struct {
	Enabled   bool   `yaml:"enabled" json:"enabled"`
	ChatGroup string `yaml:"chat_group" json:"chat_group"` // Default group chat ID
}

// BitableConfig holds settings for Feishu Multidimensional Tables
type BitableConfig struct {
	Enabled   bool   `yaml:"enabled" json:"enabled"`
	AppToken  string `yaml:"app_token" json:"app_token"`
	TableID   string `yaml:"table_id" json:"table_id"`
	StatusCol string `yaml:"status_column" json:"status_column"`
	TaskIDCol string `yaml:"task_id_column" json:"task_id_column"`
}

// JiraConfig holds settings for Jira Integration
type JiraConfig struct {
	Enabled                 bool                `yaml:"enabled" json:"enabled"`
	BaseURL                 string              `yaml:"base_url" json:"base_url"`
	Username                string              `yaml:"username" json:"username"`
	APIToken                string              `yaml:"api_token" json:"api_token"`
	SyncProjects            []string            `yaml:"sync_projects" json:"sync_projects"`
	SyncUsers               []string            `yaml:"sync_users" json:"sync_users"`
	SyncStatuses            []string            `yaml:"sync_statuses" json:"sync_statuses"`
	CustomJQL               string              `yaml:"custom_jql" json:"custom_jql"`
	VersionSources          []JiraVersionSource `yaml:"version_sources,omitempty" json:"version_sources,omitempty"`
	VersionCatalogEnabled   bool                `yaml:"version_catalog_enabled,omitempty" json:"version_catalog_enabled"`
	VersionWritebackEnabled bool                `yaml:"version_writeback_enabled,omitempty" json:"version_writeback_enabled"`
}

// JiraVersionSource adds every issue assigned to one Jira release version to the sync scope.
type JiraVersionSource struct {
	ProjectKey  string `yaml:"project_key" json:"project_key"`
	ProjectName string `yaml:"project_name" json:"project_name"`
	VersionURL  string `yaml:"version_url" json:"version_url"`
}

// AIConfig holds settings for LLM deconstructor
type AIConfig struct {
	Enabled                bool    `yaml:"enabled" json:"enabled"`
	Provider               string  `yaml:"provider" json:"provider"` // e.g. "openai"
	BaseURL                string  `yaml:"base_url" json:"base_url"`
	EndpointType           string  `yaml:"endpoint_type" json:"endpoint_type"` // "responses" or native Claude "messages"; legacy "completions" migrates to responses
	APIToken               string  `yaml:"api_token" json:"api_token"`
	Model                  string  `yaml:"model" json:"model"`
	ProjectArchitecture    string  `yaml:"project_architecture" json:"project_architecture"`
	DeliveryWorkflow       string  `yaml:"delivery_workflow" json:"delivery_workflow"`
	ImplementedFeatures    string  `yaml:"implemented_features" json:"implemented_features"`
	EstimationGuidelines   string  `yaml:"estimation_guidelines" json:"estimation_guidelines"`
	DefaultWorkHoursPerDay float64 `yaml:"default_work_hours_per_day" json:"default_work_hours_per_day"`
}

// LoadConfig reads configuration from a YAML file
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// SaveConfig writes configuration back to a YAML file
func SaveConfig(path string, cfg *Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// Protocol returns the active provider wire protocol. Chat Completions is kept
// only as a persisted legacy value and is normalized to Responses.
func (c *AIConfig) Protocol() string {
	endpointType := strings.ToLower(strings.TrimSpace(c.EndpointType))
	if endpointType == "messages" || endpointType == "anthropic" {
		return "messages"
	}
	provider := strings.ToLower(strings.TrimSpace(c.Provider))
	baseURL := strings.ToLower(strings.TrimSpace(c.BaseURL))
	if (provider == "anthropic" || provider == "claude") && strings.Contains(baseURL, "api.anthropic.com") {
		return "messages"
	}
	return "responses"
}

// GetRealAPIURL returns the provider endpoint for the normalized protocol.
func (c *AIConfig) GetRealAPIURL() string {
	urlStr := strings.TrimSpace(c.BaseURL)
	if urlStr == "" {
		return ""
	}
	protocol := c.Protocol()
	parsed, err := url.Parse(urlStr)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return strings.TrimSuffix(urlStr, "/") + "/v1/" + protocol
	}
	pathLower := strings.ToLower(strings.TrimSuffix(parsed.Path, "/"))
	knownSuffixes := []string{"/v1/chat/completions", "/chat/completions", "/v1/responses", "/responses", "/v1/messages", "/messages"}
	for _, suffix := range knownSuffixes {
		if strings.HasSuffix(pathLower, suffix) {
			prefix := strings.TrimSuffix(parsed.Path, parsed.Path[len(parsed.Path)-len(suffix):])
			parsed.Path = strings.TrimSuffix(prefix, "/") + "/v1/" + protocol
			return parsed.String()
		}
	}
	if strings.HasSuffix(pathLower, "/v1") {
		parsed.Path = strings.TrimSuffix(parsed.Path, "/") + "/" + protocol
		return parsed.String()
	}
	parsed.Path = strings.TrimSuffix(parsed.Path, "/") + "/v1/" + protocol
	return parsed.String()
}
