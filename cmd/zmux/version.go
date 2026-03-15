package main

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print zmux version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("zmux %s (%s/%s)\n", version, runtime.GOOS, runtime.GOARCH)
	},
}
