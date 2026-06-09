package recipe

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/donjor/zmux/internal/config"
)

func DefaultDirs(fs config.FS, profile config.Profile) []string {
	if profile.RecipesDir != "" {
		return []string{profile.RecipesDir}
	}
	home, err := fs.UserHomeDir()
	if err != nil {
		return []string{"~/.zmux/recipes"}
	}
	return []string{filepath.Join(home, ".zmux", "recipes")}
}

func ConfiguredDirs(fs config.FS, profile config.Profile, cfg config.Config) []string {
	defaultPaths := config.DefaultConfig().Recipes.Paths
	if len(cfg.Recipes.Paths) > 0 && !samePaths(cfg.Recipes.Paths, defaultPaths) {
		return cfg.Recipes.Paths
	}
	return DefaultDirs(fs, profile)
}

func samePaths(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func Load(fs config.FS, dirs []string, disabledNames ...[]string) ([]Definition, error) {
	byName := map[string]Definition{}
	disabled := map[string]bool{}
	if len(disabledNames) > 0 {
		for _, name := range disabledNames[0] {
			disabled[name] = true
		}
	}
	for _, def := range bundledRaw() {
		byName[def.Recipe.Name] = def
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
			data, err := fs.ReadFile(path)
			if err != nil {
				continue
			}
			r, err := decode(data)
			if err != nil {
				continue
			}
			byName[r.Name] = Definition{Recipe: r, Source: SourceUser, Path: path, Raw: data}
		}
	}
	defs := make([]Definition, 0, len(byName))
	for _, def := range byName {
		defs = append(defs, def)
	}
	if err := resolveDefinitions(defs); err != nil {
		return nil, err
	}
	filtered := defs[:0]
	for _, def := range defs {
		if def.Source == SourceBundled && disabled[def.Recipe.Name] {
			continue
		}
		filtered = append(filtered, def)
	}
	defs = filtered
	sortDefinitions(defs)
	return defs, nil
}

func Find(defs []Definition, name string) (Definition, bool) {
	for _, def := range defs {
		if def.Recipe.Name == name {
			return def, true
		}
	}
	return Definition{}, false
}

func Names(defs []Definition) []string {
	names := make([]string, 0, len(defs))
	for _, def := range defs {
		names = append(names, def.Recipe.Name)
	}
	return names
}

func JoinNames(defs []Definition) string {
	names := Names(defs)
	if len(names) == 0 {
		return "none"
	}
	return strings.Join(names, ", ")
}

func UserRecipePath(fs config.FS, profile config.Profile, name string) (string, error) {
	dirs := DefaultDirs(fs, profile)
	if len(dirs) == 0 {
		return "", fmt.Errorf("no recipe directory configured")
	}
	dir, err := config.ExpandHome(fs, dirs[0])
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name+".toml"), nil
}
