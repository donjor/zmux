package recipe

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/donjor/zmux/internal/session"
	"github.com/donjor/zmux/internal/workspace"
)

func DefaultOptions(r Recipe, cwd string) PlanOptions {
	applyDefaults(&r)
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			cwd = "."
		}
	}
	cwdName := filepath.Base(cwd)
	if cwdName == "." || cwdName == string(filepath.Separator) {
		cwdName = "workspace"
	}
	workspaceName := r.Defaults.Workspace
	if workspaceName == "" {
		workspaceName = cwdName
	}
	vars := map[string]string{
		"cwd":       cwd,
		"cwd_name":  cwdName,
		"recipe":    r.Name,
		"workspace": cwdName,
	}
	if rendered, err := renderTemplate(workspaceName, vars); err == nil {
		workspaceName = rendered
	}
	vars["workspace"] = workspaceName
	sessionName := r.Defaults.Session
	if sessionName == "" {
		sessionName = workspaceName
	}
	vars["session"] = sessionName
	if rendered, err := renderTemplate(sessionName, vars); err == nil {
		sessionName = rendered
	}
	tabMode := r.Defaults.TabMode
	if tabMode == "" {
		tabMode = r.Options.TabMode
	}
	if tabMode == "" {
		tabMode = TabModeRun
	}
	return PlanOptions{
		CWD:       cwd,
		Workspace: Slug(workspaceName),
		Session:   Slug(sessionName),
		TabMode:   tabMode,
	}
}

func PlanRecipe(r Recipe, opts PlanOptions, state State) (Plan, error) {
	applyDefaults(&r)
	if err := Validate(r); err != nil {
		return Plan{}, err
	}
	if r.Context == ContextInside && !opts.InsideZmux {
		return Plan{}, fmt.Errorf("recipe %q is inside-zmux only", r.Name)
	}
	if r.Context == ContextOutside && opts.InsideZmux {
		return Plan{}, fmt.Errorf("recipe %q is outside-zmux only", r.Name)
	}
	cwd := opts.CWD
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			cwd = "."
		}
	}
	if r.Defaults.CWD != "" && opts.CWD == "" {
		rendered, err := renderTemplate(r.Defaults.CWD, map[string]string{"cwd": cwd})
		if err != nil {
			return Plan{}, fmt.Errorf("render default cwd: %w", err)
		}
		cwd = rendered
	}
	cwdName := filepath.Base(cwd)
	if cwdName == "." || cwdName == string(filepath.Separator) {
		cwdName = "workspace"
	}
	defaults := DefaultOptions(r, cwd)
	if opts.Workspace == "" {
		opts.Workspace = defaults.Workspace
	}
	if opts.Session == "" {
		opts.Session = defaults.Session
	}
	if opts.TabMode == "" {
		opts.TabMode = defaults.TabMode
	}
	if !validTabMode(opts.TabMode) {
		return Plan{}, fmt.Errorf("unsupported tab mode %q", opts.TabMode)
	}
	vars := map[string]string{
		"cwd":       cwd,
		"cwd_name":  cwdName,
		"recipe":    r.Name,
		"workspace": opts.Workspace,
		"session":   opts.Session,
	}
	if vars["workspace"] == "" {
		vars["workspace"] = cwdName
	}
	workspaceName, err := renderTemplate(r.Workspace, vars)
	if err != nil {
		return Plan{}, fmt.Errorf("render workspace: %w", err)
	}
	if opts.Workspace != "" {
		workspaceName = opts.Workspace
	}
	vars["workspace"] = workspaceName
	if opts.Session == "" {
		opts.Session = workspaceName
	}
	vars["session"] = opts.Session

	if r.Inputs.MinItems > 0 && len(opts.Items) < r.Inputs.MinItems {
		return Plan{}, fmt.Errorf("recipe %q expects at least %d item(s)", r.Name, r.Inputs.MinItems)
	}

	focusSession, err := renderTemplate(r.Options.FocusSession, vars)
	if err != nil {
		return Plan{}, fmt.Errorf("render focus session: %w", err)
	}
	focusTab, err := renderTemplate(r.Options.FocusTab, vars)
	if err != nil {
		return Plan{}, fmt.Errorf("render focus tab: %w", err)
	}

	planned, err := expandSessions(r, opts, cwd, vars)
	if err != nil {
		return Plan{}, err
	}
	if err := validateUniquePlannedNames(planned); err != nil {
		return Plan{}, err
	}
	if focusSession == "" && len(planned) > 0 {
		focusSession = planned[0].Name
	}

	p := Plan{
		RecipeName:        r.Name,
		RecipeDescription: r.Description,
		Context:           r.Context,
		Kind:              r.Kind,
		Workspace:         workspaceName,
		CWD:               cwd,
		SessionDefault:    opts.Session,
		Items:             append([]string(nil), opts.Items...),
		TabMode:           opts.TabMode,
		Detach:            opts.Detach,
		InsideZmux:        opts.InsideZmux,
		RunCommand:        RunCommandWithOptions(opts.Bin, r.Name, opts, defaults),
		FocusSession:      focusSession,
		FocusTab:          focusTab,
		Sessions:          planned,
	}
	if _, ok := state.Workspaces[workspaceName]; ok {
		p.Warnings = append(p.Warnings, PlanWarning{Target: workspaceName, Message: "workspace exists; will reuse it"})
		p.Actions = append(p.Actions, PlanAction{Kind: ActionUseWorkspace, Target: workspaceName, Skipped: true})
	} else {
		p.Actions = append(p.Actions, PlanAction{Kind: ActionCreateWorkspace, Target: workspaceName})
	}

	wsState := state.Workspaces[workspaceName]
	for si := range p.Sessions {
		sess := &p.Sessions[si]
		if err := session.ValidateName(sess.Name); err != nil {
			return Plan{}, fmt.Errorf("invalid planned session %q: %w", sess.Name, err)
		}
		if _, ok := state.Sessions[sess.Name]; ok {
			sess.Exists = true
			p.Warnings = append(p.Warnings, PlanWarning{Target: sess.Name, Message: "session exists; will reuse it"})
			p.Actions = append(p.Actions, PlanAction{Kind: ActionUseSession, Target: sess.Name, Skipped: true})
		} else {
			p.Actions = append(p.Actions, PlanAction{Kind: ActionCreateSession, Target: sess.Name, Detail: sess.CWD})
		}
		if wsState.Sessions != nil && wsState.Sessions[sess.Name] {
			sess.WorkspaceMember = true
		} else {
			p.Actions = append(p.Actions, PlanAction{Kind: ActionAddMembership, Target: workspaceName, Detail: sess.Name})
		}
		for ti := range sess.Tabs {
			tab := &sess.Tabs[ti]
			if tab.Name == "" {
				return Plan{}, fmt.Errorf("planned tab in %q has empty name", sess.Name)
			}
			target := sess.Name + ":" + tab.Name
			if ss, ok := state.Sessions[sess.Name]; ok {
				if _, ok := ss.Windows[tab.Name]; ok {
					tab.Exists = true
					p.Warnings = append(p.Warnings, PlanWarning{Target: target, Message: "tab exists; command will be skipped"})
				}
			}
			if tab.Exists {
				p.Actions = append(p.Actions, PlanAction{Kind: ActionUseTab, Target: target, Skipped: true})
				continue
			}
			p.Actions = append(p.Actions, PlanAction{Kind: ActionCreateTab, Target: target, Detail: tab.CWD})
			if tab.Command != "" && p.TabMode != TabModeEmpty {
				detail := tab.Command
				if p.TabMode == TabModeReady {
					detail = "ready: " + detail
				}
				p.Actions = append(p.Actions, PlanAction{Kind: ActionSendCommand, Target: target, Detail: detail})
			}
		}
	}
	if p.FocusSession != "" {
		target := p.FocusSession
		if p.FocusTab != "" {
			target += ":" + p.FocusTab
		}
		p.Actions = append(p.Actions, PlanAction{Kind: ActionFocus, Target: target, Skipped: p.Detach})
		if !p.Detach {
			detail := "attach-session"
			if p.InsideZmux {
				detail = "switch-client"
			}
			p.Actions = append(p.Actions, PlanAction{Kind: ActionAttach, Target: p.FocusSession, Detail: detail})
		}
	}
	return p, nil
}

func expandSessions(r Recipe, opts PlanOptions, cwd string, vars map[string]string) ([]PlannedSession, error) {
	baseCWD, err := renderCWD(cwd, r.CWD, vars)
	if err != nil {
		return nil, err
	}

	var specs []SessionSpec
	if len(r.Sessions) > 0 {
		specs = r.Sessions
	} else {
		specs = []SessionSpec{{
			Name:    r.Session,
			CWD:     r.CWD,
			ForEach: r.ForEach,
			Tabs:    r.Tabs,
		}}
	}
	if r.Kind == KindSession && specs[0].Name == "" {
		specs[0].Name = r.Session
	}

	var out []PlannedSession
	for _, spec := range specs {
		items := []string{""}
		if spec.ForEach == "items" {
			items = opts.Items
		}
		for i, item := range items {
			local := cloneVars(vars)
			local["item"] = item
			local["index"] = strconv.Itoa(i + 1)
			nameTemplate := spec.Name
			if nameTemplate == "" {
				if spec.ForEach == "items" {
					nameTemplate = "{{ item | slug }}"
				} else if r.Session != "" {
					nameTemplate = r.Session
				} else {
					nameTemplate = "{{ workspace }}"
				}
			}
			name, err := renderTemplate(nameTemplate, local)
			if err != nil {
				return nil, fmt.Errorf("render session name: %w", err)
			}
			sessionCWD, err := renderCWD(baseCWD, spec.CWD, local)
			if err != nil {
				return nil, fmt.Errorf("render session cwd: %w", err)
			}
			ps := PlannedSession{Name: name, CWD: sessionCWD}
			for _, tab := range spec.Tabs {
				pt, err := renderTab(baseCWD, tab, local)
				if err != nil {
					return nil, err
				}
				ps.Tabs = append(ps.Tabs, pt)
			}
			out = append(out, ps)
		}
	}
	return out, nil
}

func renderTab(baseCWD string, tab TabSpec, vars map[string]string) (PlannedTab, error) {
	name, err := renderTemplate(tab.Name, vars)
	if err != nil {
		return PlannedTab{}, fmt.Errorf("render tab name: %w", err)
	}
	command, err := renderTemplate(tab.Command, vars)
	if err != nil {
		return PlannedTab{}, fmt.Errorf("render tab command: %w", err)
	}
	cwd, err := renderCWD(baseCWD, tab.CWD, vars)
	if err != nil {
		return PlannedTab{}, fmt.Errorf("render tab cwd: %w", err)
	}
	return PlannedTab{Name: name, Command: command, CWD: cwd}, nil
}

func renderCWD(base string, value string, vars map[string]string) (string, error) {
	if value == "" {
		return base, nil
	}
	rendered, err := renderTemplate(value, vars)
	if err != nil {
		return "", err
	}
	if rendered == "." {
		return base, nil
	}
	if filepath.IsAbs(rendered) {
		return rendered, nil
	}
	return filepath.Clean(filepath.Join(base, rendered)), nil
}

func cloneVars(vars map[string]string) map[string]string {
	out := make(map[string]string, len(vars)+2)
	for k, v := range vars {
		out[k] = v
	}
	return out
}

func validateUniquePlannedNames(sessions []PlannedSession) error {
	seenSessions := map[string]bool{}
	for _, sess := range sessions {
		if seenSessions[sess.Name] {
			return fmt.Errorf("recipe plans duplicate session %q", sess.Name)
		}
		seenSessions[sess.Name] = true
		seenTabs := map[string]bool{}
		for _, tab := range sess.Tabs {
			if seenTabs[tab.Name] {
				return fmt.Errorf("recipe plans duplicate tab %q in session %q", tab.Name, sess.Name)
			}
			seenTabs[tab.Name] = true
		}
	}
	return nil
}

func ValidateWorkspaceName(name string) error {
	return workspace.ValidateWorkspaceName(name)
}
