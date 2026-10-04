// Command cfgb is the Git-based blog tool.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"github.com/ymmt2005/cfgb/internal/build"
	"github.com/ymmt2005/cfgb/internal/version"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	cmd := newRootCommand(build.Run)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	// A nil Cobra args slice would fall back to os.Args, including test flags.
	cmd.SetArgs(append([]string{}, args...))
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(stderr, err)
		var exit *build.ExitError
		if errors.As(err, &exit) {
			return exit.Code
		}
		return 2
	}
	return 0
}

// Construct each invocation independently: commands/flags must not retain
// parsed values across calls. Domain behavior stays in internal/build.
func newRootCommand(buildRun func(build.Options) error) *cobra.Command {
	var configPath string
	root := &cobra.Command{
		Use:           "cfgb",
		Short:         "Git-based Blog on Cloudflare",
		Version:       version.Version,
		SilenceErrors: true,
		SilenceUsage:  true,
		Args:          cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("cfgb: a command is required (implemented: version, build)")
		},
	}
	root.CompletionOptions.DisableDefaultCmd = true
	root.SetVersionTemplate("cfgb {{.Version}}\n")
	root.PersistentFlags().StringVar(&configPath, "config", "", "configuration file (default: nearest ancestor cfgb.yaml)")
	root.PersistentFlags().BoolP("version", "v", false, "print the CFGB version")
	root.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if cmd.Flags().Changed("config") && configPath == "" {
			return fmt.Errorf("cfgb: --config requires a nonempty file path")
		}
		return nil
	}
	root.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print the CFGB version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "cfgb %s\n", version.Version)
			if err != nil {
				return &build.ExitError{Code: 3, Err: err}
			}
			return nil
		},
	})
	var out string
	buildCmd := &cobra.Command{
		Use:     "build",
		Short:   "Build the site with the embedded renderer",
		Version: version.Version,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := os.Getwd()
			if err != nil {
				return &build.ExitError{Code: 2, Err: err}
			}
			return buildRun(build.Options{
				Dir: dir, Config: configPath, Out: out,
				Stdout: cmd.OutOrStdout(), Stderr: cmd.ErrOrStderr(),
			})
		},
	}
	buildCmd.Flags().StringVar(&out, "out", "", "artifact directory relative to the configuration directory (default: dist)")
	root.AddCommand(buildCmd)
	return root
}
