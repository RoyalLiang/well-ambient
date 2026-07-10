package delivery

import (
	"fmt"
	"path"
	"sort"
	"strings"
)

type ReviewPathRule struct {
	Pattern            string   `json:"pattern"`
	RequiredRole       string   `json:"required_role"`
	ReviewerCandidates []string `json:"reviewer_candidates"`
}

type ReviewerResolutionInput struct {
	RequiredRoles      []string
	ReviewerCandidates []string
	MinimumApprovals   int
	AcceptanceOwner    string
	ProtectedPathRules []ReviewPathRule
	ChangedPaths       []string
	Author             string
	Unavailable        []string
}

type ReviewerResolution struct {
	Resolved      bool     `json:"resolved"`
	Reviewers     []string `json:"reviewers"`
	RequiredRoles []string `json:"required_roles"`
	MatchedRules  []string `json:"matched_rules"`
	Blockers      []string `json:"blockers"`
	Reason        string   `json:"reason"`
}

func ResolveReviewers(input ReviewerResolutionInput) ReviewerResolution {
	minimum := input.MinimumApprovals
	if minimum < 1 {
		minimum = 1
	}
	blocked := normalizedSet(append(append([]string{}, input.Unavailable...), input.Author))
	roles := uniqueStrings(input.RequiredRoles)
	selected := make([]string, 0, minimum)
	matchedRules := make([]string, 0)
	blockers := make([]string, 0)

	for _, rule := range input.ProtectedPathRules {
		if !anyPathMatches(rule.Pattern, input.ChangedPaths) {
			continue
		}
		matchedRules = append(matchedRules, rule.Pattern)
		roles = appendUnique(roles, rule.RequiredRole)
		candidate := firstEligible(rule.ReviewerCandidates, blocked, selected)
		if candidate == "" {
			blockers = append(blockers, fmt.Sprintf("protected path %s has no eligible reviewer", rule.Pattern))
			continue
		}
		selected = appendUnique(selected, candidate)
	}

	for _, candidate := range uniqueStrings(input.ReviewerCandidates) {
		if len(selected) >= minimum {
			break
		}
		if blocked[strings.ToLower(candidate)] || containsFold(selected, candidate) {
			continue
		}
		selected = append(selected, candidate)
	}

	if strings.TrimSpace(input.AcceptanceOwner) == "" {
		blockers = append(blockers, "business acceptance owner is required")
	}
	if len(selected) < minimum {
		blockers = append(blockers, fmt.Sprintf("%d eligible reviewer(s) required, only %d resolved", minimum, len(selected)))
	}
	resolved := len(blockers) == 0
	reason := fmt.Sprintf("resolved %d reviewer(s) for %d approval(s)", len(selected), minimum)
	if !resolved {
		reason = strings.Join(blockers, "; ")
	}
	return ReviewerResolution{
		Resolved:      resolved,
		Reviewers:     selected,
		RequiredRoles: roles,
		MatchedRules:  uniqueStrings(matchedRules),
		Blockers:      uniqueStrings(blockers),
		Reason:        reason,
	}
}

type FileAction struct {
	Action   string `json:"action"`
	Path     string `json:"path"`
	Content  string `json:"content,omitempty"`
	Encoding string `json:"encoding,omitempty"`
}

func ValidateFileActions(actions []FileAction) ([]string, []string) {
	paths := make([]string, 0, len(actions))
	blockers := make([]string, 0)
	seen := map[string]bool{}
	for index, action := range actions {
		operation := NormalizeState(action.Action)
		filePath := strings.TrimSpace(strings.ReplaceAll(action.Path, "\\", "/"))
		if operation != "create" && operation != "update" && operation != "delete" {
			blockers = append(blockers, fmt.Sprintf("change %d has unsupported action %q", index+1, action.Action))
		}
		if filePath == "" || strings.HasPrefix(filePath, "/") || strings.Contains(filePath, "\x00") {
			blockers = append(blockers, fmt.Sprintf("change %d has an invalid path", index+1))
			continue
		}
		cleaned := path.Clean(filePath)
		if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") || cleaned == ".git" || strings.HasPrefix(cleaned, ".git/") {
			blockers = append(blockers, fmt.Sprintf("change %d targets a forbidden path", index+1))
			continue
		}
		key := strings.ToLower(cleaned)
		if seen[key] {
			blockers = append(blockers, fmt.Sprintf("duplicate change path %s", cleaned))
			continue
		}
		seen[key] = true
		paths = append(paths, cleaned)
		if operation != "delete" && action.Encoding != "" && action.Encoding != "text" && action.Encoding != "base64" {
			blockers = append(blockers, fmt.Sprintf("change %s has unsupported encoding %q", cleaned, action.Encoding))
		}
	}
	sort.Strings(paths)
	return paths, uniqueStrings(blockers)
}

func anyPathMatches(pattern string, paths []string) bool {
	for _, candidate := range paths {
		if matchPath(pattern, candidate) {
			return true
		}
	}
	return false
}

func matchPath(pattern, candidate string) bool {
	pattern = strings.TrimSpace(strings.ReplaceAll(pattern, "\\", "/"))
	candidate = strings.TrimSpace(strings.ReplaceAll(candidate, "\\", "/"))
	if pattern == "" || candidate == "" {
		return false
	}
	if pattern == "**" || pattern == "**/*" {
		return true
	}
	if strings.HasSuffix(pattern, "/**") {
		prefix := strings.TrimSuffix(pattern, "/**")
		return candidate == prefix || strings.HasPrefix(candidate, prefix+"/")
	}
	if strings.HasPrefix(pattern, "**/*.") {
		return strings.HasSuffix(candidate, strings.TrimPrefix(pattern, "**/*"))
	}
	matched, err := path.Match(pattern, candidate)
	return err == nil && matched
}

func firstEligible(candidates []string, blocked map[string]bool, selected []string) string {
	for _, candidate := range uniqueStrings(candidates) {
		if !blocked[strings.ToLower(candidate)] && !containsFold(selected, candidate) {
			return candidate
		}
	}
	return ""
}

func normalizedSet(values []string) map[string]bool {
	result := map[string]bool{}
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value != "" {
			result[value] = true
		}
	}
	return result
}

func uniqueStrings(values []string) []string {
	result := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		key := strings.ToLower(value)
		if value == "" || seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, value)
	}
	return result
}

func appendUnique(values []string, value string) []string {
	if strings.TrimSpace(value) == "" || containsFold(values, value) {
		return values
	}
	return append(values, strings.TrimSpace(value))
}

func containsFold(values []string, candidate string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(candidate)) {
			return true
		}
	}
	return false
}
