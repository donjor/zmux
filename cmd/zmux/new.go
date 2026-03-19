package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/donjor/zmux/internal/config"
	"github.com/donjor/zmux/internal/session"
	"github.com/spf13/cobra"
)

var newTemplateFlag string

var newCmd = &cobra.Command{
	Use:     "new [name]",
	Aliases: []string{"n"},
	Short:   "Create a new session and attach",
	Long: `Create a new tmux session and attach to it.

If no name is given, creates a tmp-N session.
With --template/-t, creates from a template layout.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := os.Getwd()
		if err != nil {
			dir = os.Getenv("HOME")
		}

		name := ""
		if len(args) > 0 {
			name = args[0]
		}

		if newTemplateFlag != "" {
			return runNewFromTemplate(name, dir)
		}

		return runNew(name, dir)
	},
}

func runNew(name, dir string) error {
	if err := session.ValidateName(name); err != nil {
		return err
	}

	if name == "" {
		name = session.NextTmpName(app.Runner)
	}

	if app.Runner.HasSession(name) {
		return fmt.Errorf("session %q already exists", name)
	}

	if err := session.Create(app.Runner, name, dir); err != nil {
		return err
	}

	return session.Attach(app.Runner, name)
}

func runNewFromTemplate(name, dir string) error {
	cfg := config.DefaultConfig()
	templates, _ := session.LoadTemplates(app.FS, cfg.Templates.Paths)

	var tmpl *session.Template
	for i := range templates {
		if templates[i].Name == newTemplateFlag {
			tmpl = &templates[i]
			break
		}
	}

	if tmpl == nil {
		available := make([]string, len(templates))
		for i, t := range templates {
			available[i] = t.Name
		}
		return fmt.Errorf("template %q not found (available: %s)", newTemplateFlag, joinOr(available))
	}

	if name == "" {
		name = tmpl.Name
	}

	if app.Runner.HasSession(name) {
		return fmt.Errorf("session %q already exists", name)
	}

	if err := session.CreateFromTemplate(app.Runner, *tmpl, name, dir); err != nil {
		return err
	}

	return session.Attach(app.Runner, name)
}

func joinOr(items []string) string {
	if len(items) == 0 {
		return "none"
	}
	return strings.Join(items, ", ")
}

func init() {
	newCmd.Flags().StringVarP(&newTemplateFlag, "template", "t", "", "create from template")
	rootCmd.AddCommand(newCmd)
}
