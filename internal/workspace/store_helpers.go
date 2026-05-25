package workspace

import "sort"

// removeString returns slice with all occurrences of val removed.
func removeString(slice []string, val string) []string {
	out := make([]string, 0, len(slice))
	for _, s := range slice {
		if s != val {
			out = append(out, s)
		}
	}
	return out
}

// sortStrings is a thin alias kept for migrate.go's call site.
func sortStrings(s []string) {
	sort.Strings(s)
}
