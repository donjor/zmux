package main

import (
	"fmt"
	"os"
)

// version is injected at build time via -ldflags -X
var version = "dev"

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, formatError(err))
		os.Exit(exitCodeForError(err))
	}
}
