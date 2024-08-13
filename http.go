package bep14

import (
	"strings"
)

// parseList parses a comma separated list of values. Commas are ignored in
// quoted strings. Quoted values are not unescaped or unquoted. Whitespace is
// trimmed.
func parseList(values []string) []string {
	var result = make([]string, 0, len(values))
	for _, s := range values {
		if !strings.Contains(s, ",") {
			result = append(result, s)
			continue
		}

		for _, h := range strings.Split(s, ",") {
			result = append(result, strings.TrimSpace(h))
		}
	}

	return result
}
