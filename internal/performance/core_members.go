package performance

import (
	"strings"

	userdb "well-ambient/internal/db/user"

	"gorm.io/gorm"
)

// coreMemberMatcher is deliberately fail-closed. Personnel scoring is a
// publication boundary: without an explicit core-member source, no subject is
// eligible for a score snapshot.
type coreMemberMatcher struct {
	canonicalByKey map[string]string
}

func loadCoreMemberMatcher(tx *gorm.DB, configured []string) (coreMemberMatcher, error) {
	configured = normalizeCoreMemberValues(configured)
	matcher := coreMemberMatcher{canonicalByKey: make(map[string]string, len(configured)*4)}
	if len(configured) == 0 {
		return matcher, nil
	}

	var users []userdb.User
	if err := tx.Find(&users).Error; err != nil {
		return coreMemberMatcher{}, err
	}

	for _, member := range configured {
		canonical := member
		memberKeys := coreMemberKeyVariants(member)
		for _, user := range users {
			userKeys := coreMemberIdentityKeys(user)
			if !coreMemberKeysIntersect(memberKeys, userKeys) {
				continue
			}
			if name := strings.TrimSpace(user.Name); name != "" {
				canonical = name
			} else if username := strings.TrimSpace(user.Username); username != "" {
				canonical = username
			}
			for _, key := range userKeys {
				if _, exists := matcher.canonicalByKey[key]; !exists {
					matcher.canonicalByKey[key] = canonical
				}
			}
			break
		}
		for _, key := range memberKeys {
			if _, exists := matcher.canonicalByKey[key]; !exists {
				matcher.canonicalByKey[key] = canonical
			}
		}
	}
	return matcher, nil
}

func (m coreMemberMatcher) canonical(value string) (string, bool) {
	for _, key := range coreMemberKeyVariants(value) {
		if canonical, ok := m.canonicalByKey[key]; ok {
			return canonical, true
		}
	}
	return "", false
}

func normalizeCoreMemberValues(values []string) []string {
	normalized := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		keys := coreMemberKeyVariants(value)
		if len(keys) == 0 {
			continue
		}
		if _, exists := seen[keys[0]]; exists {
			continue
		}
		seen[keys[0]] = struct{}{}
		normalized = append(normalized, value)
	}
	return normalized
}

func coreMemberIdentityKeys(user userdb.User) []string {
	values := []string{user.Name, user.Username, user.Email}
	keys := make([]string, 0, len(values)*4)
	seen := make(map[string]struct{}, len(values)*4)
	for _, value := range values {
		for _, key := range coreMemberKeyVariants(value) {
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			keys = append(keys, key)
		}
	}
	return keys
}

func coreMemberKeyVariants(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" || value == "-" || value == "未指派" || strings.EqualFold(value, "unassigned") {
		return nil
	}
	variants := []string{value}
	if index := strings.Index(value, "@"); index >= 0 {
		variants = append(variants, value[:index])
	}
	if fields := strings.Fields(value); len(fields) > 0 {
		variants = append(variants, fields[0])
	}

	keys := make([]string, 0, len(variants)*2)
	seen := make(map[string]struct{}, len(variants)*2)
	for _, variant := range variants {
		key := strings.ToLower(strings.TrimSpace(variant))
		compact := strings.NewReplacer(".", "", "_", "", "-", "", " ", "").Replace(key)
		for _, candidate := range []string{key, compact} {
			if candidate == "" {
				continue
			}
			if _, exists := seen[candidate]; exists {
				continue
			}
			seen[candidate] = struct{}{}
			keys = append(keys, candidate)
		}
	}
	return keys
}

func coreMemberKeysIntersect(left, right []string) bool {
	lookup := make(map[string]struct{}, len(left))
	for _, key := range left {
		lookup[key] = struct{}{}
	}
	for _, key := range right {
		if _, ok := lookup[key]; ok {
			return true
		}
	}
	return false
}
