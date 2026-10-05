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
	// Cobra's help callback cannot return an error. Record stream failures so
	// help (and other framework output) cannot turn a failed write into exit 0.
	out := &checkedWriter{Writer: stdout}
	errOut := &checkedWriter{Writer: stderr}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	// A nil Cobra args slice would fall back to os.Args, including test flags.
	cmd.SetArgs(append([]string{}, args...))
	err := cmd.Execute()
	if writeErr := errors.Join(out.err, errOut.err); writeErr != nil {
		err = errors.Join(&build.ExitError{Code: 3, Err: fmt.Errorf("write command output: %w", writeErr)}, err)
	}
	if err != nil {
		if _, writeErr := fmt.Fprintln(stderr, err); writeErr != nil {
			// The diagnostic stream itself has failed; there is no other stream
			// to report it on. Preserve failure through the I/O exit code.
			return 3
		}
		var exit *build.ExitError
		if errors.As(err, &exit) {
			return exit.Code
		}
		return 2
	}
	return 0
}

type checkedWriter struct {
	io.Writer
	err error
}

func (w *checkedWriter) Write(p []byte) (int, error) {
	n, err := w.Writer.Write(p)
	if err == nil && n < len(p) {
		err = io.ErrShortWrite
	}
	if err != nil && w.err == nil {
		w.err = err
	}
	return n, err
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
	var out, baseURL string
	var static, force bool
	buildCmd := &cobra.Command{
		Use:     "build",
		Short:   "Build the site with the embedded renderer",
		Version: version.Version,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := os.Getwd()
			if err != nil {
				return &build.ExitError{Code: 3, Err: err}
			}
			return buildRun(build.Options{
				Dir: dir, Config: configPath, Out: out, BaseURL: baseURL, Static: static, Force: force,
				Stdin: cmd.InOrStdin(), Stdout: cmd.OutOrStdout(), Stderr: cmd.ErrOrStderr(),
			})
		},
	}
	buildCmd.Flags().StringVar(&out, "out", "", "artifact directory to replace, relative to the configuration directory (default: dist)")
	buildCmd.Flags().BoolVarP(&force, "force", "f", false, "replace existing output without confirmation")
	buildCmd.Flags().StringVar(&baseURL, "base-url", "", "override the public site URL, including its hosting path")
	buildCmd.Flags().BoolVar(&static, "static", false, "emit static entry/alias pages and direct language links for hosts without a Worker")
	root.AddCommand(buildCmd)
	return root
}
