package service

import (
	"regexp"
	"slices"
	"strings"
)

var domainPattern = regexp.MustCompile(`^(?i)[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+$`)

func NormalizeDomain(domain string) (string, bool) {
	normalized := strings.ToLower(strings.TrimSpace(domain))
	normalized = strings.TrimSuffix(normalized, ".")
	if !domainPattern.MatchString(normalized) {
		return "", false
	}
	return normalized, true
}

func IsValidProxyMode(mode string) bool {
	return slices.Contains([]string{"http", "https", "http_and_https"}, strings.TrimSpace(mode))
}
