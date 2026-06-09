// Command qa is the dedicated QA walkthrough runner (plan 028, reworked):
// committed checklists in <repo>/checklists/*.toml, scorecards in gitignored
// <repo>/.qa/. Deliberately NOT a zmux verb; it is invoked via the repo's ./qa
// wrapper (cached go build), promotable to PATH later if it earns it.
package main

import (
	"os"

	"github.com/donjor/zmux/internal/config"
	"github.com/donjor/zmux/internal/qa"
	qacli "github.com/donjor/zmux/internal/qa/cli"
)

func main() {
	os.Exit(qacli.Run(qacli.Deps{
		FS:     &config.RealFS{},
		State:  qa.RealStateFS{},
		Runner: qa.ShellRunner{},
	}, os.Args[1:]))
}
