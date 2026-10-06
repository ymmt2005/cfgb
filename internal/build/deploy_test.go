package build

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ymmt2005/cfgb/internal/config"
	"github.com/ymmt2005/cfgb/internal/worker"
)

func deployFixture(t *testing.T) (DeployOptions, deployManifest, string) {
	t.Helper()
	stubBuildProbes(t)
	_, repo := testRepo(t)
	if _, err := output("git", "-C", repo, "checkout", "-b", "main"); err != nil {
		t.Fatal(err)
	}
	workspace, err := newWorkspace()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(workspace); err != nil {
			t.Error(err)
		}
	})
	if err := os.Mkdir(filepath.Join(workspace, "renderer"), 0o700); err != nil {
		t.Fatal(err)
	}
	artifact := filepath.Join(repo, "dist")
	for name, data := range map[string]string{
		"site/en/index.html": "edited site bytes", "worker/index.js": "export default {};",
	} {
		file := filepath.Join(artifact, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	pins, err := loadToolchain()
	if err != nil {
		t.Fatal(err)
	}
	m := deployManifest{SchemaVersion: 1, ToolchainSessionID: filepath.Base(workspace), Checks: []string{"render", "pagefind"}, Publications: []publicationRecord{{ArticleKey: "article", Locale: "en", Summary: "Summary", PublishedAt: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)}}}
	m.Toolchain.NodeRange = pins.NodeRange
	m.Toolchain.NodeVersion = testedNodeVersion
	m.Toolchain.PackageManager = "npm"
	m.Toolchain.WranglerVersion = pins.WranglerVersion
	m.Toolchain.WorkerCompatibilityDate = worker.CompatibilityDate
	writeDeployManifest(t, artifact, m)
	t.Setenv("CFGB_CF_WORKER_NAME", "example-worker")
	t.Setenv("CLOUDFLARE_ACCOUNT_ID", "test-account")
	t.Setenv("CLOUDFLARE_API_TOKEN", "test-token")
	t.Setenv("WORKERS_CI_BRANCH", "")
	return DeployOptions{Dir: repo, From: "dist"}, m, workspace
}

func writeDeployManifest(t *testing.T, artifact string, m deployManifest) {
	t.Helper()
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(artifact, "build-manifest.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestDeployUploadsEditedArtifactAndCleansSessions(t *testing.T) {
	for _, dryRun := range []bool{false, true} {
		t.Run(map[bool]string{false: "upload", true: "dry run"}[dryRun], func(t *testing.T) {
			opts, manifest, workspace := deployFixture(t)
			opts.DryRun = dryRun
			artifact := filepath.Join(opts.Dir, opts.From)
			// Upload does not need an installer or the build's exact Node patch.
			manifest.Toolchain.NodeVersion = "26.10.0"
			writeDeployManifest(t, artifact, manifest)
			original, err := os.ReadFile(filepath.Join(artifact, "build-manifest.json"))
			if err != nil {
				t.Fatal(err)
			}
			t.Setenv("CFGB_PACKAGE_MANAGER", "unused-during-upload")
			if dryRun {
				t.Setenv("CLOUDFLARE_API_TOKEN", "")
			}
			calls := 0
			var configPath string
			upload := func(dir string, stdout, stderr io.Writer, extra []string, name string, args ...string) error {
				calls++
				if dir != filepath.Join(workspace, "renderer") || name != "node" || args[0] != filepath.Join(dir, "node_modules", "wrangler", "bin", "wrangler.js") {
					t.Fatalf("unexpected command: %s %s %v", dir, name, args)
				}
				if slices.Contains(args, "--version") {
					_, err := io.WriteString(stdout, manifest.Toolchain.WranglerVersion+"\n")
					return err
				}
				if len(args) < 5 || args[1] != "deploy" || args[2] != "--config" || args[4] != "--no-bundle" || slices.Contains(args, "--dry-run") != dryRun {
					t.Fatalf("upload arguments: %v", args)
				}
				configPath = args[3]
				raw, err := os.ReadFile(configPath)
				if err != nil {
					t.Fatal(err)
				}
				var conf struct {
					Main   string `json:"main"`
					Assets struct {
						Directory string `json:"directory"`
					} `json:"assets"`
				}
				if err := json.Unmarshal(raw, &conf); err != nil {
					t.Fatal(err)
				}
				if conf.Main != filepath.Join(artifact, "worker", "index.js") || conf.Assets.Directory != filepath.Join(artifact, "site") {
					t.Fatalf("configuration did not select supplied artifact: %s", raw)
				}
				return nil
			}
			if err := deploy(opts, upload); err != nil {
				t.Fatal(err)
			}
			if calls != 2 {
				t.Fatalf("expected version probe and upload, got %d calls", calls)
			}
			if _, err := os.Stat(filepath.Dir(configPath)); !os.IsNotExist(err) {
				t.Fatalf("upload workspace remained: %v", err)
			}
			_, err = os.Stat(workspace)
			if (dryRun && err != nil) || (!dryRun && !os.IsNotExist(err)) {
				t.Fatalf("build workspace lifecycle: dryRun=%t, %v", dryRun, err)
			}
			data, err := os.ReadFile(filepath.Join(artifact, "site", "en", "index.html"))
			if err != nil || string(data) != "edited site bytes" {
				t.Fatalf("upload changed site bytes: %s, %v", data, err)
			}
			data, err = os.ReadFile(filepath.Join(artifact, "build-manifest.json"))
			if err != nil || !bytes.Equal(data, original) {
				t.Fatalf("upload changed manifest: %v", err)
			}
		})
	}
}

func TestDeployPreflightDoesNotUploadOrConsumeSession(t *testing.T) {
	for _, tc := range []struct {
		name, code string
		exit       int
		mutate     func(*testing.T, DeployOptions, *deployManifest)
	}{
		{"future publication", "E_FUTURE_DATE", 1, func(t *testing.T, o DeployOptions, m *deployManifest) {
			m.Publications[0].PublishedAt = time.Now().Add(time.Hour)
		}},
		{"future update", "E_FUTURE_DATE", 1, func(t *testing.T, o DeployOptions, m *deployManifest) {
			future := time.Now().Add(time.Hour)
			m.Publications[0].UpdatedAt = &future
		}},
		{"missing summary", "E_SUMMARY_REQUIRED", 1, func(t *testing.T, o DeployOptions, m *deployManifest) { m.Publications[0].Summary = " \n" }},
		{"incomplete checks", "E_ARTIFACT", 1, func(t *testing.T, o DeployOptions, m *deployManifest) { m.Checks = []string{"render"} }},
		{"missing snapshot", "E_ARTIFACT", 1, func(t *testing.T, o DeployOptions, m *deployManifest) { m.Publications = nil }},
		{"wrong artifact version", "E_ARTIFACT", 1, func(t *testing.T, o DeployOptions, m *deployManifest) { m.SchemaVersion++ }},
		{"wrong runtime date", "E_ARTIFACT", 1, func(t *testing.T, o DeployOptions, m *deployManifest) {
			m.Toolchain.WorkerCompatibilityDate = "2099-01-01"
		}},
		{"unsupported Node range", "E_ARTIFACT", 1, func(t *testing.T, o DeployOptions, m *deployManifest) { m.Toolchain.NodeRange = ">=99" }},
		{"session traversal", "E_TOOLCHAIN", 2, func(t *testing.T, o DeployOptions, m *deployManifest) { m.ToolchainSessionID = "cfgb-build-../other" }},
		{"missing session", "E_TOOLCHAIN", 2, func(t *testing.T, o DeployOptions, m *deployManifest) { m.ToolchainSessionID += "-missing" }},
		{"missing token", "E_DEPLOY_TARGET", 2, func(t *testing.T, o DeployOptions, m *deployManifest) { t.Setenv("CLOUDFLARE_API_TOKEN", "") }},
		{"wrong branch", "E_DEPLOY_TARGET", 1, func(t *testing.T, o DeployOptions, m *deployManifest) {
			if _, err := output("git", "-C", o.Dir, "checkout", "-b", "post/test"); err != nil {
				t.Fatal(err)
			}
			t.Setenv("WORKERS_CI_BRANCH", "main")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts, m, workspace := deployFixture(t)
			tc.mutate(t, opts, &m)
			writeDeployManifest(t, filepath.Join(opts.Dir, opts.From), m)
			called := false
			err := deploy(opts, func(string, io.Writer, io.Writer, []string, string, ...string) error { called = true; return nil })
			var exit *ExitError
			if !errors.As(err, &exit) || exit.Code != tc.exit || !strings.Contains(err.Error(), tc.code) || called {
				t.Fatalf("preflight: err=%v, called=%t", err, called)
			}
			if _, err := os.Stat(workspace); err != nil {
				t.Fatalf("preflight consumed session: %v", err)
			}
		})
	}
}

func TestDeployFailurePreservesCauseAndRemovesWorkspaces(t *testing.T) {
	opts, m, workspace := deployFixture(t)
	failure := errors.New("remote upload failed")
	var configPath string
	err := deploy(opts, func(dir string, stdout, stderr io.Writer, extra []string, name string, args ...string) error {
		if slices.Contains(args, "--version") {
			_, err := io.WriteString(stdout, m.Toolchain.WranglerVersion)
			return err
		}
		configPath = args[3]
		return failure
	})
	var exit *ExitError
	if !errors.Is(err, failure) || !errors.As(err, &exit) || exit.Code != 3 {
		t.Fatalf("lost provider failure: %v", err)
	}
	for _, path := range []string{workspace, filepath.Dir(configPath)} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("failure left workspace %s: %v", path, err)
		}
	}
}

func TestDeployMissingAndCorruptArtifact(t *testing.T) {
	for _, name := range []string{"missing Worker", "invalid manifest"} {
		t.Run(name, func(t *testing.T) {
			opts, _, workspace := deployFixture(t)
			artifact := filepath.Join(opts.Dir, opts.From)
			if name == "missing Worker" {
				if err := os.Remove(filepath.Join(artifact, "worker", "index.js")); err != nil {
					t.Fatal(err)
				}
			} else if err := os.WriteFile(filepath.Join(artifact, "build-manifest.json"), []byte("{"), 0o600); err != nil {
				t.Fatal(err)
			}
			called := false
			err := deploy(opts, func(string, io.Writer, io.Writer, []string, string, ...string) error { called = true; return nil })
			var exit *ExitError
			if !errors.As(err, &exit) || exit.Code != 1 || called {
				t.Fatalf("invalid artifact: %v, called=%t", err, called)
			}
			if name == "missing Worker" && !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("missing file cause lost: %v", err)
			}
			if _, err := os.Stat(workspace); err != nil {
				t.Fatalf("artifact preflight consumed session: %v", err)
			}
		})
	}
}

func TestDeployRejectsDifferentInstalledWrangler(t *testing.T) {
	opts, _, workspace := deployFixture(t)
	err := deploy(opts, func(dir string, stdout, stderr io.Writer, extra []string, name string, args ...string) error {
		if !slices.Contains(args, "--version") {
			t.Fatal("uploaded with a different Wrangler")
		}
		_, err := io.WriteString(stdout, "4.135.0\n")
		return err
	})
	var exit *ExitError
	if !errors.As(err, &exit) || exit.Code != 2 || !strings.Contains(err.Error(), "E_TOOLCHAIN") {
		t.Fatalf("Wrangler mismatch: %v", err)
	}
	if _, err := os.Stat(workspace); err != nil {
		t.Fatalf("version preflight consumed session: %v", err)
	}
}

func TestDeployPrefixMountAndRuntimeConfig(t *testing.T) {
	opts, m, workspace := deployFixture(t)
	cfg, err := config.Load(opts.Dir)
	if err != nil {
		t.Fatal(err)
	}
	artifact := filepath.Join(opts.Dir, opts.From)
	m.BaseURL = "https://example.invalid/blog/"
	for _, special := range []string{"_headers", "_redirects", "404.html"} {
		if err := os.WriteFile(filepath.Join(artifact, "site", special), []byte(special), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := wranglerConfig(cfg, m, artifact, t.TempDir(), "test-worker", "test-account")
	if err != nil {
		t.Fatal(err)
	}
	var conf struct {
		CompatibilityDate string         `json:"compatibility_date"`
		WorkersDev        bool           `json:"workers_dev"`
		PreviewURLs       bool           `json:"preview_urls"`
		Previews          map[string]any `json:"previews"`
		Assets            struct {
			Directory   string   `json:"directory"`
			WorkerFirst []string `json:"run_worker_first"`
			HTML        string   `json:"html_handling"`
			NotFound    string   `json:"not_found_handling"`
			Binding     string   `json:"binding"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(raw, &conf); err != nil {
		t.Fatal(err)
	}
	if conf.CompatibilityDate != worker.CompatibilityDate || conf.WorkersDev || !conf.PreviewURLs || conf.Previews == nil || !slices.Equal(conf.Assets.WorkerFirst, []string{"/blog/", "/blog/__locale"}) || conf.Assets.HTML != "auto-trailing-slash" || conf.Assets.NotFound != "404-page" || conf.Assets.Binding != "ASSETS" {
		t.Fatalf("runtime config: %s", raw)
	}
	for path, want := range map[string]string{"blog/en/index.html": "edited site bytes", "_headers": "_headers", "_redirects": "_redirects", "404.html": "404.html", "blog/404.html": "404.html"} {
		data, err := os.ReadFile(filepath.Join(conf.Assets.Directory, filepath.FromSlash(path)))
		if err != nil || string(data) != want {
			t.Fatalf("mounted %s: %q, %v", path, data, err)
		}
	}
	if _, err := os.Stat(workspace); err != nil {
		t.Fatal(err)
	}
}
