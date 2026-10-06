package build

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/ymmt2005/cfgb/internal/config"
	"github.com/ymmt2005/cfgb/internal/worker"
)

// DeployOptions selects an existing artifact and the current deployment config.
// From is relative to Dir. DryRun validates the Wrangler upload without publishing.
type DeployOptions struct {
	Dir, Config, From string
	DryRun            bool
	Stdout, Stderr    io.Writer
}

type deployManifest struct {
	SchemaVersion      int                 `json:"schemaVersion"`
	BaseURL            string              `json:"baseUrl"`
	ToolchainSessionID string              `json:"toolchainSessionId"`
	Checks             []string            `json:"checks"`
	Publications       []publicationRecord `json:"publications"`
	Toolchain          struct {
		NodeRange               string `json:"nodeRange"`
		NodeVersion             string `json:"nodeVersion"`
		PackageManager          string `json:"packageManager"`
		WranglerVersion         string `json:"wranglerVersion"`
		WorkerCompatibilityDate string `json:"workerCompatibilityDate"`
	} `json:"toolchain"`
}

type toolCommand func(string, io.Writer, io.Writer, []string, string, ...string) error

// Deploy uploads the supplied Worker and assets using the retained toolchain.
func Deploy(opts DeployOptions) error {
	return deploy(opts, command)
}

func deploy(opts DeployOptions, run toolCommand) (err error) {
	if opts.Stdout == nil {
		opts.Stdout = io.Discard
	}
	if opts.Stderr == nil {
		opts.Stderr = io.Discard
	}
	if opts.From == "" {
		return &ExitError{Code: 2, Err: fmt.Errorf("deploy requires --from DIR")}
	}
	var cfg *config.File
	if opts.Config != "" {
		file := opts.Config
		if !filepath.IsAbs(file) {
			file = filepath.Join(opts.Dir, file)
		}
		cfg, err = config.LoadFile(file)
	} else {
		cfg, err = config.Load(opts.Dir)
	}
	if err != nil {
		code := 2
		var fsErr *config.IOError
		if errors.As(err, &fsErr) {
			code = 3
		}
		return &ExitError{Code: code, Err: err}
	}
	if err := productionBranch(cfg); err != nil {
		return err
	}
	artifact := opts.From
	if !filepath.IsAbs(artifact) {
		artifact = filepath.Join(opts.Dir, artifact)
	}
	artifact, err = filepath.Abs(artifact)
	if err != nil {
		return &ExitError{Code: 3, Err: err}
	}
	manifest, err := readDeployManifest(artifact)
	if err != nil {
		return err
	}
	if err := publicationGate(manifest.Publications, time.Now()); err != nil {
		return err
	}
	workerName := os.Getenv("CFGB_CF_WORKER_NAME")
	account := os.Getenv("CLOUDFLARE_ACCOUNT_ID")
	if workerName == "" || account == "" || (!opts.DryRun && os.Getenv("CLOUDFLARE_API_TOKEN") == "") {
		return &ExitError{Code: 2, Err: fmt.Errorf("E_DEPLOY_TARGET: set CFGB_CF_WORKER_NAME, CLOUDFLARE_ACCOUNT_ID and CLOUDFLARE_API_TOKEN (token unnecessary for --dry-run)")}
	}
	pins, err := loadToolchain()
	if err != nil {
		return &ExitError{Code: 2, Err: fmt.Errorf("E_TOOLCHAIN: %w", err)}
	}
	if manifest.Toolchain.NodeRange != pins.NodeRange || manifest.Toolchain.WranglerVersion != pins.WranglerVersion || manifest.Toolchain.WorkerCompatibilityDate != worker.CompatibilityDate {
		return &ExitError{Code: 1, Err: fmt.Errorf("E_ARTIFACT: artifact runtime requirements do not match this CFGB toolchain")}
	}
	// The installed upload toolchain needs Node, but no installer operation.
	node, err := output("node", "-p", "process.versions.node")
	if err != nil {
		return &ExitError{Code: 2, Err: fmt.Errorf("E_TOOLCHAIN: probe Node: %w", err)}
	}
	if !nodeInRange(node, pins.NodeRange) {
		return &ExitError{Code: 2, Err: fmt.Errorf("E_TOOLCHAIN: Node %s is outside %s", node, pins.NodeRange)}
	}
	workspace, err := uploadToolchain(manifest.ToolchainSessionID)
	if err != nil {
		return err
	}
	renderer := filepath.Join(workspace, "renderer")
	wrangler := filepath.Join(renderer, "node_modules", "wrangler", "bin", "wrangler.js")
	var versionOut bytes.Buffer
	extra := []string{"WRANGLER_SEND_METRICS=false", "CI=true"}
	if err := run(renderer, &versionOut, opts.Stderr, extra, "node", wrangler, "--version"); err != nil {
		return &ExitError{Code: 2, Err: fmt.Errorf("E_TOOLCHAIN: probe installed Wrangler: %w", err)}
	}
	if strings.TrimSpace(versionOut.String()) != pins.WranglerVersion {
		return &ExitError{Code: 2, Err: fmt.Errorf("E_TOOLCHAIN: installed Wrangler must be %s, found %q", pins.WranglerVersion, strings.TrimSpace(versionOut.String()))}
	}
	upload, err := os.MkdirTemp("", "cfgb-upload-*")
	if err != nil {
		return &ExitError{Code: 3, Err: fmt.Errorf("create upload workspace: %w", err)}
	}
	defer func() {
		if removeErr := os.RemoveAll(upload); removeErr != nil {
			err = errors.Join(err, &ExitError{Code: 3, Err: fmt.Errorf("remove upload workspace %s: %w", upload, removeErr)})
		}
	}()
	rawConfig, err := wranglerConfig(cfg, manifest, artifact, upload, workerName, account)
	if err != nil {
		return err
	}
	configPath := filepath.Join(upload, "wrangler.json")
	if err := os.WriteFile(configPath, rawConfig, 0o600); err != nil {
		return &ExitError{Code: 3, Err: fmt.Errorf("write upload configuration: %w", err)}
	}
	args := []string{wrangler, "deploy", "--config", configPath, "--no-bundle"}
	if opts.DryRun {
		args = append(args, "--dry-run", "--outdir", filepath.Join(upload, "bundle"))
	} else {
		// Only a started upload consumes its build session. Preflight failures
		// and dry runs preserve it for correction and the actual deployment.
		defer func() {
			if removeErr := os.RemoveAll(workspace); removeErr != nil {
				err = errors.Join(err, &ExitError{Code: 3, Err: fmt.Errorf("remove toolchain workspace %s: %w", workspace, removeErr)})
			}
		}()
	}
	if err := run(renderer, opts.Stdout, opts.Stderr, extra, "node", args...); err != nil {
		return &ExitError{Code: 3, Err: fmt.Errorf("cloudflare deploy: %w", err)}
	}
	return nil
}

func productionBranch(cfg *config.File) error {
	branch, err := output("git", "-C", cfg.Root(), "branch", "--show-current")
	if err != nil {
		return &ExitError{Code: 3, Err: fmt.Errorf("read deployment branch: %w", err)}
	}
	if branch == "" {
		branch = os.Getenv("WORKERS_CI_BRANCH")
	}
	if branch == "" || branch != cfg.Deploy.ProductionBranch {
		return &ExitError{Code: 1, Err: fmt.Errorf("E_DEPLOY_TARGET: deploy requires branch %q, current branch is %q", cfg.Deploy.ProductionBranch, branch)}
	}
	return nil
}

func readDeployManifest(artifact string) (manifest deployManifest, err error) {
	root, err := os.OpenRoot(artifact)
	if err != nil {
		return manifest, artifactIO(fmt.Errorf("open %s: %w", artifact, err))
	}
	defer func() {
		if closeErr := root.Close(); closeErr != nil {
			err = errors.Join(err, &ExitError{Code: 3, Err: fmt.Errorf("close artifact: %w", closeErr)})
		}
	}()
	raw, err := root.ReadFile("build-manifest.json")
	if err != nil {
		return manifest, artifactIO(fmt.Errorf("read manifest: %w", err))
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return manifest, &ExitError{Code: 1, Err: fmt.Errorf("E_ARTIFACT: decode manifest: %w", err)}
	}
	if manifest.SchemaVersion != 1 || !slices.Contains(manifest.Checks, "render") || !slices.Contains(manifest.Checks, "pagefind") || manifest.Publications == nil {
		return manifest, &ExitError{Code: 1, Err: fmt.Errorf("E_ARTIFACT: unsupported or incomplete build manifest")}
	}
	for _, name := range []string{"site", "worker/index.js"} {
		info, err := root.Stat(name)
		if err != nil {
			return manifest, artifactIO(fmt.Errorf("inspect %s: %w", name, err))
		}
		if (name == "site" && !info.IsDir()) || (name != "site" && !info.Mode().IsRegular()) {
			return manifest, &ExitError{Code: 1, Err: fmt.Errorf("E_ARTIFACT: invalid %s", name)}
		}
	}
	return manifest, nil
}

func publicationGate(publications []publicationRecord, now time.Time) error {
	for _, article := range publications {
		context := article.ArticleKey + "/" + article.Locale
		if strings.TrimSpace(article.Summary) == "" {
			return &ExitError{Code: 1, Err: fmt.Errorf("E_SUMMARY_REQUIRED: %s: publication requires a summary", context)}
		}
		if article.PublishedAt.After(now) || (article.UpdatedAt != nil && article.UpdatedAt.After(now)) {
			return &ExitError{Code: 1, Err: fmt.Errorf("E_FUTURE_DATE: %s: publication timestamp is in the future", context)}
		}
	}
	return nil
}

func uploadToolchain(id string) (string, error) {
	if !strings.HasPrefix(id, "cfgb-build-") || !filepath.IsLocal(id) || strings.ContainsAny(id, `/\`) || filepath.Base(id) != id {
		return "", &ExitError{Code: 2, Err: fmt.Errorf("E_TOOLCHAIN: invalid toolchain session basename %q", id)}
	}
	workspace := filepath.Join(os.TempDir(), id)
	info, err := os.Lstat(workspace)
	if err != nil {
		return "", &ExitError{Code: 2, Err: fmt.Errorf("E_TOOLCHAIN: inspect retained session: %w", err)}
	}
	if !info.IsDir() {
		return "", &ExitError{Code: 2, Err: fmt.Errorf("E_TOOLCHAIN: retained session is not a directory")}
	}
	return workspace, nil
}

func artifactIO(err error) error {
	code := 3
	if errors.Is(err, os.ErrNotExist) {
		code = 1
	}
	return &ExitError{Code: code, Err: fmt.Errorf("E_ARTIFACT: %w", err)}
}

func wranglerConfig(cfg *config.File, manifest deployManifest, artifact, upload, name, account string) ([]byte, error) {
	baseURL := manifest.BaseURL
	if baseURL == "" { // Artifacts created before baseUrl was recorded use current config.
		baseURL = cfg.Site.BaseURL
	}
	origin, err := url.Parse(cfg.Site.BaseURL)
	if err != nil {
		return nil, &ExitError{Code: 2, Err: fmt.Errorf("E_DEPLOY_TARGET: production URL: %w", err)}
	}
	if origin.Hostname() == "" {
		return nil, &ExitError{Code: 2, Err: fmt.Errorf("E_DEPLOY_TARGET: site.baseUrl must supply the production hostname")}
	}
	prefix, err := siteBasePath(baseURL)
	if err != nil {
		return nil, &ExitError{Code: 1, Err: fmt.Errorf("E_ARTIFACT: base URL: %w", err)}
	}
	assets := filepath.Join(artifact, "site")
	if prefix != "" {
		assets, err = mountAssets(assets, upload, prefix)
		if err != nil {
			return nil, &ExitError{Code: 3, Err: fmt.Errorf("stage prefixed assets: %w", err)}
		}
	}
	return json.MarshalIndent(map[string]any{
		"name": name, "account_id": account,
		"main":               filepath.Join(artifact, "worker", "index.js"),
		"compatibility_date": worker.CompatibilityDate,
		"workers_dev":        false, "preview_urls": true, "previews": map[string]any{},
		"routes": []map[string]any{{"pattern": origin.Hostname(), "custom_domain": true}},
		"assets": map[string]any{
			"directory": assets, "binding": "ASSETS",
			"html_handling": "auto-trailing-slash", "not_found_handling": "404-page",
			"run_worker_first": []string{prefix + "/", prefix + "/__locale"},
		},
	}, "", "  ")
}

// Static Assets has no mount-prefix option. Stage prefixed files without
// changing their bytes; _headers and _redirects remain at the assets root.
func mountAssets(site, upload, prefix string) (assets string, err error) {
	path, err := url.PathUnescape(strings.TrimPrefix(prefix, "/"))
	if err != nil {
		return "", err
	}
	assets = filepath.Join(upload, "assets")
	if err := os.Mkdir(assets, 0o700); err != nil {
		return "", err
	}
	src, err := os.OpenRoot(site)
	if err != nil {
		return "", err
	}
	defer func() { err = errors.Join(err, src.Close()) }()
	dest, err := os.OpenRoot(assets)
	if err != nil {
		return "", err
	}
	defer func() { err = errors.Join(err, dest.Close()) }()
	if err := copyRootToRoot(src, ".", dest, filepath.FromSlash(path)); err != nil {
		return "", err
	}
	for _, special := range []string{"_headers", "_redirects", "404.html"} {
		if _, err := src.Stat(special); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return "", err
		}
		if err := copyRootToRoot(src, special, dest, special); err != nil {
			return "", err
		}
		if special != "404.html" {
			if err := dest.Remove(filepath.Join(filepath.FromSlash(path), special)); err != nil {
				return "", err
			}
		}
	}
	return assets, nil
}
