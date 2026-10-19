package opennames

import "testing"

func TestPrefixContract(t *testing.T) {
	for _, value := range []string{"", "well", "team-2", " abc ", "a123"} {
		if err := Validate(value); err != nil {
			t.Errorf("valid prefix %q: %v", value, err)
		}
	}
	for _, value := range []string{"Well", "-well", "well-", "well--team", "a/b", "a b", "$(id)", "a_b", "123", "abcdefghijklmnopqrstuvwxyz1234567"} {
		if Validate(value) == nil {
			t.Errorf("accepted invalid prefix %q", value)
		}
	}
	if Name("", "jira-analysis") != "well-ambient-jira-analysis" {
		t.Fatal("missing default namespace")
	}
}
