// Package build runs the embedded renderer and writes a CFGB artifact.
package build

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	cfgb "github.com/ymmt2005/cfgb"
	"github.com/ymmt2005/cfgb/internal/config"
	"github.com/ymmt2005/cfgb/internal/version"
	"github.com/ymmt2005/cfgb/internal/worker"
)

// ExitError is a command failure with a CFGB exit code.
type ExitError struct {
	Code int
	Err  error
}

func (e *ExitError) Error() string { return e.Err.Error() }

func (e *ExitError) Unwrap() error { return e.Err }

// Options selects the content directory and artifact path.
type Options struct {
	Dir    string
	Out    string
	Stdout io.Writer
	Stderr io.Writer
}

type requirements struct {
	NodeRange               string `json:"nodeRange"`
	TestedNodeVersion       string `json:"testedNodeVersion"`
	PnpmVersion             string `json:"pnpmVersion"`
	WranglerVersion         string `json:"wranglerVersion"`
	WorkerCompatibilityDate string `json:"workerCompatibilityDate"`
	RendererVersion         string `json:"rendererVersion"`
	LockfileHash            string `json:"lockfileHash"`
}

type siteJSON struct {
	Title         string `json:"title"`
	BaseURL       string `json:"baseUrl"`
	DefaultLocale string `json:"defaultLocale"`
	Timezone      string `json:"timezone"`
	Locales       map[string]struct {
		Label string `json:"label"`
	} `json:"locales"`
	ContentRoot  string `json:"contentRoot"`
	TopicsFile   string `json:"topicsFile"`
	LinkcardsDir string `json:"linkcardsDir"`
	LatestPosts  int    `json:"latestPosts"`
}

// Run builds the site in Dir into Out.
func Run(opts Options) error {
	if opts.Stdout == nil {
		opts.Stdout = io.Discard
	}
	if opts.Stderr == nil {
		opts.Stderr = io.Discard
	}
	cfg, err := config.Load(opts.Dir)
	if err != nil {
		return &ExitError{Code: 2, Err: err}
	}
	out, err := outputDir(cfg, opts.Out)
	if err != nil {
		return &ExitError{Code: 2, Err: err}
	}
	req, err := loadRequirements()
	if err != nil {
		return &ExitError{Code: 2, Err: err}
	}
	nodeVersion, pnpmVersion, err := checkToolchain(req)
	if err != nil {
		return &ExitError{Code: 2, Err: fmt.Errorf("E_TOOLCHAIN: %w", err)}
	}
	session, err := sessionDir()
	if err != nil {
		return &ExitError{Code: 2, Err: fmt.Errorf("E_TOOLCHAIN: %w", err)}
	}
	rendererDir := filepath.Join(session, "renderer")
	if err := extractRenderer(rendererDir); err != nil {
		return &ExitError{Code: 3, Err: err}
	}
	snapshot := filepath.Join(session, "snapshot")
	contentRoot, topicsFile, linkcardsDir, err := stageContent(cfg, snapshot)
	if err != nil {
		return &ExitError{Code: 3, Err: err}
	}
	if err := stageMedia(contentRoot, filepath.Join(rendererDir, "public", "media")); err != nil {
		return &ExitError{Code: 3, Err: err}
	}
	sitePath := filepath.Join(session, "site.json")
	if err := writeSiteJSON(sitePath, cfg, contentRoot, topicsFile, linkcardsDir); err != nil {
		return &ExitError{Code: 3, Err: err}
	}
	routesPath := filepath.Join(session, "routes.json")
	fmt.Fprintf(opts.Stdout, "installing renderer dependencies\n")
	if err := command(rendererDir, opts.Stdout, opts.Stderr, nil, "pnpm", "install", "--frozen-lockfile"); err != nil {
		return &ExitError{Code: 3, Err: fmt.Errorf("pnpm install: %w", err)}
	}
	env := []string{"CFGB_SITE_JSON=" + sitePath, "CFGB_ROUTES_OUT=" + routesPath}
	fmt.Fprintf(opts.Stdout, "rendering\n")
	if err := command(rendererDir, opts.Stdout, opts.Stderr, env, "pnpm", "exec", "astro", "build"); err != nil {
		return &ExitError{Code: 1, Err: fmt.Errorf("render: %w", err)}
	}
	if err := command(rendererDir, opts.Stdout, opts.Stderr, env, "pnpm", "exec", "pagefind", "--site", "dist"); err != nil {
		return &ExitError{Code: 1, Err: fmt.Errorf("pagefind: %w", err)}
	}
	routes, err := readRoutes(routesPath)
	if err != nil {
		return &ExitError{Code: 1, Err: err}
	}
	if err := publish(out, filepath.Join(rendererDir, "dist")); err != nil {
		return &ExitError{Code: 3, Err: err}
	}
	locales := make([]string, 0, len(cfg.Locales))
	for locale := range cfg.Locales {
		locales = append(locales, locale)
	}
	sort.Strings(locales)
	source, err := worker.Source(worker.Options{
		DefaultLocale: cfg.Site.DefaultLocale,
		Locales:       locales,
		Routes:        routes,
	})
	if err != nil {
		return &ExitError{Code: 3, Err: err}
	}
	if err := os.MkdirAll(filepath.Join(out, "worker"), 0o755); err != nil {
		return &ExitError{Code: 3, Err: err}
	}
	if err := os.WriteFile(filepath.Join(out, "worker", "index.js"), source, 0o644); err != nil {
		return &ExitError{Code: 3, Err: err}
	}
	manifest, err := manifestJSON(cfg, req, nodeVersion, pnpmVersion, sessionID(session), routes)
	if err != nil {
		return &ExitError{Code: 3, Err: err}
	}
	if err := os.WriteFile(filepath.Join(out, "build-manifest.json"), manifest, 0o644); err != nil {
		return &ExitError{Code: 3, Err: err}
	}
	fmt.Fprintf(opts.Stdout, "built %s\n", out)
	return nil
}

func loadRequirements() (requirements, error) {
	var req requirements
	raw, err := cfgb.FS.ReadFile("toolchain-requirements.json")
	if err != nil {
		return req, err
	}
	if err := json.Unmarshal(raw, &req); err != nil {
		return req, err
	}
	return req, nil
}

func checkToolchain(req requirements) (string, string, error) {
	node, err := output("node", "-p", "process.versions.node")
	if err != nil {
		return "", "", fmt.Errorf("node is required (%s)", req.NodeRange)
	}
	if !nodeInRange(node, req.NodeRange) {
		return "", "", fmt.Errorf("node %s is outside %s", node, req.NodeRange)
	}
	pnpm, err := output("pnpm", "-v")
	if err != nil {
		return "", "", fmt.Errorf("pnpm %s is required", req.PnpmVersion)
	}
	pnpm = strings.TrimPrefix(pnpm, "v")
	if pnpm != req.PnpmVersion {
		return "", "", fmt.Errorf("pnpm %s is required, found %s", req.PnpmVersion, pnpm)
	}
	return node, pnpm, nil
}

func nodeInRange(version, constraint string) bool {
	parts := strings.Fields(constraint)
	if len(parts) != 2 {
		return false
	}
	return compareVersion(version, strings.TrimPrefix(parts[0], ">=")) >= 0 &&
		compareVersion(version, strings.TrimPrefix(parts[1], "<")) < 0
}

func compareVersion(left, right string) int {
	lp := parseVersion(left)
	rp := parseVersion(right)
	for i := 0; i < 3; i++ {
		if lp[i] != rp[i] {
			if lp[i] < rp[i] {
				return -1
			}
			return 1
		}
	}
	return 0
}

func parseVersion(value string) [3]int {
	var out [3]int
	for i, part := range strings.Split(value, ".") {
		if i > 2 {
			break
		}
		fmt.Sscanf(part, "%d", &out[i])
	}
	return out
}

func sessionDir() (string, error) {
	id, err := newSessionID()
	if err != nil {
		return "", err
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(cache, "cfgb", "builds", id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func newSessionID() (string, error) {
	if raw := os.Getenv("WORKERS_CI_BUILD_UUID"); raw != "" {
		sum := sha256.Sum256([]byte(raw))
		return hex.EncodeToString(sum[:]), nil
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func sessionID(dir string) string {
	return filepath.Base(dir)
}

func extractRenderer(dest string) error {
	return fs.WalkDir(cfgb.FS, "renderer", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(name, "renderer")
		rel = strings.TrimPrefix(rel, "/")
		target := filepath.Join(dest, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		raw, err := cfgb.FS.ReadFile(name)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, raw, 0o644)
	})
}

func stageContent(cfg *config.File, snapshot string) (string, string, string, error) {
	base := filepath.Dir(cfg.Path())
	content := resolveInRepo(cfg.Root(), base, cfg.Content.Root)
	topics := resolveInRepo(cfg.Root(), base, cfg.Content.Topics)
	cards := resolveInRepo(cfg.Root(), base, cfg.Content.Linkcards)
	contentDest := filepath.Join(snapshot, "content")
	topicsDest := filepath.Join(snapshot, "topics.yaml")
	cardsDest := filepath.Join(snapshot, "linkcards")
	if err := copyPath(content, contentDest); err != nil {
		return "", "", "", err
	}
	if err := copyPath(topics, topicsDest); err != nil {
		return "", "", "", err
	}
	if err := copyPath(cards, cardsDest); err != nil {
		return "", "", "", err
	}
	return contentDest, topicsDest, cardsDest, nil
}

func resolveInRepo(repo, base, value string) string {
	if filepath.IsAbs(value) {
		return value
	}
	return filepath.Join(base, value)
}

func stageMedia(contentRoot, dest string) error {
	posts := filepath.Join(contentRoot, "posts")
	return filepath.WalkDir(posts, func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.Contains(name, string(filepath.Separator)+"assets"+string(filepath.Separator)) {
			return nil
		}
		rel, err := filepath.Rel(posts, name)
		if err != nil {
			return err
		}
		// YEAR/KEY/assets/file -> YEAR/KEY/file
		parts := strings.Split(rel, string(filepath.Separator))
		if len(parts) < 4 || parts[2] != "assets" {
			return nil
		}
		target := filepath.Join(append([]string{dest}, append(parts[:2], parts[3:]...)...)...)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return copyPath(name, target)
	})
}

func writeSiteJSON(path string, cfg *config.File, contentRoot, topicsFile, linkcardsDir string) error {
	locales := map[string]struct {
		Label string `json:"label"`
	}{}
	for locale, item := range cfg.Locales {
		locales[locale] = struct {
			Label string `json:"label"`
		}{Label: item.Label}
	}
	raw, err := json.Marshal(siteJSON{
		Title:         cfg.Site.Title,
		BaseURL:       cfg.Site.BaseURL,
		DefaultLocale: cfg.Site.DefaultLocale,
		Timezone:      cfg.Site.Timezone,
		Locales:       locales,
		ContentRoot:   contentRoot,
		TopicsFile:    topicsFile,
		LinkcardsDir:  linkcardsDir,
		LatestPosts:   cfg.Home.LatestPosts,
	})
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}

func readRoutes(path string) ([]string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var routes []string
	if err := json.Unmarshal(raw, &routes); err != nil {
		return nil, err
	}
	return routes, nil
}

func outputDir(cfg *config.File, out string) (string, error) {
	if out == "" {
		out = "dist"
	}
	if !filepath.IsAbs(out) {
		out = filepath.Join(filepath.Dir(cfg.Path()), out)
	}
	out = filepath.Clean(out)
	rel, err := filepath.Rel(cfg.Root(), out)
	if err != nil || strings.HasPrefix(rel, "..") || rel == "." {
		return "", fmt.Errorf("--out must be a directory inside the repository")
	}
	contentRel, _ := filepath.Rel(cfg.Root(), filepath.Join(filepath.Dir(cfg.Path()), cfg.Content.Root))
	if rel == contentRel || strings.HasPrefix(rel, contentRel+string(filepath.Separator)) {
		return "", fmt.Errorf("--out must be outside the content root")
	}
	if rel == ".git" || strings.HasPrefix(rel, ".git"+string(filepath.Separator)) {
		return "", fmt.Errorf("--out must not be inside .git")
	}
	info, err := os.Stat(out)
	if err != nil {
		if os.IsNotExist(err) {
			return out, os.MkdirAll(out, 0o755)
		}
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("--out is not a directory")
	}
	entries, err := os.ReadDir(out)
	if err != nil {
		return "", err
	}
	if len(entries) == 0 {
		return out, nil
	}
	if _, err := os.Stat(filepath.Join(out, "build-manifest.json")); err != nil {
		return "", fmt.Errorf("--out is not empty and is not a previous CFGB artifact")
	}
	return out, nil
}

func publish(out, dist string) error {
	staging := out + ".tmp"
	_ = os.RemoveAll(staging)
	if err := os.MkdirAll(filepath.Join(staging, "site"), 0o755); err != nil {
		return err
	}
	if err := copyPath(dist, filepath.Join(staging, "site")); err != nil {
		return err
	}
	if err := os.RemoveAll(out); err != nil {
		return err
	}
	return os.Rename(staging, out)
}

func manifestJSON(cfg *config.File, req requirements, nodeVersion, pnpmVersion, session string, routes []string) ([]byte, error) {
	commit, branch, dirty := gitState(cfg.Root())
	manifest := map[string]any{
		"schemaVersion":      1,
		"cfgbVersion":        version.Version,
		"rendererVersion":    req.RendererVersion,
		"toolchainSessionId": session,
		"source": map[string]any{
			"commit": commit,
			"branch": branch,
			"dirty":  dirty,
		},
		"checks": []string{"render", "pagefind"},
		"routes": routes,
		"toolchain": map[string]any{
			"nodeRange":               req.NodeRange,
			"testedNodeVersion":       req.TestedNodeVersion,
			"pnpmVersion":             req.PnpmVersion,
			"wranglerVersion":         req.WranglerVersion,
			"workerCompatibilityDate": req.WorkerCompatibilityDate,
			"rendererVersion":         req.RendererVersion,
			"lockfileHash":            req.LockfileHash,
			"nodeVersion":             nodeVersion,
			"observedPnpmVersion":     pnpmVersion,
		},
	}
	if raw := os.Getenv("WORKERS_CI_BUILD_UUID"); raw != "" {
		manifest["buildUUID"] = raw
	}
	return json.MarshalIndent(manifest, "", "  ")
}

func gitState(repo string) (string, string, bool) {
	commit, err := output("git", "-C", repo, "rev-parse", "HEAD")
	if err != nil {
		return "", "", true
	}
	branch, _ := output("git", "-C", repo, "rev-parse", "--abbrev-ref", "HEAD")
	status, _ := output("git", "-C", repo, "status", "--porcelain")
	return commit, branch, strings.TrimSpace(status) != ""
}

func command(dir string, stdout, stderr io.Writer, extra []string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Env = append(os.Environ(), extra...)
	return cmd.Run()
}

func output(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	raw, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(raw)), nil
}

func copyPath(src, dest string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return copyDir(src, dest)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func copyDir(src, dest string) error {
	return filepath.WalkDir(src, func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, name)
		if err != nil {
			return err
		}
		target := filepath.Join(dest, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyPath(name, target)
	})
}
