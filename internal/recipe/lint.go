package recipe

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/donjor/zmux/internal/config"
)

type LintResult struct {
	Name string
	Path string
	Err  error
}

func Lint(fs config.FS, dirs []string, names []string) []LintResult {
	nameSet := map[string]bool{}
	for _, name := range names {
		nameSet[name] = true
	}
	wants := func(name string) bool {
		return len(nameSet) == 0 || nameSet[name]
	}

	var results []LintResult
	for _, def := range Bundled() {
		if !wants(def.Recipe.Name) {
			continue
		}
		results = append(results, LintResult{
			Name: def.Recipe.Name,
			Path: def.Path,
			Err:  Validate(def.Recipe),
		})
	}

	for _, dir := range dirs {
		expanded, err := config.ExpandHome(fs, dir)
		if err != nil {
			continue
		}
		matches, err := fs.Glob(filepath.Join(expanded, "*.toml"))
		if err != nil {
			continue
		}
		sort.Strings(matches)
		for _, path := range matches {
			name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
			data, err := fs.ReadFile(path)
			if err != nil {
				if wants(name) {
					results = append(results, LintResult{Name: name, Path: path, Err: err})
				}
				continue
			}
			r, err := Parse(data)
			if err == nil {
				name = r.Name
				err = Validate(r)
			}
			if wants(name) {
				results = append(results, LintResult{Name: name, Path: path, Err: err})
			}
		}
	}
	return results
}
