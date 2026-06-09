package recipe

import "fmt"

func resolveDefinitions(defs []Definition) error {
	byName := map[string]int{}
	for i, def := range defs {
		if def.Recipe.Name == "" {
			return fmt.Errorf("recipe in %s missing required field: name", def.Path)
		}
		byName[def.Recipe.Name] = i
	}

	resolved := map[string]bool{}
	visiting := map[string]bool{}
	var resolve func(int) (Recipe, error)
	resolve = func(i int) (Recipe, error) {
		name := defs[i].Recipe.Name
		if resolved[name] {
			return defs[i].Recipe, nil
		}
		if visiting[name] {
			return Recipe{}, fmt.Errorf("recipe %q has an extends cycle", name)
		}
		visiting[name] = true
		r := defs[i].Recipe
		if r.Extends != "" {
			parentIndex, ok := byName[r.Extends]
			if !ok {
				return Recipe{}, fmt.Errorf("recipe %q extends unknown recipe %q", name, r.Extends)
			}
			parent, err := resolve(parentIndex)
			if err != nil {
				return Recipe{}, err
			}
			r = mergeRecipe(parent, r)
		}
		applyDefaults(&r)
		if err := Validate(r); err != nil {
			return Recipe{}, err
		}
		defs[i].Recipe = r
		resolved[name] = true
		visiting[name] = false
		return r, nil
	}

	for i := range defs {
		if _, err := resolve(i); err != nil {
			return err
		}
	}
	return nil
}

func mergeRecipe(parent Recipe, child Recipe) Recipe {
	out := parent
	out.Name = child.Name
	out.Extends = child.Extends
	if child.Description != "" {
		out.Description = child.Description
	}
	if child.Context != "" {
		out.Context = child.Context
	}
	if child.Kind != "" {
		out.Kind = child.Kind
	}
	if child.Workspace != "" {
		out.Workspace = child.Workspace
	}
	if child.Session != "" {
		out.Session = child.Session
	}
	if child.CWD != "" {
		out.CWD = child.CWD
	}
	if child.ForEach != "" {
		out.ForEach = child.ForEach
	}
	out.Inputs = mergeInputs(parent.Inputs, child.Inputs)
	out.Defaults = mergeDefaults(parent.Defaults, child.Defaults)
	out.Options = mergeOptions(parent.Options, child.Options)
	if len(child.Tabs) > 0 {
		out.Tabs = mergeTabs(parent.Tabs, child.Tabs)
	}
	if len(child.Sessions) > 0 {
		out.Sessions = mergeSessions(parent.Sessions, child.Sessions)
	}
	return out
}

func mergeInputs(parent Inputs, child Inputs) Inputs {
	out := parent
	if child.Items {
		out.Items = true
	}
	if child.MinItems != 0 {
		out.MinItems = child.MinItems
	}
	if child.Prompt != "" {
		out.Prompt = child.Prompt
	}
	if child.Workspace {
		out.Workspace = true
	}
	if child.Session {
		out.Session = true
	}
	if child.CWD {
		out.CWD = true
	}
	if child.TabMode {
		out.TabMode = true
	}
	return out
}

func mergeDefaults(parent Defaults, child Defaults) Defaults {
	out := parent
	if child.Workspace != "" {
		out.Workspace = child.Workspace
	}
	if child.Session != "" {
		out.Session = child.Session
	}
	if child.CWD != "" {
		out.CWD = child.CWD
	}
	if child.TabMode != "" {
		out.TabMode = child.TabMode
	}
	return out
}

func mergeOptions(parent Options, child Options) Options {
	out := parent
	if child.FocusSession != "" {
		out.FocusSession = child.FocusSession
	}
	if child.FocusTab != "" {
		out.FocusTab = child.FocusTab
	}
	if child.Rerun != "" {
		out.Rerun = child.Rerun
	}
	if child.TabMode != "" {
		out.TabMode = child.TabMode
	}
	return out
}

func mergeTabs(parent []TabSpec, child []TabSpec) []TabSpec {
	out := append([]TabSpec(nil), parent...)
	index := map[string]int{}
	for i, tab := range out {
		index[tab.Name] = i
	}
	for _, tab := range child {
		if i, ok := index[tab.Name]; ok {
			out[i] = tab
			continue
		}
		index[tab.Name] = len(out)
		out = append(out, tab)
	}
	return out
}

func mergeSessions(parent []SessionSpec, child []SessionSpec) []SessionSpec {
	out := append([]SessionSpec(nil), parent...)
	index := map[string]int{}
	for i, sess := range out {
		index[sessionMergeKey(sess)] = i
	}
	for _, sess := range child {
		key := sessionMergeKey(sess)
		if i, ok := index[key]; ok {
			merged := out[i]
			if sess.Name != "" {
				merged.Name = sess.Name
			}
			if sess.CWD != "" {
				merged.CWD = sess.CWD
			}
			if sess.ForEach != "" {
				merged.ForEach = sess.ForEach
			}
			if len(sess.Tabs) > 0 {
				merged.Tabs = mergeTabs(merged.Tabs, sess.Tabs)
			}
			out[i] = merged
			continue
		}
		index[key] = len(out)
		out = append(out, sess)
	}
	return out
}

func sessionMergeKey(sess SessionSpec) string {
	if sess.Name != "" {
		return sess.Name
	}
	return "<default>"
}
