package tabs

import "strings"

const (
	// DockSession is the reserved session hidden tabs are parked in. Lazily
	// created on first hide; allowed to die when emptied.
	DockSession = "__zmux_dock"
	// ReservedPrefix marks zmux-internal sessions. Every listing/adoption
	// surface (workspace reconcile, pickers, ls, bar, source discovery)
	// must skip these — belt and braces with the explicit dock mark.
	ReservedPrefix = "__zmux_"
	// OptDockMark is the session option stamped on a dock zmux created. A
	// pre-existing unmarked __zmux_dock is NOT ours: placement verbs refuse
	// rather than adopt a user's session.
	OptDockMark = "@zmux_dock"
)

// IsReservedSession reports whether a session name is zmux-internal and must
// be filtered from user-facing surfaces.
func IsReservedSession(name string) bool {
	return strings.HasPrefix(name, ReservedPrefix)
}
