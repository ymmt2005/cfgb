// Package build runs the embedded renderer and writes a CFGB artifact.
package build

import (
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
	"github.com/ymmt2005/cfgb/internal/frontmatter"
	"github.com/ymmt2005/cfgb/internal/version"
	"github.com/ymmt2005/cfgb/internal/worker"
)

// minimumNpmVersion is the oldest npm whose install scripts are opt-in.
const minimumNpmVersion = "12.0.0"

// minimumPnpmVersion is the oldest optional pnpm release CFGB accepts.
const minimumPnpmVersion = "11.0.0"

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
	PackageManager          string `json:"packageManager"`
	NpmVersion              string `json:"npmVersion"`
	PnpmVersion             string `json:"pnpmVersion"`
	WranglerVersion         string `json:"wranglerVersion"`
	WorkerCompatibilityDate string `json:"workerCompatibilityDate"`
	RendererVersion         string `json:"rendererVersion"`
	LockfileHash            string `json:"lockfileHash"`
	PnpmLockfileHash        string `json:"pnpmLockfileHash"`
}

// toolchainCheck is the package manager selected for this build.
type toolchainCheck struct {
	Node           string
	PackageManager string
	NpmVersion     string
	PnpmVersion    string
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
	MetadataFile string `json:"metadataFile"`
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
	tc, err := checkToolchain(req)
	if err != nil {
		return &ExitError{Code: 2, Err: fmt.Errorf("E_TOOLCHAIN: %w", err)}
	}
	if err := resetOutput(out); err != nil {
		return &ExitError{Code: 3, Err: err}
	}
	workspace, err := newWorkspace()
	if err != nil {
		_ = os.RemoveAll(out)
		return &ExitError{Code: 2, Err: fmt.Errorf("E_TOOLCHAIN: %w", err)}
	}
	built := false
	defer discardFailedBuild(out, workspace, &built)
	rendererDir := filepath.Join(workspace, "renderer")
	if err := extractRenderer(rendererDir); err != nil {
		return &ExitError{Code: 3, Err: err}
	}
	snapshot := filepath.Join(workspace, "snapshot")
	contentRoot, topicsFile, linkcardsDir, err := stageContent(cfg, snapshot)
	if err != nil {
		return &ExitError{Code: 3, Err: err}
	}
	if err := stageMedia(cfg, filepath.Join(rendererDir, "public", "media")); err != nil {
		return &ExitError{Code: 3, Err: err}
	}
	locales := make([]string, 0, len(cfg.Locales))
	for locale := range cfg.Locales {
		locales = append(locales, locale)
	}
	sort.Strings(locales)
	index, err := frontmatter.Collect(contentRoot, topicsFile, locales)
	if err != nil {
		return &ExitError{Code: 3, Err: err}
	}
	metadataPath := filepath.Join(workspace, "metadata.json")
	metadata, err := json.Marshal(index)
	if err != nil {
		return &ExitError{Code: 3, Err: err}
	}
	if err := os.WriteFile(metadataPath, metadata, 0o644); err != nil {
		return &ExitError{Code: 3, Err: err}
	}
	sitePath := filepath.Join(workspace, "site.json")
	if err := writeSiteJSON(sitePath, cfg, contentRoot, topicsFile, linkcardsDir, metadataPath); err != nil {
		return &ExitError{Code: 3, Err: err}
	}
	routesPath := filepath.Join(workspace, "routes.json")
	fmt.Fprintf(opts.Stdout, "installing renderer dependencies with %s\n", tc.PackageManager)
	if err := installRenderer(rendererDir, opts.Stdout, opts.Stderr, tc); err != nil {
		return &ExitError{Code: 3, Err: err}
	}
	env := []string{"CFGB_SITE_JSON=" + sitePath, "CFGB_ROUTES_OUT=" + routesPath}
	fmt.Fprintf(opts.Stdout, "rendering\n")
	if err := renderSite(rendererDir, opts.Stdout, opts.Stderr, env, tc); err != nil {
		return &ExitError{Code: 1, Err: err}
	}
	routes, err := readRoutes(routesPath)
	if err != nil {
		return &ExitError{Code: 1, Err: err}
	}
	source, err := worker.Source(worker.Options{
		DefaultLocale: cfg.Site.DefaultLocale,
		Locales:       locales,
		Routes:        routes,
	})
	if err != nil {
		return &ExitError{Code: 3, Err: err}
	}
	manifest, err := manifestJSON(cfg, req, tc, workspace, routes, out)
	if err != nil {
		return &ExitError{Code: 3, Err: err}
	}
	if err := deliver(out, filepath.Join(rendererDir, "dist"), source, manifest); err != nil {
		return &ExitError{Code: 3, Err: err}
	}
	built = true
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

func packageManager() (string, error) {
	switch strings.TrimSpace(os.Getenv("CFGB_PACKAGE_MANAGER")) {
	case "", "npm":
		return "npm", nil
	case "pnpm":
		return "pnpm", nil
	default:
		return "", fmt.Errorf("CFGB_PACKAGE_MANAGER must be npm or pnpm")
	}
}

func checkToolchain(req requirements) (toolchainCheck, error) {
	var tc toolchainCheck
	node, err := output("node", "-p", "process.versions.node")
	if err != nil {
		return tc, fmt.Errorf("node is required (%s)", req.NodeRange)
	}
	if !nodeInRange(node, req.NodeRange) {
		return tc, fmt.Errorf("node %s is outside %s", node, req.NodeRange)
	}
	manager, err := packageManager()
	if err != nil {
		return tc, err
	}
	tc.Node = node
	tc.PackageManager = manager
	switch manager {
	case "npm":
		npm, err := output("npm", "-v")
		if err != nil {
			return tc, fmt.Errorf("npm >= %s is required (tested %s)", minimumNpmVersion, req.NpmVersion)
		}
		npm = strings.TrimPrefix(npm, "v")
		if compareVersion(npm, minimumNpmVersion) < 0 {
			return tc, fmt.Errorf("npm >= %s is required (tested %s), found %s", minimumNpmVersion, req.NpmVersion, npm)
		}
		tc.NpmVersion = npm
	case "pnpm":
		pnpm, err := output("pnpm", "-v")
		if err != nil {
			return tc, fmt.Errorf("pnpm >= %s is required (tested %s)", minimumPnpmVersion, req.PnpmVersion)
		}
		pnpm = strings.TrimPrefix(pnpm, "v")
		if compareVersion(pnpm, minimumPnpmVersion) < 0 {
			return tc, fmt.Errorf("pnpm >= %s is required (tested %s), found %s", minimumPnpmVersion, req.PnpmVersion, pnpm)
		}
		tc.PnpmVersion = pnpm
	}
	return tc, nil
}

func installRenderer(dir string, stdout, stderr io.Writer, tc toolchainCheck) error {
	switch tc.PackageManager {
	case "npm":
		if err := command(dir, stdout, stderr, nil, "npm", "ci"); err != nil {
			return fmt.Errorf("npm ci: %w", err)
		}
	case "pnpm":
		if err := command(dir, stdout, stderr, []string{"COREPACK_ENABLE_AUTO_PIN=0"}, "pnpm", "install", "--frozen-lockfile"); err != nil {
			return fmt.Errorf("pnpm install: %w", err)
		}
	default:
		return fmt.Errorf("unsupported package manager %s", tc.PackageManager)
	}
	return nil
}

func renderSite(dir string, stdout, stderr io.Writer, extra []string, tc toolchainCheck) error {
	env := extra
	if tc.PackageManager == "pnpm" {
		env = append([]string{"COREPACK_ENABLE_AUTO_PIN=0"}, extra...)
	}
	switch tc.PackageManager {
	case "npm":
		if err := command(dir, stdout, stderr, env, "npm", "exec", "--", "astro", "build"); err != nil {
			return fmt.Errorf("render: %w", err)
		}
		if err := command(dir, stdout, stderr, env, "npm", "exec", "--", "pagefind", "--site", "dist"); err != nil {
			return fmt.Errorf("pagefind: %w", err)
		}
	case "pnpm":
		if err := command(dir, stdout, stderr, env, "pnpm", "exec", "astro", "build"); err != nil {
			return fmt.Errorf("render: %w", err)
		}
		if err := command(dir, stdout, stderr, env, "pnpm", "exec", "pagefind", "--site", "dist"); err != nil {
			return fmt.Errorf("pagefind: %w", err)
		}
	default:
		return fmt.Errorf("unsupported package manager %s", tc.PackageManager)
	}
	return nil
}

func nodeInRange(version, constraint string) bool {
	for _, clause := range strings.Split(constraint, "||") {
		if nodeMatchesClause(version, strings.TrimSpace(clause)) {
			return true
		}
	}
	return false
}

func nodeMatchesClause(version, clause string) bool {
	parts := strings.Fields(clause)
	if len(parts) == 0 {
		return false
	}
	for _, part := range parts {
		switch {
		case strings.HasPrefix(part, ">="):
			if compareVersion(version, strings.TrimPrefix(part, ">=")) < 0 {
				return false
			}
		case strings.HasPrefix(part, "<="):
			if compareVersion(version, strings.TrimPrefix(part, "<=")) > 0 {
				return false
			}
		case strings.HasPrefix(part, ">"):
			if compareVersion(version, strings.TrimPrefix(part, ">")) <= 0 {
				return false
			}
		case strings.HasPrefix(part, "<"):
			if compareVersion(version, strings.TrimPrefix(part, "<")) >= 0 {
				return false
			}
		default:
			return false
		}
	}
	return true
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

// newWorkspace creates this invocation's toolchain directory. MkdirTemp uses
// mode 0700 before umask. The build identifier does not choose the name.
func newWorkspace() (string, error) {
	return os.MkdirTemp("", "cfgb-build-*")
}

// discardFailedBuild removes the incomplete output and the workspace created by
// this invocation. A successful build leaves both in place.
func discardFailedBuild(out, workspace string, built *bool) {
	if built != nil && *built {
		return
	}
	_ = os.RemoveAll(out)
	_ = os.RemoveAll(workspace)
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
	root, err := os.OpenRoot(cfg.Root())
	if err != nil {
		return "", "", "", err
	}
	defer root.Close()
	base := filepath.Dir(cfg.Path())
	content, err := repoRelative(cfg.Root(), base, cfg.Content.Root)
	if err != nil {
		return "", "", "", err
	}
	topics, err := repoRelative(cfg.Root(), base, cfg.Content.Topics)
	if err != nil {
		return "", "", "", err
	}
	cards, err := repoRelative(cfg.Root(), base, cfg.Content.Linkcards)
	if err != nil {
		return "", "", "", err
	}
	contentDest := filepath.Join(snapshot, "content")
	topicsDest := filepath.Join(snapshot, "topics.yaml")
	cardsDest := filepath.Join(snapshot, "linkcards")
	if err := copyFromRoot(root, content, contentDest); err != nil {
		return "", "", "", err
	}
	if err := copyFromRoot(root, topics, topicsDest); err != nil {
		return "", "", "", err
	}
	info, err := root.Stat(cards)
	if err != nil {
		if !os.IsNotExist(err) {
			return "", "", "", err
		}
	} else if !info.IsDir() {
		return "", "", "", fmt.Errorf("link card cache is not a directory")
	} else if err := copyFromRoot(root, cards, cardsDest); err != nil {
		return "", "", "", err
	}
	return contentDest, topicsDest, cardsDest, nil
}

// repoRelative converts a config path into a name for os.Root. Absolute paths
// are expressed relative to the repository. Escapes are left for Root to reject.
func repoRelative(repo, base, value string) (string, error) {
	abs := value
	if !filepath.IsAbs(value) {
		abs = filepath.Join(base, value)
	}
	rel, err := filepath.Rel(filepath.Clean(repo), filepath.Clean(abs))
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(rel), nil
}

func stageMedia(cfg *config.File, dest string) error {
	root, err := os.OpenRoot(cfg.Root())
	if err != nil {
		return err
	}
	defer root.Close()
	content, err := repoRelative(cfg.Root(), filepath.Dir(cfg.Path()), cfg.Content.Root)
	if err != nil {
		return err
	}
	posts, err := root.OpenRoot(joinRoot(content, "posts"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer posts.Close()
	years, err := readRootDir(posts, ".")
	if err != nil {
		return err
	}
	for _, year := range years {
		if !isArticleYear(year.Name()) {
			continue
		}
		yearDir, err := rootedIsDir(posts, year.Name())
		if err != nil {
			return err
		}
		if !yearDir {
			continue
		}
		keys, err := readRootDir(posts, year.Name())
		if err != nil {
			return err
		}
		for _, key := range keys {
			articleDir, err := rootedIsDir(posts, joinRoot(year.Name(), key.Name()))
			if err != nil {
				return err
			}
			if !articleDir {
				continue
			}
			group, err := posts.OpenRoot(joinRoot(year.Name(), key.Name()))
			if err != nil {
				return err
			}
			err = copyArticleAssets(group, filepath.Join(dest, year.Name(), key.Name()))
			group.Close()
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func copyArticleAssets(group *os.Root, dest string) error {
	info, err := group.Stat("assets")
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return nil
	}
	assets, err := group.OpenRoot("assets")
	if err != nil {
		return err
	}
	defer assets.Close()
	return copyFromRoot(assets, ".", dest)
}

func readRootDir(root *os.Root, name string) ([]fs.DirEntry, error) {
	file, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return file.ReadDir(-1)
}

func writeSiteJSON(path string, cfg *config.File, contentRoot, topicsFile, linkcardsDir, metadataFile string) error {
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
		MetadataFile:  metadataFile,
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
	repo := filepath.Clean(cfg.Root())
	if !insideRepo(repo, out) {
		return "", fmt.Errorf("--out must be a directory inside the repository")
	}
	base := filepath.Dir(cfg.Path())
	protected := []struct {
		path string
		dir  bool
	}{
		{resolveAbs(base, cfg.Content.Root), true},
		{resolveAbs(base, cfg.Content.Linkcards), true},
		{filepath.Join(repo, ".git"), true},
		{resolveAbs(base, cfg.Content.Topics), false},
		{cfg.Path(), false},
	}
	for _, item := range protected {
		if overlaps(out, item.path, item.dir) {
			return "", fmt.Errorf("--out must not contain or sit inside source content or Git and configuration inputs")
		}
	}
	return out, nil
}

// resetOutput removes the selected output entry and creates <out>/.tmp.
// RemoveAll deletes a symlink entry without following it, so a link is not
// used as the deletion target.
func resetOutput(out string) error {
	if err := os.RemoveAll(out); err != nil {
		return err
	}
	return os.MkdirAll(filepath.Join(out, ".tmp"), 0o755)
}

func resolveAbs(base, value string) string {
	if filepath.IsAbs(value) {
		return filepath.Clean(value)
	}
	return filepath.Clean(filepath.Join(base, value))
}

func insideRepo(repo, path string) bool {
	rel, err := filepath.Rel(repo, filepath.Clean(path))
	if err != nil {
		return false
	}
	return rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func overlaps(out, target string, targetIsDir bool) bool {
	out = filepath.Clean(out)
	target = filepath.Clean(target)
	if containsPath(out, target) {
		return true
	}
	return targetIsDir && containsPath(target, out)
}

func containsPath(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// deliver writes the complete artifact under the staging directory created by
// resetOutput, then promotes site/, worker/, and build-manifest.json. It does
// not remove <out> and does not touch a sibling <out>.tmp.
func deliver(out, dist string, workerSource, manifest []byte) error {
	return stageOutput(out, dist, workerSource, manifest)
}

func stageOutput(out, dist string, workerSource, manifest []byte) error {
	root, err := os.OpenRoot(out)
	if err != nil {
		return err
	}
	defer root.Close()
	if err := root.Mkdir(".tmp", 0o755); err != nil && !os.IsExist(err) {
		return err
	}
	defer root.RemoveAll(".tmp")
	staging, err := root.OpenRoot(".tmp")
	if err != nil {
		return err
	}
	writeErr := writeStaged(staging, dist, workerSource, manifest)
	staging.Close()
	if writeErr != nil {
		return writeErr
	}
	if err := root.Rename(".tmp/site", "site"); err != nil {
		return err
	}
	if err := root.Rename(".tmp/worker", "worker"); err != nil {
		return err
	}
	if err := root.Rename(".tmp/build-manifest.json", "build-manifest.json"); err != nil {
		return err
	}
	return root.RemoveAll(".tmp")
}

func writeStaged(staging *os.Root, dist string, workerSource, manifest []byte) error {
	site, err := os.OpenRoot(dist)
	if err != nil {
		return err
	}
	defer site.Close()
	if err := copyRootToRoot(site, ".", staging, "site"); err != nil {
		return err
	}
	if err := staging.Mkdir("worker", 0o755); err != nil {
		return err
	}
	if err := staging.WriteFile("worker/index.js", workerSource, 0o644); err != nil {
		return err
	}
	if err := staging.WriteFile("build-manifest.json", manifest, 0o644); err != nil {
		return err
	}
	return nil
}

func rootedIsDir(root *os.Root, name string) (bool, error) {
	info, err := root.Stat(name)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return info.IsDir(), nil
}

func isArticleYear(name string) bool {
	if len(name) != 4 {
		return false
	}
	for _, r := range name {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func manifestJSON(cfg *config.File, req requirements, tc toolchainCheck, workspace string, routes []string, out string) ([]byte, error) {
	commit, branch, dirty := gitState(cfg.Root(), out)
	manifest := map[string]any{
		"schemaVersion":      1,
		"cfgbVersion":        version.Version,
		"rendererVersion":    req.RendererVersion,
		"toolchainSessionId": filepath.Base(workspace),
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
			"packageManager":          tc.PackageManager,
			"npmVersion":              req.NpmVersion,
			"pnpmVersion":             req.PnpmVersion,
			"wranglerVersion":         req.WranglerVersion,
			"workerCompatibilityDate": req.WorkerCompatibilityDate,
			"rendererVersion":         req.RendererVersion,
			"lockfileHash":            req.LockfileHash,
			"pnpmLockfileHash":        req.PnpmLockfileHash,
			"nodeVersion":             tc.Node,
			"observedNpmVersion":      tc.NpmVersion,
			"observedPnpmVersion":     tc.PnpmVersion,
		},
	}
	if raw := os.Getenv("WORKERS_CI_BUILD_UUID"); raw != "" {
		manifest["buildUUID"] = raw
	}
	return json.MarshalIndent(manifest, "", "  ")
}

func gitState(repo, out string) (string, string, bool) {
	commit, err := output("git", "-C", repo, "rev-parse", "HEAD")
	if err != nil {
		return "", "", true
	}
	branch, _ := output("git", "-C", repo, "rev-parse", "--abbrev-ref", "HEAD")
	args := []string{"-C", repo, "status", "--porcelain", "--", "."}
	rel, relErr := filepath.Rel(filepath.Clean(repo), filepath.Clean(out))
	if relErr == nil && filepath.IsLocal(rel) {
		args = append(args, ":(top,literal,exclude)"+filepath.ToSlash(rel))
	}
	status, err := output("git", args...)
	if err != nil {
		return commit, branch, true
	}
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

func copyFromRoot(root *os.Root, name, dest string) error {
	return copyRootPath(root, name, dest, 0)
}

func copyRootPath(root *os.Root, name, dest string, depth int) error {
	if depth > 64 {
		return fmt.Errorf("directory is too deep or cyclic: %s", name)
	}
	file, err := root.Open(name)
	if err != nil {
		return err
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return err
	}
	if !info.IsDir() {
		defer file.Close()
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		out, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		defer out.Close()
		_, err = io.Copy(out, file)
		return err
	}
	entries, err := file.ReadDir(-1)
	file.Close()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	for _, entry := range entries {
		child := joinRoot(name, entry.Name())
		if err := copyRootPath(root, child, filepath.Join(dest, entry.Name()), depth+1); err != nil {
			return err
		}
	}
	return nil
}

func copyRootToRoot(src *os.Root, name string, dest *os.Root, to string) error {
	return copyRootToRootDepth(src, name, dest, to, 0)
}

func copyRootToRootDepth(src *os.Root, name string, dest *os.Root, to string, depth int) error {
	if depth > 64 {
		return fmt.Errorf("directory is too deep or cyclic: %s", name)
	}
	file, err := src.Open(name)
	if err != nil {
		return err
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return err
	}
	if !info.IsDir() {
		defer file.Close()
		data, err := io.ReadAll(file)
		if err != nil {
			return err
		}
		return dest.WriteFile(to, data, 0o644)
	}
	entries, err := file.ReadDir(-1)
	file.Close()
	if err != nil {
		return err
	}
	if err := dest.MkdirAll(to, 0o755); err != nil {
		return err
	}
	for _, entry := range entries {
		if err := copyRootToRootDepth(src, joinRoot(name, entry.Name()), dest, joinRoot(to, entry.Name()), depth+1); err != nil {
			return err
		}
	}
	return nil
}

func joinRoot(parent, name string) string {
	parent = strings.TrimSuffix(filepath.ToSlash(parent), "/")
	name = filepath.ToSlash(name)
	if parent == "" || parent == "." {
		return name
	}
	return parent + "/" + name
}
