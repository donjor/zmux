package qa

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/donjor/zmux/internal/config"
)

// Ref points at a discovered checklist without parsing it.
type Ref struct {
	Path string
	Stem string
}

// FindRepoRoot walks up from dir to the first directory containing .git —
// checklists and step commands are repo-relative, like git itself.
func FindRepoRoot(fs config.FS, dir string) (string, error) {
	cur, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	for {
		if _, err := fs.Stat(filepath.Join(cur, ".git")); err == nil {
			return cur, nil
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return "", fmt.Errorf("no repository found above %s (checklists live in <repo>/checklists/)", dir)
		}
		cur = parent
	}
}

// Discover lists the repo's committed checklists (flat checklists/*.toml, sorted).
// Stems are unique by construction in a flat dir; the dup-stem lint in
// LintRefs is future-proofing for nested discovery.
func Discover(fs config.FS, repoRoot string) ([]Ref, error) {
	matches, err := fs.Glob(filepath.Join(repoRoot, "checklists", "*.toml"))
	if err != nil {
		return nil, fmt.Errorf("scan qa/: %w", err)
	}
	sort.Strings(matches)
	refs := make([]Ref, 0, len(matches))
	for _, m := range matches {
		refs = append(refs, Ref{Path: m, Stem: strings.TrimSuffix(filepath.Base(m), ".toml")})
	}
	return refs, nil
}

// LintRefs reports stem collisions across a discovered set — deterministic
// ambiguity instead of "first match wins".
func LintRefs(refs []Ref) []string {
	seen := make(map[string]string, len(refs))
	var issues []string
	for _, r := range refs {
		if prev, ok := seen[r.Stem]; ok {
			issues = append(issues, fmt.Sprintf("duplicate checklist stem %q: %s and %s", r.Stem, prev, r.Path))
			continue
		}
		seen[r.Stem] = r.Path
	}
	return issues
}

// Resolve maps a user-supplied name to a checklist path: an explicit path
// (contains a separator or .toml suffix) wins; otherwise the stem is looked
// up in the repo's discovered set. Misses list what IS available.
func Resolve(fs config.FS, repoRoot, name string) (string, error) {
	if strings.ContainsRune(name, filepath.Separator) || strings.HasSuffix(name, ".toml") {
		if _, err := fs.Stat(name); err != nil {
			return "", fmt.Errorf("checklist %s: %w", name, err)
		}
		return name, nil
	}
	refs, err := Discover(fs, repoRoot)
	if err != nil {
		return "", err
	}
	var stems []string
	var hits []Ref
	for _, r := range refs {
		stems = append(stems, r.Stem)
		if r.Stem == name {
			hits = append(hits, r)
		}
	}
	switch {
	case len(hits) == 1:
		return hits[0].Path, nil
	case len(hits) > 1:
		var paths []string
		for _, h := range hits {
			paths = append(paths, h.Path)
		}
		return "", fmt.Errorf("checklist %q is ambiguous: %s", name, strings.Join(paths, ", "))
	case len(stems) == 0:
		return "", fmt.Errorf("no checklists in %s/checklists/", repoRoot)
	default:
		return "", fmt.Errorf("no checklist %q (have: %s)", name, strings.Join(stems, ", "))
	}
}
