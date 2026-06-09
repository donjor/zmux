package recipe

type Kind string

const (
	KindSession   Kind = "session"
	KindWorkspace Kind = "workspace"
)

type Source string

const (
	SourceBundled Source = "bundled"
	SourceUser    Source = "user"
)

type Recipe struct {
	Name        string        `toml:"name"`
	Description string        `toml:"description"`
	Extends     string        `toml:"extends"`
	Context     string        `toml:"context"`
	Kind        Kind          `toml:"kind"`
	Workspace   string        `toml:"workspace"`
	Session     string        `toml:"session"`
	CWD         string        `toml:"cwd"`
	ForEach     string        `toml:"foreach"`
	Inputs      Inputs        `toml:"inputs"`
	Defaults    Defaults      `toml:"defaults"`
	Tabs        []TabSpec     `toml:"tabs"`
	Sessions    []SessionSpec `toml:"sessions"`
	Options     Options       `toml:"options"`
}

const (
	ContextAny     = "any"
	ContextInside  = "inside"
	ContextOutside = "outside"
)

const (
	TabModeRun   = "run"
	TabModeReady = "ready"
	TabModeEmpty = "empty"
)

type Inputs struct {
	Items     bool   `toml:"items"`
	MinItems  int    `toml:"min_items"`
	Prompt    string `toml:"prompt"`
	Workspace bool   `toml:"workspace"`
	Session   bool   `toml:"session"`
	CWD       bool   `toml:"cwd"`
	TabMode   bool   `toml:"tab_mode"`
}

type Defaults struct {
	Workspace string `toml:"workspace"`
	Session   string `toml:"session"`
	CWD       string `toml:"cwd"`
	TabMode   string `toml:"tab_mode"`
}

type Options struct {
	FocusSession string `toml:"focus_session"`
	FocusTab     string `toml:"focus_tab"`
	Rerun        string `toml:"rerun"`
	TabMode      string `toml:"tab_mode"`
}

type SessionSpec struct {
	Name    string    `toml:"name"`
	CWD     string    `toml:"cwd"`
	ForEach string    `toml:"foreach"`
	Tabs    []TabSpec `toml:"tabs"`
}

type TabSpec struct {
	Name    string `toml:"name"`
	Command string `toml:"command"`
	CWD     string `toml:"cwd"`
}

type Definition struct {
	Recipe Recipe
	Source Source
	Path   string
	Raw    []byte
}

func (d Definition) Label() string {
	if d.Source == SourceBundled {
		return d.Recipe.Name + " (bundled)"
	}
	return d.Recipe.Name
}

type PlanOptions struct {
	Items      []string
	CWD        string
	Workspace  string
	Session    string
	TabMode    string
	Detach     bool
	Bin        string
	InsideZmux bool
}

type State struct {
	Sessions   map[string]SessionState
	Workspaces map[string]WorkspaceState
}

type SessionState struct {
	Name    string
	Windows map[string]WindowState
}

type WindowState struct {
	Name string
}

type WorkspaceState struct {
	Name     string
	Sessions map[string]bool
}

type Plan struct {
	RecipeName        string
	RecipeDescription string
	Context           string
	Kind              Kind
	Workspace         string
	CWD               string
	SessionDefault    string
	Items             []string
	TabMode           string
	Detach            bool
	InsideZmux        bool
	RunCommand        string
	FocusSession      string
	FocusTab          string
	Warnings          []PlanWarning
	Sessions          []PlannedSession
	Actions           []PlanAction
}

type PlanWarning struct {
	Target  string
	Message string
}

type PlannedSession struct {
	Name            string
	CWD             string
	Exists          bool
	WorkspaceMember bool
	Tabs            []PlannedTab
}

type PlannedTab struct {
	Name    string
	Command string
	CWD     string
	Exists  bool
}

type ActionKind string

const (
	ActionCreateWorkspace ActionKind = "create-workspace"
	ActionUseWorkspace    ActionKind = "use-workspace"
	ActionCreateSession   ActionKind = "create-session"
	ActionUseSession      ActionKind = "use-session"
	ActionAddMembership   ActionKind = "add-membership"
	ActionCreateTab       ActionKind = "create-tab"
	ActionUseTab          ActionKind = "use-tab"
	ActionSendCommand     ActionKind = "send-command"
	ActionFocus           ActionKind = "focus"
	ActionAttach          ActionKind = "attach"
)

type PlanAction struct {
	Kind    ActionKind
	Target  string
	Detail  string
	Skipped bool
}
