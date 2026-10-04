// Package build runs the embedded renderer and writes a CFGB artifact.
package build

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

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

// minimumWranglerVersion is the oldest Wrangler that can create Worker Previews.
const minimumWranglerVersion = "4.135.0"

// testedNodeVersion, testedNpmVersion, and testedPnpmVersion are the releases
// exercised by CI. They are recorded in the manifest and named in errors.
const (
	testedNodeVersion = "24.21.0"
	testedNpmVersion  = "12.2.0"
	testedPnpmVersion = "12.8.1"
)

// ExitError is a command failure with a CFGB exit code.
type ExitError struct {
	Code int
	Err  error
}

func (e *ExitError) Error() string { return e.Err.Error() }

func (e *ExitError) Unwrap() error { return e.Err }

// Options selects the discovery directory, optional configuration file and
// artifact path. A relative Config is relative to Dir; content and Out paths
// are relative to the selected configuration's directory.
type Options struct {
	Dir    string
	Config string
	Out    string
	Stdout io.Writer
	Stderr io.Writer
}

// releasePins are read from the embedded renderer package.
type releasePins struct {
	NodeRange       string
	RendererVersion string
	WranglerVersion string
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
func Run(opts Options) (err error) {
	if opts.Stdout == nil {
		opts.Stdout = io.Discard
	}
	if opts.Stderr == nil {
		opts.Stderr = io.Discard
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
		return &ExitError{Code: 2, Err: err}
	}
	if err := cfg.ValidateSite(); err != nil {
		return &ExitError{Code: 2, Err: err}
	}
	out, err := outputDir(cfg, opts.Out)
	if err != nil {
		return &ExitError{Code: 2, Err: err}
	}
	pins, err := loadToolchain()
	if err != nil {
		return &ExitError{Code: 2, Err: fmt.Errorf("E_TOOLCHAIN: %w", err)}
	}
	tc, err := checkToolchain(pins)
	if err != nil {
		return &ExitError{Code: 2, Err: fmt.Errorf("E_TOOLCHAIN: %w", err)}
	}
	if err := resetOutput(out); err != nil {
		return &ExitError{Code: 3, Err: err}
	}
	workspace, err := newWorkspace()
	if err != nil {
		return errors.Join(&ExitError{Code: 2, Err: fmt.Errorf("E_TOOLCHAIN: %w", err)}, discardFailedBuild(out, "", nil))
	}
	built := false
	defer func() {
		if cleanupErr := discardFailedBuild(out, workspace, &built); cleanupErr != nil {
			err = errors.Join(err, &ExitError{Code: 3, Err: cleanupErr})
		}
	}()
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
		return contentError(err)
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
	if _, err := fmt.Fprintf(opts.Stdout, "installing renderer dependencies with %s\n", tc.PackageManager); err != nil {
		return &ExitError{Code: 3, Err: fmt.Errorf("write build progress: %w", err)}
	}
	if err := installRenderer(rendererDir, opts.Stdout, opts.Stderr, tc); err != nil {
		return &ExitError{Code: 3, Err: err}
	}
	env := []string{"CFGB_SITE_JSON=" + sitePath, "CFGB_ROUTES_OUT=" + routesPath}
	if _, err := fmt.Fprintln(opts.Stdout, "rendering"); err != nil {
		return &ExitError{Code: 3, Err: fmt.Errorf("write build progress: %w", err)}
	}
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
	manifest, err := manifestJSON(cfg, pins, tc, workspace, routes, out, index)
	if err != nil {
		return &ExitError{Code: 3, Err: err}
	}
	if err := deliver(out, filepath.Join(rendererDir, "dist"), source, manifest); err != nil {
		return &ExitError{Code: 3, Err: err}
	}
	built = true
	if _, err := fmt.Fprintf(opts.Stdout, "built %s\n", out); err != nil {
		return &ExitError{Code: 3, Err: fmt.Errorf("write build result: %w", err)}
	}
	return nil
}

func loadToolchain() (releasePins, error) {
	var pins releasePins
	raw, err := cfgb.FS.ReadFile("renderer/package.json")
	if err != nil {
		return pins, err
	}
	var pkg struct {
		Version string `json:"version"`
		Engines struct {
			Node string `json:"node"`
		} `json:"engines"`
		Dependencies map[string]string `json:"dependencies"`
	}
	if err := json.Unmarshal(raw, &pkg); err != nil {
		return pins, err
	}
	if strings.TrimSpace(pkg.Engines.Node) == "" {
		return pins, fmt.Errorf("renderer engines.node is required")
	}
	wrangler := pkg.Dependencies["wrangler"]
	parsed, err := parseSemver(wrangler)
	if err != nil {
		return pins, fmt.Errorf("renderer wrangler dependency must be an exact semantic version, found %q", wrangler)
	}
	floor, err := parseSemver(minimumWranglerVersion)
	if err != nil {
		return pins, err
	}
	if compareSemver(parsed, floor) < 0 {
		return pins, fmt.Errorf("wrangler >= %s is required, found %s", minimumWranglerVersion, wrangler)
	}
	pins.NodeRange = pkg.Engines.Node
	pins.RendererVersion = pkg.Version
	pins.WranglerVersion = wrangler
	return pins, nil
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

func checkToolchain(pins releasePins) (toolchainCheck, error) {
	var tc toolchainCheck
	node, err := output("node", "-p", "process.versions.node")
	if err != nil {
		return tc, fmt.Errorf("node is required (%s): %w", pins.NodeRange, err)
	}
	if _, err := parseSemver(node); err != nil {
		return tc, fmt.Errorf("node version %q is not a valid semantic version", node)
	}
	if !nodeInRange(node, pins.NodeRange) {
		return tc, fmt.Errorf("node %s is outside %s", node, pins.NodeRange)
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
			return tc, fmt.Errorf("npm >= %s is required (tested %s): %w", minimumNpmVersion, testedNpmVersion, err)
		}
		npm = strings.TrimPrefix(npm, "v")
		if err := requireMinimumVersion("npm", npm, minimumNpmVersion, testedNpmVersion); err != nil {
			return tc, err
		}
		tc.NpmVersion = npm
	case "pnpm":
		pnpm, err := output("pnpm", "-v")
		if err != nil {
			return tc, fmt.Errorf("pnpm >= %s is required (tested %s): %w", minimumPnpmVersion, testedPnpmVersion, err)
		}
		pnpm = strings.TrimPrefix(pnpm, "v")
		if err := requireMinimumVersion("pnpm", pnpm, minimumPnpmVersion, testedPnpmVersion); err != nil {
			return tc, err
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

func requireMinimumVersion(tool, found, minimum, tested string) error {
	parsed, err := parseSemver(found)
	if err != nil {
		return fmt.Errorf("%s version %q is not a valid semantic version", tool, found)
	}
	floor, err := parseSemver(minimum)
	if err != nil {
		return err
	}
	// A stable floor does not admit any prerelease, including one whose
	// numeric version is already above that floor.
	bounds := []semverBound{{op: ">=", version: floor}}
	if !stableRangeAllows(parsed, bounds) || compareSemver(parsed, floor) < 0 {
		return fmt.Errorf("%s >= %s is required (tested %s), found %s", tool, minimum, tested, found)
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
	parsed, err := parseSemver(version)
	if err != nil {
		return false
	}
	parts := strings.Fields(clause)
	if len(parts) == 0 {
		return false
	}
	bounds := make([]semverBound, 0, len(parts))
	for _, part := range parts {
		op, raw, ok := splitComparator(part)
		if !ok {
			return false
		}
		bound, err := parseBound(raw)
		if err != nil {
			return false
		}
		bounds = append(bounds, semverBound{op: op, version: bound})
	}
	if !stableRangeAllows(parsed, bounds) {
		return false
	}
	for _, bound := range bounds {
		if !compareOp(parsed, bound.op, bound.version) {
			return false
		}
	}
	return true
}

func splitComparator(part string) (string, string, bool) {
	switch {
	case strings.HasPrefix(part, ">="):
		return ">=", strings.TrimPrefix(part, ">="), true
	case strings.HasPrefix(part, "<="):
		return "<=", strings.TrimPrefix(part, "<="), true
	case strings.HasPrefix(part, ">"):
		return ">", strings.TrimPrefix(part, ">"), true
	case strings.HasPrefix(part, "<"):
		return "<", strings.TrimPrefix(part, "<"), true
	default:
		return "", "", false
	}
}

func compareOp(version semver, op string, bound semver) bool {
	cmp := compareSemver(version, bound)
	switch op {
	case ">=":
		return cmp >= 0
	case "<=":
		return cmp <= 0
	case ">":
		return cmp > 0
	case "<":
		return cmp < 0
	default:
		return false
	}
}

// stableRangeAllows keeps a prerelease out of a range whose comparators are
// stable releases. A prerelease matches only when one comparator names a
// prerelease of that same major.minor.patch.
func stableRangeAllows(version semver, bounds []semverBound) bool {
	if len(version.pre) == 0 {
		return true
	}
	for _, bound := range bounds {
		if len(bound.version.pre) > 0 &&
			bound.version.major == version.major &&
			bound.version.minor == version.minor &&
			bound.version.patch == version.patch {
			return true
		}
	}
	return false
}

type semverBound struct {
	op      string
	version semver
}

type semver struct {
	major, minor, patch int
	pre                 []preIdent
}

type preIdent struct {
	numeric bool
	number  int
	text    string
}

var (
	semverPattern = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-((?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\+([0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$`)
	numericIdent  = regexp.MustCompile(`^(0|[1-9]\d*)$`)
	partialBound  = regexp.MustCompile(`^(0|[1-9]\d*)(?:\.(0|[1-9]\d*))?$`)
)

func parseSemver(value string) (semver, error) {
	match := semverPattern.FindStringSubmatch(value)
	if match == nil {
		return semver{}, fmt.Errorf("invalid semantic version %q", value)
	}
	major, err := strconv.Atoi(match[1])
	if err != nil {
		return semver{}, fmt.Errorf("invalid semantic version %q", value)
	}
	minor, err := strconv.Atoi(match[2])
	if err != nil {
		return semver{}, fmt.Errorf("invalid semantic version %q", value)
	}
	patch, err := strconv.Atoi(match[3])
	if err != nil {
		return semver{}, fmt.Errorf("invalid semantic version %q", value)
	}
	var pre []preIdent
	if match[4] != "" {
		for _, part := range strings.Split(match[4], ".") {
			if numericIdent.MatchString(part) {
				number, err := strconv.Atoi(part)
				if err != nil {
					return semver{}, fmt.Errorf("invalid semantic version %q", value)
				}
				pre = append(pre, preIdent{numeric: true, number: number})
				continue
			}
			pre = append(pre, preIdent{text: part})
		}
	}
	return semver{major: major, minor: minor, patch: patch, pre: pre}, nil
}

// parseBound accepts a full semantic version, or a major / major.minor floor
// used by the embedded node range, such as "<25".
func parseBound(value string) (semver, error) {
	if parsed, err := parseSemver(value); err == nil {
		return parsed, nil
	}
	if !partialBound.MatchString(value) {
		return semver{}, fmt.Errorf("invalid version bound %q", value)
	}
	padded := value
	switch strings.Count(value, ".") {
	case 0:
		padded += ".0.0"
	case 1:
		padded += ".0"
	}
	return parseSemver(padded)
}

func compareSemver(left, right semver) int {
	if cmp := compareInt(left.major, right.major); cmp != 0 {
		return cmp
	}
	if cmp := compareInt(left.minor, right.minor); cmp != 0 {
		return cmp
	}
	if cmp := compareInt(left.patch, right.patch); cmp != 0 {
		return cmp
	}
	return comparePre(left.pre, right.pre)
}

func compareInt(left, right int) int {
	if left < right {
		return -1
	}
	if left > right {
		return 1
	}
	return 0
}

func comparePre(left, right []preIdent) int {
	if len(left) == 0 && len(right) == 0 {
		return 0
	}
	if len(left) == 0 {
		return 1
	}
	if len(right) == 0 {
		return -1
	}
	n := len(left)
	if len(right) < n {
		n = len(right)
	}
	for i := 0; i < n; i++ {
		if cmp := compareIdent(left[i], right[i]); cmp != 0 {
			return cmp
		}
	}
	return compareInt(len(left), len(right))
}

func compareIdent(left, right preIdent) int {
	switch {
	case left.numeric && right.numeric:
		return compareInt(left.number, right.number)
	case left.numeric:
		return -1
	case right.numeric:
		return 1
	}
	if left.text < right.text {
		return -1
	}
	if left.text > right.text {
		return 1
	}
	return 0
}

// newWorkspace creates this invocation's toolchain directory. MkdirTemp uses
// mode 0700 before umask. The build identifier does not choose the name.
func newWorkspace() (string, error) {
	return os.MkdirTemp("", "cfgb-build-*")
}

// discardFailedBuild removes the incomplete output and the workspace created by
// this invocation. A successful build leaves both in place.
func discardFailedBuild(out, workspace string, built *bool) error {
	if built != nil && *built {
		return nil
	}
	var err error
	for _, name := range []string{out, workspace} {
		if name == "" {
			continue
		}
		if cleanupErr := os.RemoveAll(name); cleanupErr != nil {
			err = errors.Join(err, fmt.Errorf("remove failed build %s: %w", name, cleanupErr))
		}
	}
	return err
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

func stageContent(cfg *config.File, snapshot string) (contentRoot, topicsFile, linkcardsDir string, err error) {
	root, err := os.OpenRoot(cfg.Root())
	if err != nil {
		return "", "", "", err
	}
	defer func() { err = errors.Join(err, root.Close()) }()
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

func stageMedia(cfg *config.File, dest string) (err error) {
	root, err := os.OpenRoot(cfg.Root())
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	content, err := repoRelative(cfg.Root(), filepath.Dir(cfg.Path()), cfg.Content.Root)
	if err != nil {
		return err
	}
	if err := stageArticleMedia(root, content, dest); err != nil {
		return err
	}
	for _, item := range []struct{ dir, name string }{
		{"home/assets", "home"},
		{"pages/about/assets", "about"},
		{"aside/assets", "aside"},
	} {
		if err := copyOptionalAssets(root, joinRoot(content, item.dir), filepath.Join(dest, item.name)); err != nil {
			return err
		}
	}
	return nil
}

func stageArticleMedia(root *os.Root, content, dest string) (err error) {
	posts, err := root.OpenRoot(joinRoot(content, "posts"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer func() { err = errors.Join(err, posts.Close()) }()
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
			err = copyOptionalAssets(group, "assets", filepath.Join(dest, year.Name(), key.Name()))
			err = errors.Join(err, group.Close())
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func copyOptionalAssets(root *os.Root, name, dest string) (err error) {
	info, err := root.Stat(name)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return nil
	}
	assets, err := root.OpenRoot(name)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, assets.Close()) }()
	return copyFromRoot(assets, ".", dest)
}

func readRootDir(root *os.Root, name string) (entries []fs.DirEntry, err error) {
	file, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, file.Close()) }()
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

func stageOutput(out, dist string, workerSource, manifest []byte) (err error) {
	root, err := os.OpenRoot(out)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	if err := root.Mkdir(".tmp", 0o755); err != nil && !os.IsExist(err) {
		return err
	}
	defer func() {
		if cleanupErr := root.RemoveAll(".tmp"); cleanupErr != nil {
			err = errors.Join(err, fmt.Errorf("remove output staging: %w", cleanupErr))
		}
	}()
	staging, err := root.OpenRoot(".tmp")
	if err != nil {
		return err
	}
	writeErr := writeStaged(staging, dist, workerSource, manifest)
	writeErr = errors.Join(writeErr, staging.Close())
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
	return nil
}

func writeStaged(staging *os.Root, dist string, workerSource, manifest []byte) (err error) {
	site, err := os.OpenRoot(dist)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, site.Close()) }()
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

func manifestJSON(cfg *config.File, pins releasePins, tc toolchainCheck, workspace string, routes []string, out string, index frontmatter.Index) ([]byte, error) {
	commit, branch, dirty := gitState(cfg.Root(), out)
	ciCommit := os.Getenv("WORKERS_CI_COMMIT_SHA")
	ciBranch := os.Getenv("WORKERS_CI_BRANCH")
	rawBuildID := os.Getenv("WORKERS_CI_BUILD_UUID")
	workers := os.Getenv("WORKERS_CI") != "" || ciCommit != "" || ciBranch != "" || rawBuildID != ""
	if ciCommit != "" {
		commit = ciCommit
	}
	if ciBranch != "" {
		branch = ciBranch
	}
	if branch == "HEAD" {
		branch = ""
	}
	manifest := map[string]any{
		"schemaVersion":      1,
		"cfgbVersion":        version.Version,
		"rendererVersion":    pins.RendererVersion,
		"toolchainSessionId": filepath.Base(workspace),
		"dirty":              dirty,
		"publications":       publicationSnapshot(index),
		"checks":             []string{"render", "pagefind"},
		"routes":             routes,
		"toolchain": map[string]any{
			"nodeRange":               pins.NodeRange,
			"testedNodeVersion":       testedNodeVersion,
			"packageManager":          tc.PackageManager,
			"npmVersion":              testedNpmVersion,
			"pnpmVersion":             testedPnpmVersion,
			"wranglerVersion":         pins.WranglerVersion,
			"workerCompatibilityDate": worker.CompatibilityDate,
			"rendererVersion":         pins.RendererVersion,
			"nodeVersion":             tc.Node,
			"observedNpmVersion":      tc.NpmVersion,
			"observedPnpmVersion":     tc.PnpmVersion,
		},
	}
	if commit != "" {
		manifest["sourceCommit"] = commit
	}
	if branch != "" {
		manifest["sourceBranch"] = branch
	}
	switch {
	case workers:
		manifest["provenanceProvider"] = "workers-builds"
	case commit != "" || branch != "":
		manifest["provenanceProvider"] = "git"
	}
	if rawBuildID != "" {
		manifest["buildUUID"] = rawBuildID
	}
	return json.MarshalIndent(manifest, "", "  ")
}

type publicationRecord struct {
	ArticleKey  string     `json:"articleKey"`
	Locale      string     `json:"locale"`
	Slug        string     `json:"slug"`
	PublishedAt time.Time  `json:"publishedAt"`
	Summary     string     `json:"summary,omitempty"`
	UpdatedAt   *time.Time `json:"updatedAt,omitempty"`
}

func publicationSnapshot(index frontmatter.Index) []publicationRecord {
	pubs := make([]publicationRecord, 0, len(index.Posts))
	for _, post := range index.Posts {
		pubs = append(pubs, publicationRecord{
			ArticleKey:  post.ArticleKey,
			Locale:      post.Locale,
			Slug:        post.Data.Slug,
			PublishedAt: post.Data.PublishedAt,
			Summary:     post.Data.Summary,
			UpdatedAt:   post.Data.UpdatedAt,
		})
	}
	sort.Slice(pubs, func(i, j int) bool {
		if pubs[i].ArticleKey != pubs[j].ArticleKey {
			return pubs[i].ArticleKey < pubs[j].ArticleKey
		}
		return pubs[i].Locale < pubs[j].Locale
	})
	return pubs
}

func gitState(repo, out string) (string, string, bool) {
	commit, err := output("git", "-C", repo, "rev-parse", "HEAD")
	if err != nil {
		return "", "", true
	}
	branch, err := output("git", "-C", repo, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		// Branch metadata is optional diagnostics. A failed lookup means it is
		// unavailable, not a reason to prevent rendering a static site.
		branch = ""
	}
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
	return copyRootPath(root, name, dest, nil)
}

func copyRootPath(root *os.Root, name, dest string, ancestors []fs.FileInfo) (err error) {
	file, err := root.Open(name)
	if err != nil {
		return err
	}
	info, err := file.Stat()
	if err != nil {
		return errors.Join(err, file.Close())
	}
	if !info.IsDir() {
		defer func() { err = errors.Join(err, file.Close()) }()
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		out, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		return copyAndClose(out, file)
	}
	ancestors, err = directoryAncestors(name, info, ancestors)
	if err != nil {
		return errors.Join(err, file.Close())
	}
	entries, err := file.ReadDir(-1)
	err = errors.Join(err, file.Close())
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	for _, entry := range entries {
		child := joinRoot(name, entry.Name())
		if err := copyRootPath(root, child, filepath.Join(dest, entry.Name()), ancestors); err != nil {
			return err
		}
	}
	return nil
}

func copyRootToRoot(src *os.Root, name string, dest *os.Root, to string) error {
	return copyRootToRootPath(src, name, dest, to, nil)
}

func copyRootToRootPath(src *os.Root, name string, dest *os.Root, to string, ancestors []fs.FileInfo) (err error) {
	file, err := src.Open(name)
	if err != nil {
		return err
	}
	info, err := file.Stat()
	if err != nil {
		return errors.Join(err, file.Close())
	}
	if !info.IsDir() {
		defer func() { err = errors.Join(err, file.Close()) }()
		data, err := io.ReadAll(file)
		if err != nil {
			return err
		}
		return dest.WriteFile(to, data, 0o644)
	}
	ancestors, err = directoryAncestors(name, info, ancestors)
	if err != nil {
		return errors.Join(err, file.Close())
	}
	entries, err := file.ReadDir(-1)
	err = errors.Join(err, file.Close())
	if err != nil {
		return err
	}
	if err := dest.MkdirAll(to, 0o755); err != nil {
		return err
	}
	for _, entry := range entries {
		if err := copyRootToRootPath(src, joinRoot(name, entry.Name()), dest, joinRoot(to, entry.Name()), ancestors); err != nil {
			return err
		}
	}
	return nil
}

// Only a directory already on this traversal's ancestor chain is a cycle.
// Other branches may legitimately reach the same directory through symlinks.
func directoryAncestors(name string, info fs.FileInfo, ancestors []fs.FileInfo) ([]fs.FileInfo, error) {
	for _, ancestor := range ancestors {
		if os.SameFile(info, ancestor) {
			return nil, fmt.Errorf("directory cycle: %s", name)
		}
	}
	return append(ancestors, info), nil
}

func joinRoot(parent, name string) string {
	parent = strings.TrimSuffix(filepath.ToSlash(parent), "/")
	name = filepath.ToSlash(name)
	if parent == "" || parent == "." {
		return name
	}
	return parent + "/" + name
}

// contentError distinguishes invalid content from failures to read that content.
func contentError(err error) *ExitError {
	code := 3
	var validation *frontmatter.ValidationError
	if errors.As(err, &validation) {
		code = 1
	}
	return &ExitError{Code: code, Err: err}
}
