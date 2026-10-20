package config

import "testing"

func TestHeadroomDefaultFollowsAIEnablement(t *testing.T) {
	disabled := false
	enabled := true
	for _, tt := range []struct {
		cfg  AIConfig
		want bool
	}{{AIConfig{}, false}, {AIConfig{Enabled: true}, true}, {AIConfig{Enabled: true, HeadroomEnabled: &disabled}, false}, {AIConfig{Enabled: true, HeadroomEnabled: &enabled}, true}, {AIConfig{HeadroomEnabled: &enabled}, false}} {
		if tt.cfg.UseHeadroom() != tt.want {
			t.Errorf("config %+v: want %v", tt.cfg, tt.want)
		}
	}
}
