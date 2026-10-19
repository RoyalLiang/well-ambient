// Package opennames defines the namespace shared by public Skills and MCP tools.
package opennames

import (
	"fmt"
	"regexp"
	"strings"
)

const DefaultPrefix = "well-ambient"

var prefixPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$`)

func Normalize(prefix string) string {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return DefaultPrefix
	}
	return prefix
}

func Validate(prefix string) error {
	prefix = Normalize(prefix)
	if len(prefix) > 32 || !prefixPattern.MatchString(prefix) {
		return fmt.Errorf("名称前缀须为 1–32 位小写字母、数字或单连字符，以字母开头，不能以连字符结尾")
	}
	return nil
}

func Name(prefix, canonical string) string {
	return Normalize(prefix) + "-" + canonical
}
