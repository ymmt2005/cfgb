// Command cfgb is the Git-based blog tool.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/ymmt2005/cfgb/internal/build"
	"github.com/ymmt2005/cfgb/internal/version"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "cfgb: a command is required")
		fmt.Fprintln(stderr, "implemented in this build: version, build")
		return 2
	}
	switch args[0] {
	case "version", "--version", "-v":
		fmt.Fprintf(stdout, "cfgb %s\n", version.Version)
		return 0
	case "build":
		out, err := buildFlags(args[1:])
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		dir, err := os.Getwd()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		err = build.Run(build.Options{Dir: dir, Out: out, Stdout: stdout, Stderr: stderr})
		if err == nil {
			return 0
		}
		fmt.Fprintln(stderr, err)
		if exit, ok := err.(*build.ExitError); ok {
			return exit.Code
		}
		return 1
	default:
		fmt.Fprintf(stderr, "cfgb: %s is not implemented\n", args[0])
		return 2
	}
}

func buildFlags(args []string) (string, error) {
	out := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--out":
			if i+1 >= len(args) {
				return "", fmt.Errorf("cfgb: --out requires a directory")
			}
			i++
			out = args[i]
		default:
			if len(args[i]) > 0 && args[i][0] == '-' {
				return "", fmt.Errorf("cfgb: unknown flag %s", args[i])
			}
			return "", fmt.Errorf("cfgb: unexpected argument %s", args[i])
		}
	}
	return out, nil
}
