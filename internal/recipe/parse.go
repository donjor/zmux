package recipe

import (
	"fmt"

	toml "github.com/pelletier/go-toml/v2"
)

func Parse(data []byte) (Recipe, error) {
	r, err := decode(data)
	if err != nil {
		return Recipe{}, err
	}
	applyDefaults(&r)
	if err := Validate(r); err != nil {
		return Recipe{}, err
	}
	return r, nil
}

func decode(data []byte) (Recipe, error) {
	var r Recipe
	if err := toml.Unmarshal(data, &r); err != nil {
		return Recipe{}, fmt.Errorf("parse recipe: %w", err)
	}
	return r, nil
}

func Marshal(r Recipe) ([]byte, error) {
	applyDefaults(&r)
	return toml.Marshal(r)
}

func applyDefaults(r *Recipe) {
	if r.Context == "" {
		r.Context = ContextAny
	}
	if r.Kind == "" {
		r.Kind = KindSession
	}
	if r.Workspace == "" {
		r.Workspace = "{{ cwd_name | slug }}"
	}
	if r.Defaults.TabMode == "" {
		r.Defaults.TabMode = r.Options.TabMode
	}
	if r.Defaults.TabMode == "" {
		r.Defaults.TabMode = TabModeRun
	}
	if r.Options.TabMode == "" {
		r.Options.TabMode = r.Defaults.TabMode
	}
	if r.Inputs.MinItems == 0 && (r.Inputs.Items || r.ForEach == "items" || hasItemFanout(r.Sessions)) {
		r.Inputs.MinItems = 1
	}
	if r.Inputs.Prompt == "" {
		r.Inputs.Prompt = "Items"
	}
}

func Validate(r Recipe) error {
	if r.Name == "" {
		return fmt.Errorf("recipe missing required field: name")
	}
	switch r.Context {
	case ContextAny, ContextInside, ContextOutside:
	default:
		return fmt.Errorf("recipe %q has unsupported context %q", r.Name, r.Context)
	}
	switch r.Kind {
	case KindSession, KindWorkspace:
	default:
		return fmt.Errorf("recipe %q has unsupported kind %q", r.Name, r.Kind)
	}
	if r.ForEach != "" && r.ForEach != "items" {
		return fmt.Errorf("recipe %q has unsupported foreach %q", r.Name, r.ForEach)
	}
	if r.Options.Rerun != "" && r.Options.Rerun != "skip" && r.Options.Rerun != "send" {
		return fmt.Errorf("recipe %q has unsupported rerun policy %q", r.Name, r.Options.Rerun)
	}
	if !validTabMode(r.Options.TabMode) {
		return fmt.Errorf("recipe %q has unsupported tab mode %q", r.Name, r.Options.TabMode)
	}
	if r.Defaults.TabMode != "" && !validTabMode(r.Defaults.TabMode) {
		return fmt.Errorf("recipe %q has unsupported default tab mode %q", r.Name, r.Defaults.TabMode)
	}
	for _, s := range r.Sessions {
		if s.ForEach != "" && s.ForEach != "items" {
			return fmt.Errorf("recipe %q session %q has unsupported foreach %q", r.Name, s.Name, s.ForEach)
		}
		for _, t := range s.Tabs {
			if t.Name == "" {
				return fmt.Errorf("recipe %q has a tab without a name", r.Name)
			}
		}
	}
	for _, t := range r.Tabs {
		if t.Name == "" {
			return fmt.Errorf("recipe %q has a tab without a name", r.Name)
		}
	}
	if r.Kind == KindWorkspace && r.Extends == "" && len(r.Sessions) == 0 && len(r.Tabs) == 0 {
		return fmt.Errorf("workspace recipe %q needs sessions or tabs", r.Name)
	}
	if r.Kind == KindSession && r.Extends == "" && len(r.Tabs) == 0 && len(r.Sessions) == 0 {
		return fmt.Errorf("session recipe %q needs tabs", r.Name)
	}
	return nil
}

func validTabMode(mode string) bool {
	switch mode {
	case "", TabModeRun, TabModeReady, TabModeEmpty:
		return true
	default:
		return false
	}
}

func hasItemFanout(sessions []SessionSpec) bool {
	for _, s := range sessions {
		if s.ForEach == "items" {
			return true
		}
	}
	return false
}
