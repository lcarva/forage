package forage

import (
	"regexp"
	"strings"
)

var normRe = regexp.MustCompile(`[-_.]+`)

func normalizeName(name string) string {
	return strings.ToLower(normRe.ReplaceAllString(name, "-"))
}

var validExtensions = []string{".tar.gz", ".tar.bz2", ".zip", ".whl", ".egg"}

func matchesVersion(filename, pkg, version string) bool {
	parts := strings.SplitN(filename, "-", 2)
	if len(parts) < 2 {
		return false
	}
	normalized := strings.ToLower(normalizeName(parts[0]) + "-" + parts[1])
	prefix := normalizeName(pkg) + "-" + version
	if !strings.HasPrefix(normalized, prefix) {
		return false
	}
	rest := normalized[len(prefix):]
	if len(rest) == 0 {
		return false
	}
	if rest[0] == '-' {
		return true
	}
	for _, ext := range validExtensions {
		if rest == ext {
			return true
		}
	}
	return false
}
