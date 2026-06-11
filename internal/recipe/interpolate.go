package recipe

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

var interpolationPattern = regexp.MustCompile(`\{\{\s*([a-z_]+)((?:\s*\|\s*[a-z_]+)*)\s*\}\}`)

func renderTemplate(input string, vars map[string]string) (string, error) {
	var firstErr error
	out := interpolationPattern.ReplaceAllStringFunc(input, func(match string) string {
		if firstErr != nil {
			return match
		}
		groups := interpolationPattern.FindStringSubmatch(match)
		key := groups[1]
		value, ok := vars[key]
		if !ok {
			firstErr = fmt.Errorf("unknown recipe variable %q", key)
			return match
		}
		filters := strings.Split(groups[2], "|")
		for _, raw := range filters {
			filter := strings.TrimSpace(raw)
			if filter == "" {
				continue
			}
			switch filter {
			case "slug":
				value = Slug(value)
			case "shellquote":
				value = ShellQuote(value)
			default:
				firstErr = fmt.Errorf("unknown recipe filter %q", filter)
				return match
			}
		}
		return value
	})
	if firstErr != nil {
		return "", firstErr
	}
	return out, nil
}

func Slug(input string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(strings.TrimSpace(input)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "item"
	}
	if out[0] >= '0' && out[0] <= '9' {
		return "r-" + out
	}
	return out
}

func ShellQuote(input string) string {
	if input == "" {
		return "''"
	}
	safe := true
	for _, r := range input {
		//nolint:staticcheck // QF1001: negated allowed-set ("not a safe char") reads clearer than the De Morgan expansion.
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune("_@%+=:,./-", r)) {
			safe = false
			break
		}
	}
	if safe {
		return input
	}
	return "'" + strings.ReplaceAll(input, "'", "'\\''") + "'"
}
