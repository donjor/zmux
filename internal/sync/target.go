package sync

// SyncTarget represents a source from which theme names can be pulled.
type SyncTarget interface {
	// Name returns the human-readable name of the sync target (e.g., "ghostty", "nvim").
	Name() string

	// Pull reads the current theme from the sync target and returns its name.
	// Returns an error if the theme cannot be determined.
	Pull() (string, error)
}
