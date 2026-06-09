package tmux

// OptionScope selects the scope flag for a batched option write.
type OptionScope string

const (
	ScopePane   OptionScope = "-p"
	ScopeWindow OptionScope = "-w"
)

// OptionWrite is one set-option in a batch: pane or window scope, set or
// unset. Value is ignored when Unset.
type OptionWrite struct {
	Scope  OptionScope
	Target string
	Key    string
	Value  string
	Unset  bool
}

// ApplyOptions applies every write in ONE tmux invocation, joining the
// set-option commands with ";" argv separators — a state write that used to
// cost a process spawn per option costs one for the whole batch.
func (c *Client) ApplyOptions(writes []OptionWrite) error {
	if len(writes) == 0 {
		return nil
	}
	return c.runSilent(buildApplyOptionsArgs(writes)...)
}

func buildApplyOptionsArgs(writes []OptionWrite) []string {
	args := make([]string, 0, len(writes)*7)
	for i, w := range writes {
		if i > 0 {
			args = append(args, ";")
		}
		args = append(args, "set-option", string(w.Scope))
		if w.Unset {
			args = append(args, "-u")
		}
		if w.Target != "" {
			args = append(args, "-t", w.Target)
		}
		args = append(args, w.Key)
		if !w.Unset {
			args = append(args, w.Value)
		}
	}
	return args
}
