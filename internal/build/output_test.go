package build

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/ymmt2005/cfgb/internal/config"
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
	if _, err := outputDir(cfg, filepath.Join(repo, "linked-src")); err == nil {
		t.Fatal("symlink onto source was accepted")
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
	if err := deliver(out, t.TempDir(), []byte("worker"), []byte("{}")); err == nil {
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
