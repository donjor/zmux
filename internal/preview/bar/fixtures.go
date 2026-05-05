package bar

// Fixtures provides realistic session name sets for driving the bar
// preview. Each set is ordered (position 0 = first in workspace).
var Fixtures = map[string][]string{
	"short": {"a", "b", "c", "d", "e"},

	"realistic": {"main", "feat-auth", "fix-pagination", "staging", "review"},

	"long": {
		"workspace-primary-picker",
		"dashboard-refactor-iteration",
		"post-refactor-detox-again",
		"something-with-a-really-long-name",
	},

	"mixed": {
		"main",
		"feat-auth",
		"main-b", // clone of main
		"review-of-pr-12345",
		"staging",
	},

	"agent-workflow": {
		"claude",
		"nvim",
		"server",
		"shell",
		"agent",
	},
}

// FixtureNames returns the ordered list of set names so the ChoiceControl
// stays stable across runs.
func FixtureNames() []string {
	return []string{"short", "realistic", "long", "mixed", "agent-workflow"}
}
