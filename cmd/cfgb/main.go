// Command cfgb is the Git-based blog tool.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/ymmt2005/cfgb/internal/version"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "cfgb: a command is required")
		fmt.Fprintln(stderr, "implemented in this build: version")
		return 2
	}
	switch args[0] {
	case "version", "--version", "-v":
		fmt.Fprintf(stdout, "cfgb %s\n", version.Version)
		return 0
	default:
		fmt.Fprintf(stderr, "cfgb: %s is not implemented\n", args[0])
		return 2
	}
}
