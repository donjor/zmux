// Command zmux is a thin launcher for the zmux CLI; the command tree lives in
// the importable internal/cli package.
package main

import (
	"os"
	"runtime/debug"

	"github.com/donjor/zmux/internal/app"
	"github.com/donjor/zmux/internal/cli"
	zdebug "github.com/donjor/zmux/internal/debug"
)

// version is injected at build time via -ldflags -X main.version (see Makefile).
// Falls back to the Go module pseudo-version when built via `go install`, or to
// "dev" for plain `go build` invocations.
var version = "dev"

func init() {
	if version != "dev" {
		return
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	if info.Main.Version == "" || info.Main.Version == "(devel)" {
		return
	}
	version = info.Main.Version
}

func main() {
	a := app.New()
	// Route debug logs to the active profile's path (~/.zzmux/debug.log for the
	// zzmux edge profile) so they don't share the live zmux log.
	zdebug.SetLogPath(a.Profile.DebugLog)
	os.Exit(cli.Run(a, version))
}
