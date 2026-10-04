package build

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
	t.Cleanup(func() {
		if err := os.Chmod(parent, 0o755); err != nil {
			t.Error(err)
		}
	})
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

func TestFreshWorkspacesStayDistinct(t *testing.T) {
	t.Setenv("WORKERS_CI_BUILD_UUID", "build/../same-id")
	first, err := newWorkspace()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(first); err != nil {
			t.Error(err)
		}
	})
	second, err := newWorkspace()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(second); err != nil {
			t.Error(err)
		}
	})
	if first == second {
		t.Fatal("repeated builds shared a workspace")
	}
	for _, dir := range []string{first, second} {
		info, err := os.Stat(dir)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o700 {
			t.Fatalf("mode = %o", info.Mode().Perm())
		}
		rel, err := filepath.Rel(os.TempDir(), dir)
		if err != nil || !filepath.IsLocal(rel) || strings.Contains(rel, string(filepath.Separator)) {
			t.Fatalf("workspace = %s", dir)
		}
		if !strings.HasPrefix(rel, "cfgb-build-") || strings.Contains(rel, "same-id") {
			t.Fatalf("basename = %s", rel)
		}
	}
}

func TestFailedBuildRemovesItsWorkspace(t *testing.T) {
	failed, err := newWorkspace()
	if err != nil {
		t.Fatal(err)
	}
	retained, err := newWorkspace()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(retained); err != nil {
			t.Error(err)
		}
	})
	out := filepath.Join(t.TempDir(), "dist")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	built := false
	if err := discardFailedBuild(out, failed, &built); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(failed); !os.IsNotExist(err) {
		t.Fatal("failed build retained its workspace")
	}
	if _, err := os.Stat(retained); err != nil {
		t.Fatal("failed build removed another workspace")
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatal("failed build retained the incomplete output")
	}

	kept, err := newWorkspace()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(kept); err != nil {
			t.Error(err)
		}
	})
	artifact := filepath.Join(t.TempDir(), "dist")
	if err := os.MkdirAll(artifact, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(artifact); err != nil {
			t.Error(err)
		}
	})
	built = true
	if err := discardFailedBuild(artifact, kept, &built); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(kept); err != nil {
		t.Fatal("successful build removed its workspace")
	}
	if _, err := os.Stat(artifact); err != nil {
		t.Fatal("successful build removed its output")
	}
}

func TestManifestSessionIDIsBasename(t *testing.T) {
	cfg, repo := testRepo(t)
	t.Setenv("WORKERS_CI_BUILD_UUID", "opaque/id")
	workspace, err := newWorkspace()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(workspace); err != nil {
			t.Error(err)
		}
	})
	raw, err := manifestJSON(cfg, releasePins{}, toolchainCheck{}, workspace, []string{"/ja/"}, filepath.Join(repo, "dist"), frontmatter.Index{})
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		ToolchainSessionID string `json:"toolchainSessionId"`
		BuildUUID          string `json:"buildUUID"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.ToolchainSessionID != filepath.Base(workspace) {
		t.Fatalf("session = %s", manifest.ToolchainSessionID)
	}
	if strings.Contains(string(raw), workspace) || strings.Contains(manifest.ToolchainSessionID, "/") {
		t.Fatalf("manifest recorded a workspace path: %s", raw)
	}
	if manifest.BuildUUID != "opaque/id" {
		t.Fatalf("buildUUID = %s", manifest.BuildUUID)
	}
}

func TestManifestRecordsOptionalDiagnostics(t *testing.T) {
	cfg, repo := testRepo(t)
	workspace, err := newWorkspace()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(workspace); err != nil {
			t.Error(err)
		}
	})
	updated := time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC)
	index := frontmatter.Index{Posts: []frontmatter.Post{
		{
			ArticleKey: "2026-09-20-markdown-showcase",
			Locale:     "ja",
			Data: frontmatter.Metadata{
				Slug:        "markdown-showcase",
				PublishedAt: time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC),
				Summary:     "構文",
				UpdatedAt:   &updated,
			},
		},
		{
			ArticleKey: "2026-09-19-protobuf-guide",
			Locale:     "en",
			Data: frontmatter.Metadata{
				Slug:        "reading-protobuf-schemas",
				PublishedAt: time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC),
			},
		},
	}}
	clearCI := func() {
		t.Setenv("WORKERS_CI", "")
		t.Setenv("WORKERS_CI_COMMIT_SHA", "")
		t.Setenv("WORKERS_CI_BRANCH", "")
		t.Setenv("WORKERS_CI_BUILD_UUID", "")
	}
	decode := func() map[string]any {
		t.Helper()
		raw, err := manifestJSON(cfg, releasePins{}, toolchainCheck{}, workspace, nil, filepath.Join(repo, "dist"), index)
		if err != nil {
			t.Fatal(err)
		}
		var manifest map[string]any
		if err := json.Unmarshal(raw, &manifest); err != nil {
			t.Fatal(err)
		}
		return manifest
	}

	clearCI()
	plain := decode()
	if _, ok := plain["sourceCommit"]; ok {
		t.Fatalf("sourceCommit = %v", plain["sourceCommit"])
	}
	if plain["dirty"] != true {
		t.Fatalf("dirty = %v", plain["dirty"])
	}
	pubs, _ := plain["publications"].([]any)
	if len(pubs) != 2 {
		t.Fatalf("publications = %#v", plain["publications"])
	}
	first, _ := pubs[0].(map[string]any)
	if first["articleKey"] != "2026-09-19-protobuf-guide" || first["publishedAt"] != "2026-09-19T00:00:00Z" || first["slug"] != "reading-protobuf-schemas" {
		t.Fatalf("first publication = %#v", first)
	}
	second, _ := pubs[1].(map[string]any)
	if second["summary"] != "構文" || second["updatedAt"] != "2026-09-21T00:00:00Z" {
		t.Fatalf("second publication = %#v", second)
	}

	t.Setenv("WORKERS_CI", "1")
	t.Setenv("WORKERS_CI_COMMIT_SHA", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	partial := decode()
	if partial["sourceCommit"] != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" || partial["provenanceProvider"] != "workers-builds" {
		t.Fatalf("partial = %#v", partial)
	}
	if _, ok := partial["sourceBranch"]; ok {
		t.Fatalf("sourceBranch = %v", partial["sourceBranch"])
	}
	if _, ok := partial["buildUUID"]; ok {
		t.Fatalf("buildUUID = %v", partial["buildUUID"])
	}

	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=cfgb", "GIT_AUTHOR_EMAIL=cfgb@example.com", "GIT_COMMITTER_NAME=cfgb", "GIT_COMMITTER_EMAIL=cfgb@example.com")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", args, err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(repo, "article.md"), []byte("source\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "article.md")
	git("commit", "-m", "source")
	t.Setenv("WORKERS_CI_BRANCH", "post/test")
	t.Setenv("WORKERS_CI_BUILD_UUID", "../build/opaque\\segment")
	mismatched := decode()
	if mismatched["sourceCommit"] != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" || mismatched["sourceBranch"] != "post/test" || mismatched["buildUUID"] != "../build/opaque\\segment" {
		t.Fatalf("mismatched = %#v", mismatched)
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

func TestLiteralOutputNameStaysExcluded(t *testing.T) {
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
	out := filepath.Join(repo, "*")
	if err := os.MkdirAll(filepath.Join(out, "site"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "site", "index.html"), []byte("page"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(article, []byte("edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, dirty := gitState(repo, out); !dirty {
		t.Fatal("output named * hid the edited article")
	}
}

func TestGitStatusFailureIsDirty(t *testing.T) {
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
	if err := os.WriteFile(filepath.Join(repo, "article.md"), []byte("source\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "article.md")
	git("commit", "-m", "source")
	if err := os.Truncate(filepath.Join(repo, ".git", "index"), 8); err != nil {
		t.Fatal(err)
	}
	commit, branch, dirty := gitState(repo, filepath.Join(repo, "site-out"))
	if commit == "" || branch == "" || !dirty {
		t.Fatalf("commit=%s branch=%s dirty=%v", commit, branch, dirty)
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

func TestProseAssetsArePublished(t *testing.T) {
	cfg, repo := testRepo(t)
	content := filepath.Join(repo, "src", "content")
	files := map[string]string{
		filepath.Join(content, "home", "assets", "portrait.svg"):           "home",
		filepath.Join(content, "pages", "about", "assets", "portrait.svg"): "about",
		filepath.Join(content, "aside", "assets", "note.svg"):              "aside",
	}
	for name, body := range files {
		if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	media := t.TempDir()
	if err := stageMedia(cfg, media); err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct{ path, body string }{
		{"home/portrait.svg", "home"},
		{"about/portrait.svg", "about"},
		{"aside/note.svg", "aside"},
	} {
		raw, err := os.ReadFile(filepath.Join(media, item.path))
		if err != nil || string(raw) != item.body {
			t.Fatalf("%s = %q, %v", item.path, raw, err)
		}
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

func TestSelectedConfigurationOutputBase(t *testing.T) {
	_, repo := testRepo(t)
	dir := filepath.Join(repo, "settings")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "blog.yaml")
	if err := os.WriteFile(file, []byte(testConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"", "custom-output"} {
		got, err := outputDir(cfg, name)
		if name == "" {
			name = "dist"
		}
		want := filepath.Join(dir, name)
		if err != nil || got != want {
			t.Fatalf("output = %q, %v, want %q", got, err, want)
		}
	}
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
