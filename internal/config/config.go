package config

import (
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config holds all configuration details for well-ambient
type Config struct {
	Server ServerConfig `yaml:"server" json:"server"`
	GitLab GitLabConfig `yaml:"gitlab" json:"gitlab"`
	Feishu FeishuConfig `yaml:"feishu" json:"feishu"`
	Jira   JiraConfig   `yaml:"jira" json:"jira"`
	AI     AIConfig     `yaml:"ai" json:"ai"`
}

// ServerConfig holds HTTP server settings
type ServerConfig struct {
	Port int    `yaml:"port" json:"port"`
	Host string `yaml:"host" json:"host"`
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
	Enabled      bool     `yaml:"enabled" json:"enabled"`
	BaseURL      string   `yaml:"base_url" json:"base_url"`
	Username     string   `yaml:"username" json:"username"`
	APIToken     string   `yaml:"api_token" json:"api_token"`
	SyncProjects []string `yaml:"sync_projects" json:"sync_projects"`
	SyncUsers    []string `yaml:"sync_users" json:"sync_users"`
	SyncStatuses []string `yaml:"sync_statuses" json:"sync_statuses"`
	CustomJQL    string   `yaml:"custom_jql" json:"custom_jql"`
}

// AIConfig holds settings for LLM deconstructor
type AIConfig struct {
	Enabled                bool    `yaml:"enabled" json:"enabled"`
	Provider               string  `yaml:"provider" json:"provider"` // e.g. "openai"
	BaseURL                string  `yaml:"base_url" json:"base_url"`
	EndpointType           string  `yaml:"endpoint_type" json:"endpoint_type"` // e.g. "completions" or "responses"
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

// GetRealAPIURL returns the constructed API URL based on configuration
func (c *AIConfig) GetRealAPIURL() string {
	urlStr := strings.TrimSpace(c.BaseURL)
	if urlStr == "" {
		return ""
	}
	endpointType := strings.ToLower(c.EndpointType)
	if endpointType == "" {
		endpointType = "completions"
	}

	lowerURL := strings.ToLower(urlStr)
	if strings.Contains(lowerURL, "/v1/") || strings.HasSuffix(lowerURL, "/completions") || strings.HasSuffix(lowerURL, "/responses") {
		return urlStr
	}

	urlStr = strings.TrimSuffix(urlStr, "/")
	if endpointType == "responses" {
		return urlStr + "/v1/responses"
	}
	return urlStr + "/v1/chat/completions"
}
