package solutioncatalog

import (
	"sort"
	"strings"
	"unicode"
)

const maxSearchTokens = 96

func catalogTokens(values ...string) []string {
	seen := make(map[string]struct{})
	for _, value := range values {
		for _, segment := range textSegments(value) {
			runes := []rune(segment)
			if len(runes) == 1 {
				seen[segment] = struct{}{}
				continue
			}
			if isASCIIWord(runes) {
				seen[segment] = struct{}{}
				continue
			}
			for index := 0; index < len(runes)-1; index++ {
				seen[string(runes[index:index+2])] = struct{}{}
			}
		}
	}
	tokens := make([]string, 0, len(seen))
	for token := range seen {
		if token != "" {
			tokens = append(tokens, token)
		}
	}
	sort.Strings(tokens)
	if len(tokens) > maxSearchTokens {
		tokens = tokens[:maxSearchTokens]
	}
	return tokens
}

func textSegments(value string) []string {
	value = strings.ToLower(strings.TrimSpace(value))
	segments := make([]string, 0)
	var builder strings.Builder
	flush := func() {
		if builder.Len() == 0 {
			return
		}
		segments = append(segments, builder.String())
		builder.Reset()
	}
	var previousASCII bool
	for _, current := range value {
		if !unicode.IsLetter(current) && !unicode.IsDigit(current) {
			flush()
			previousASCII = false
			continue
		}
		currentASCII := current <= unicode.MaxASCII
		if builder.Len() > 0 && currentASCII != previousASCII {
			flush()
		}
		builder.WriteRune(current)
		previousASCII = currentASCII
	}
	flush()
	return segments
}

func isASCIIWord(runes []rune) bool {
	for _, current := range runes {
		if current > unicode.MaxASCII {
			return false
		}
	}
	return true
}

func projectKeyFromDemandID(demandID string) string {
	demandID = strings.ToUpper(strings.TrimSpace(demandID))
	separator := strings.Index(demandID, "-")
	if separator <= 0 {
		return ""
	}
	return demandID[:separator]
}
