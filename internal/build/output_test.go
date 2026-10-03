package build

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/ymmt2005/cfgb/internal/config"
	"github.com/ymmt2005/cfgb/internal/frontmatter"
)

func TestOutputDirRejectsInputs(t *testing.T) {
	cfg, repo := testRepo(t)
	for _, rel := range []string{
		".",
		"src",
		filepath.Join("src", "content"),
		filepath.Join("src", "content", "posts"),
		filepath.Join("src", "data"),
		".git",
		filepath.Join(".git", "objects"),
	} {
		if _, err := outputDir(cfg, filepath.Join(repo, rel)); err == nil {
			t.Errorf("accepted %s", rel)
		}
	}
	out := filepath.Join(repo, "dist")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "notes.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := outputDir(cfg, out)
	if err != nil || got != out {
		t.Fatalf("dist = %s, %v", got, err)
	}
	if err := os.Symlink("src", filepath.Join(repo, "linked-src")); err != nil {
		t.Fatal(err)
	}
	if _, err := outputDir(cfg, filepath.Join(repo, "linked-src")); err != nil {
		t.Fatal(err)
	}
}

func TestDeliverPromotesCompleteArtifact(t *testing.T) {
	parent := t.TempDir()
	out := filepath.Join(parent, "dist")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "old.txt"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	sibling := out + ".tmp"
	if err := os.WriteFile(sibling, []byte("sibling"), 0o644); err != nil {
		t.Fatal(err)
	}
	dist := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dist, "ja"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dist, "ja", "index.html"), []byte("ja"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := resetOutput(out); err != nil {
		t.Fatal(err)
	}
	if err := deliver(out, dist, []byte("worker\n"), []byte("{}\n")); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"site/ja/index.html", "worker/index.js", "build-manifest.json"} {
		if _, err := os.Stat(filepath.Join(out, rel)); err != nil {
			t.Errorf("missing %s", rel)
		}
	}
	if _, err := os.Stat(filepath.Join(out, ".tmp")); !os.IsNotExist(err) {
		t.Fatal(".tmp remains in the artifact")
	}
	if _, err := os.Stat(filepath.Join(out, "old.txt")); !os.IsNotExist(err) {
		t.Fatal("previous file remains")
	}
	raw, err := os.ReadFile(sibling)
	if err != nil || string(raw) != "sibling" {
		t.Fatalf("sibling = %q, %v", raw, err)
	}
}

func TestDeliverStopsWhenRemovalFails(t *testing.T) {
	parent := t.TempDir()
	out := filepath.Join(parent, "dist")
	if err := os.WriteFile(out, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	sibling := out + ".tmp"
	if err := os.WriteFile(sibling, []byte("sibling"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(parent, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(parent, 0o755) })
	if err := resetOutput(out); err == nil {
		t.Fatal("expected removal to fail")
	}
	raw, err := os.ReadFile(out)
	if err != nil || string(raw) != "old" {
		t.Fatalf("previous output = %q, %v", raw, err)
	}
	raw, err = os.ReadFile(sibling)
	if err != nil || string(raw) != "sibling" {
		t.Fatalf("sibling = %q, %v", raw, err)
	}
	if info, err := os.Stat(out); err != nil || info.IsDir() {
		t.Fatal("failed removal replaced the output")
	}
}

func TestSessionDirIsPrivate(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	t.Setenv("WORKERS_CI_BUILD_UUID", "existing-session")
	sum := sha256.Sum256([]byte("existing-session"))
	dir := filepath.Join(cache, "cfgb", "builds", hex.EncodeToString(sum[:]))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := sessionDir()
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(got)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Fatalf("mode = %o", info.Mode().Perm())
	}
	outside := t.TempDir()
	if err := os.Remove(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, dir); err != nil {
		t.Fatal(err)
	}
	if _, err := sessionDir(); err == nil {
		t.Fatal("session symlink was accepted")
	}
	if _, err := os.Stat(filepath.Join(outside, "renderer")); !os.IsNotExist(err) {
		t.Fatal("session symlink was followed")
	}
}

func TestCacheAncestorSymlinkStaysInCache(t *testing.T) {
	cache := t.TempDir()
	outside := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	t.Setenv("WORKERS_CI_BUILD_UUID", "existing-session")
	sum := sha256.Sum256([]byte("existing-session"))
	id := hex.EncodeToString(sum[:])
	escaped := filepath.Join(outside, "builds", id)
	if err := os.MkdirAll(filepath.Join(escaped, "renderer"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(escaped, "renderer", "keep.txt"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(cache, "cfgb")); err != nil {
		t.Fatal(err)
	}
	if _, err := sessionDir(); err == nil {
		t.Fatal("cache symlink escaped")
	}
	if err := prepareSession(escaped); err == nil {
		t.Fatal("removal left the cache")
	}
	raw, err := os.ReadFile(filepath.Join(escaped, "renderer", "keep.txt"))
	if err != nil || string(raw) != "keep" {
		t.Fatalf("escaped session = %q, %v", raw, err)
	}
}

func TestInRootCacheSymlinkIsUsable(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	t.Setenv("WORKERS_CI_BUILD_UUID", "inside-session")
	if err := os.MkdirAll(filepath.Join(cache, "inside"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cache, "inside", "keep.txt"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("inside", filepath.Join(cache, "cfgb")); err != nil {
		t.Fatal(err)
	}
	dir, err := sessionDir()
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Fatalf("mode = %o", info.Mode().Perm())
	}
	if err := os.MkdirAll(filepath.Join(dir, "renderer"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "renderer", "old.txt"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := prepareSession(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "renderer", "old.txt")); !os.IsNotExist(err) {
		t.Fatal("session refresh left the previous renderer")
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(cache, "inside", "keep.txt"))
	if err != nil || string(raw) != "keep" {
		t.Fatalf("cache sibling = %q, %v", raw, err)
	}
}

func TestGeneratedOutputIsNotSourceDirty(t *testing.T) {
	repo := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=cfgb", "GIT_AUTHOR_EMAIL=cfgb@example.com", "GIT_COMMITTER_NAME=cfgb", "GIT_COMMITTER_EMAIL=cfgb@example.com")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", args, err, out)
		}
	}
	git("init")
	article := filepath.Join(repo, "article.md")
	if err := os.WriteFile(article, []byte("source\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "article.md")
	git("commit", "-m", "source")
	out := filepath.Join(repo, "site-out")
	writeOutput := func() {
		t.Helper()
		if err := os.MkdirAll(filepath.Join(out, "site"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(out, "site", "index.html"), []byte("page"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(out, "build-manifest.json"), []byte("{}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeOutput()
	if _, _, dirty := gitState(repo, out); dirty {
		t.Fatal("unignored custom output counted as a source edit")
	}
	writeOutput()
	if _, _, dirty := gitState(repo, out); dirty {
		t.Fatal("existing custom output counted as a source edit")
	}
	if err := os.WriteFile(article, []byte("edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, dirty := gitState(repo, out); !dirty {
		t.Fatal("source edit was recorded as clean")
	}
}

func TestRootedInputsRejectEscapes(t *testing.T) {
	cfg, repo := testRepo(t)
	content := filepath.Join(repo, "src", "content")
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	rel, err := filepath.Rel(content, filepath.Join(outside, "secret.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(rel, filepath.Join(content, "secret.txt")); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := stageContent(cfg, t.TempDir()); err == nil {
		t.Fatal("content symlink escaped the repository")
	}

	cfg, repo = testRepo(t)
	content = filepath.Join(repo, "src", "content")
	if err := os.WriteFile(filepath.Join(content, "real.txt"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("real.txt", filepath.Join(content, "alias.txt")); err != nil {
		t.Fatal(err)
	}
	dest, _, _, err := stageContent(cfg, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dest, "alias.txt"))
	if err != nil || string(raw) != "ok" {
		t.Fatalf("alias = %q, %v", raw, err)
	}

	group := filepath.Join(content, "posts", "2026", "article", "assets")
	if err := os.MkdirAll(group, 0o755); err != nil {
		t.Fatal(err)
	}
	assetRel, err := filepath.Rel(group, filepath.Join(outside, "secret.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(assetRel, filepath.Join(group, "pic.png")); err != nil {
		t.Fatal(err)
	}
	if err := stageMedia(cfg, t.TempDir()); err == nil {
		t.Fatal("article asset symlink escaped the group")
	}

	if err := os.Remove(filepath.Join(group, "pic.png")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(group, "a.png"), []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("a.png", filepath.Join(group, "b.png")); err != nil {
		t.Fatal(err)
	}
	media := t.TempDir()
	if err := stageMedia(cfg, media); err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile(filepath.Join(media, "2026", "article", "b.png"))
	if err != nil || string(raw) != "png" {
		t.Fatalf("asset alias = %q, %v", raw, err)
	}

	cfg.Content.Root = "../outside"
	if _, _, _, err := stageContent(cfg, t.TempDir()); err == nil {
		t.Fatal(".. escaped the repository")
	}
}

func TestOutputSymlinkIsReplaced(t *testing.T) {
	cfg, repo := testRepo(t)
	content := filepath.Join(repo, "src", "content")
	if err := os.WriteFile(filepath.Join(content, "keep.txt"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(repo, "dist")
	if err := os.Symlink(content, link); err != nil {
		t.Fatal(err)
	}
	sibling := link + ".tmp"
	if err := os.WriteFile(sibling, []byte("sibling"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := outputDir(cfg, link)
	if err != nil {
		t.Fatal(err)
	}
	if err := resetOutput(got); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(link)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		t.Fatalf("output mode = %s", info.Mode())
	}
	raw, err := os.ReadFile(filepath.Join(content, "keep.txt"))
	if err != nil || string(raw) != "keep" {
		t.Fatalf("target = %q, %v", raw, err)
	}
	if _, err := os.Stat(filepath.Join(link, ".tmp")); err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile(sibling)
	if err != nil || string(raw) != "sibling" {
		t.Fatalf("sibling = %q, %v", raw, err)
	}

	dangling := filepath.Join(repo, "dangling")
	if err := os.Symlink(filepath.Join(repo, "missing-target"), dangling); err != nil {
		t.Fatal(err)
	}
	got, err = outputDir(cfg, dangling)
	if err != nil {
		t.Fatal(err)
	}
	if err := resetOutput(got); err != nil {
		t.Fatal(err)
	}
	info, err = os.Lstat(dangling)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		t.Fatalf("dangling mode = %s", info.Mode())
	}
}

func TestMissingLinkCardCache(t *testing.T) {
	cfg, repo := testRepo(t)
	cards := filepath.Join(repo, "src", "data", "linkcards")
	if err := os.RemoveAll(cards); err != nil {
		t.Fatal(err)
	}
	_, _, linkcardsDir, err := stageContent(cfg, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(linkcardsDir); !os.IsNotExist(err) {
		t.Fatal("absent cache was created")
	}
}

func TestReusedSessionDropsDeletedInputs(t *testing.T) {
	cfg, repo := testRepo(t)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("WORKERS_CI_BUILD_UUID", "reuse-session")
	article := filepath.Join(repo, "src", "content", "posts", "2026", "guide")
	assets := filepath.Join(article, "assets")
	if err := os.MkdirAll(assets, 0o755); err != nil {
		t.Fatal(err)
	}
	post := "---\ntitle: T\nslug: t\npublishedAt: '2026-01-02T03:04:05Z'\ntopics:\n- protobuf\n---\nbody\n"
	if err := os.WriteFile(filepath.Join(article, "ja.md"), []byte(post), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assets, "hero.svg"), []byte("<svg/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "src", "data", "linkcards", "card.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	session, err := sessionDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := prepareSession(session); err != nil {
		t.Fatal(err)
	}
	contentRoot, _, cardsDir, err := stageContent(cfg, filepath.Join(session, "snapshot"))
	if err != nil {
		t.Fatal(err)
	}
	media := filepath.Join(session, "renderer", "public", "media")
	if err := stageMedia(cfg, media); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(contentRoot, "posts", "2026", "guide", "ja.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(media, "2026", "guide", "hero.svg")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(cardsDir, "card.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(repo, "src", "content", "posts", "2026")); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(repo, "src", "data", "linkcards")); err != nil {
		t.Fatal(err)
	}
	if err := prepareSession(session); err != nil {
		t.Fatal(err)
	}
	contentRoot, topicsFile, cardsDir, err := stageContent(cfg, filepath.Join(session, "snapshot"))
	if err != nil {
		t.Fatal(err)
	}
	if err := stageMedia(cfg, media); err != nil {
		t.Fatal(err)
	}
	index, err := frontmatter.Collect(contentRoot, topicsFile, []string{"ja"})
	if err != nil {
		t.Fatal(err)
	}
	if len(index.Posts) != 0 {
		t.Fatalf("deleted article remained in metadata: %#v", index.Posts)
	}
	if _, err := os.Stat(filepath.Join(contentRoot, "posts", "2026", "guide", "ja.md")); !os.IsNotExist(err) {
		t.Fatal("deleted article survived the reused session")
	}
	if _, err := os.Stat(filepath.Join(media, "2026", "guide", "hero.svg")); !os.IsNotExist(err) {
		t.Fatal("deleted media survived the reused session")
	}
	if _, err := os.Stat(filepath.Join(cardsDir, "card.json")); !os.IsNotExist(err) {
		t.Fatal("removed link card survived the reused session")
	}
	if _, err := os.Stat(session); err != nil {
		t.Fatal(err)
	}
}

func TestDirectorySymlinksKeepMedia(t *testing.T) {
	cfg, repo := testRepo(t)
	posts := filepath.Join(repo, "src", "content", "posts")
	yearData := filepath.Join(posts, "year-data", "guide", "assets")
	if err := os.MkdirAll(yearData, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(yearData, "hero.svg"), []byte("year"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("year-data", filepath.Join(posts, "2026")); err != nil {
		t.Fatal(err)
	}
	media := t.TempDir()
	if err := stageMedia(cfg, media); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(media, "2026", "guide", "hero.svg"))
	if err != nil || string(raw) != "year" {
		t.Fatalf("linked year = %q, %v", raw, err)
	}

	cfg, repo = testRepo(t)
	posts = filepath.Join(repo, "src", "content", "posts")
	article := filepath.Join(posts, "2026", "real-article", "assets")
	if err := os.MkdirAll(article, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(article, "hero.svg"), []byte("article"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("real-article", filepath.Join(posts, "2026", "guide")); err != nil {
		t.Fatal(err)
	}
	media = t.TempDir()
	if err := stageMedia(cfg, media); err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile(filepath.Join(media, "2026", "guide", "hero.svg"))
	if err != nil || string(raw) != "article" {
		t.Fatalf("linked article = %q, %v", raw, err)
	}
}

func testRepo(t *testing.T) (*config.File, string) {
	t.Helper()
	repo := t.TempDir()
	cmd := exec.Command("git", "init")
	cmd.Dir = repo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	if err := os.MkdirAll(filepath.Join(repo, "src", "content", "posts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repo, "src", "data", "linkcards"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "src", "data", "topics.yaml"), []byte("protobuf:\n  ja: PB\n  en: PB\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "cfgb.yaml"), []byte(testConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(repo)
	if err != nil {
		t.Fatal(err)
	}
	return cfg, repo
}

const testConfig = `
schemaVersion: 1
site:
  title: CFGB Example
  baseUrl: https://example.invalid
  defaultLocale: ja
  timezone: Asia/Tokyo
locales:
  ja:
    label: 日本語
`
