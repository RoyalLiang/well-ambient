package config

import (
	"strings"
	"well-ambient/internal/confluence"
)

const DefaultConfluenceParentPageURL = "https://confluence.westwell-lab.com/display/~zhiyuan_liang/well-infra"

type ConfluenceSyncConfig struct {
	Enabled       bool   `yaml:"enabled" json:"enabled"`
	ParentPageURL string `yaml:"parent_page_url" json:"parent_page_url"`
	Token         string `yaml:"token" json:"token"`
}

func (c ConfluenceSyncConfig) Normalized() ConfluenceSyncConfig {
	c.ParentPageURL = strings.TrimSpace(c.ParentPageURL)
	c.Token = strings.TrimSpace(c.Token)
	if c.ParentPageURL == "" && !c.Enabled {
		c.ParentPageURL = DefaultConfluenceParentPageURL
	}
	return c
}

func ValidateConfluenceSync(c ConfluenceSyncConfig) error {
	c = c.Normalized()
	if !c.Enabled {
		return nil
	}
	return confluence.Validate(confluence.Config{ParentPageURL: c.ParentPageURL, Token: c.Token})
}
