package recipe

import (
	"embed"
	"sort"
	"strings"
)

//go:embed recipes/*
var bundledFS embed.FS

func Bundled() []Definition {
	defs := bundledRaw()
	if err := resolveDefinitions(defs); err != nil {
		return nil
	}
	sortDefinitions(defs)
	return defs
}

func bundledRaw() []Definition {
	entries, err := bundledFS.ReadDir("recipes")
	if err != nil {
		return nil
	}
	defs := make([]Definition, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".toml") {
			continue
		}
		path := "recipes/" + entry.Name()
		data, err := bundledFS.ReadFile(path)
		if err != nil {
			continue
		}
		r, err := decode(data)
		if err != nil {
			continue
		}
		defs = append(defs, Definition{Recipe: r, Source: SourceBundled, Path: path, Raw: data})
	}
	sortDefinitions(defs)
	return defs
}

func sortDefinitions(defs []Definition) {
	sort.Slice(defs, func(i, j int) bool {
		if defs[i].Recipe.Name == defs[j].Recipe.Name {
			return defs[i].Source < defs[j].Source
		}
		return defs[i].Recipe.Name < defs[j].Recipe.Name
	})
}
