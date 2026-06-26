package palette

// ActionProvider supplies a set of actions to the palette registry.
// Each provider is responsible for one domain (sessions, themes, etc.).
type ActionProvider interface {
	// Actions returns the current set of actions.
	// This is called each time the palette opens, so it should reflect
	// the current state (e.g., current sessions, available themes).
	Actions() ([]Action, error)
}

// CoverageDeclarer is implemented by providers that surface dynamic families
// (sessions, logical tabs) or open-surface entries (dashboard, help). The
// coverage gate reads Covers() to confirm those action specs are reachable
// without requiring live rows (a session/tab/theme) to exist in a bare test env.
type CoverageDeclarer interface {
	Covers() []string
}

// Registry aggregates actions from multiple providers.
type Registry struct {
	providers []ActionProvider
}

// NewRegistry creates a registry with the given providers.
func NewRegistry(providers ...ActionProvider) *Registry {
	return &Registry{providers: providers}
}

// All returns all actions from all providers, in provider order.
// Errors from individual providers are silently ignored (the palette
// still shows actions from other providers).
func (r *Registry) All() []Action {
	var all []Action
	for _, p := range r.providers {
		actions, err := p.Actions()
		if err != nil {
			continue
		}
		all = append(all, actions...)
	}
	return all
}
