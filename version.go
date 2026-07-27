package forage

import (
	"regexp"
	"strings"
)

var normRe = regexp.MustCompile(`[-_.]+`)

func normalizeName(name string) string {
	return strings.ToLower(normRe.ReplaceAllString(name, "-"))
}

func matchesVersion(filename, pkg, version string) bool {
	parts := strings.SplitN(filename, "-", 2)
	if len(parts) < 2 {
		return false
	}
	normalized := normalizeName(parts[0]) + "-" + parts[1]
	prefix := normalizeName(pkg) + "-" + version
	if !strings.HasPrefix(normalized, prefix) {
		return false
	}
	rest := normalized[len(prefix):]
	return len(rest) > 0 && (rest[0] == '.' || rest[0] == '-')
}
