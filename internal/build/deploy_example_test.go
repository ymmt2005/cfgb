package build

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"io"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// Exercise the real pinned Wrangler on the artifact produced by the corpus.
// No upload credentials are needed and no remote deployment is performed.
func checkExampleDeployDryRun(t *testing.T, artifact string) {
	t.Helper()
	_, repo := testRepo(t)
	if _, err := output("git", "-C", repo, "checkout", "-b", "main"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CFGB_CF_WORKER_NAME", "cfgb-deploy-acceptance")
	t.Setenv("CLOUDFLARE_ACCOUNT_ID", "00000000000000000000000000000000")
	t.Setenv("CLOUDFLARE_API_TOKEN", "")
	page := filepath.Join(artifact, "site", "en", "index.html")
	data, err := os.ReadFile(page)
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, []byte("\n<!-- operator edit before upload -->\n")...)
	if err := os.WriteFile(page, data, 0o644); err != nil {
		t.Fatal(err)
	}
	before := deployArtifactBytes(t, artifact)
	var stdout, stderr bytes.Buffer
	var configPath string
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate deployment runtime helper")
	}
	helper := filepath.Join(filepath.Dir(thisFile), "..", "..", "renderer", "tests", "helpers", "deploy-runtime.mjs")
	run := func(dir string, out, errOut io.Writer, extra []string, name string, args ...string) error {
		if slices.Contains(args, "deploy") {
			configPath = args[3]
			raw, err := os.ReadFile(configPath)
			if err != nil {
				return err
			}
			var conf struct {
				Assets struct {
					Directory string `json:"directory"`
				} `json:"assets"`
			}
			if err := json.Unmarshal(raw, &conf); err != nil {
				return err
			}
			manifest, err := readDeployManifest(artifact)
			if err != nil {
				return err
			}
			prefix, err := siteBasePath(manifest.BaseURL)
			if err != nil {
				return err
			}
			staged, err := os.ReadFile(filepath.Join(conf.Assets.Directory, strings.TrimPrefix(prefix, "/"), "en", "index.html"))
			if err != nil {
				return err
			}
			if !bytes.Equal(staged, data) {
				t.Fatal("Wrangler did not receive the operator-edited page")
			}
			if err := command(dir, out, errOut, extra, name, args...); err != nil {
				return err
			}
			return command(dir, out, errOut, extra, "node", helper, args[0], configPath, prefix)
		}
		return command(dir, out, errOut, extra, name, args...)
	}
	if err := deploy(DeployOptions{Dir: repo, From: artifact, DryRun: true, Stdout: &stdout, Stderr: &stderr}, run); err != nil {
		t.Fatalf("real Wrangler dry run: %v\n%s\n%s", err, &stdout, &stderr)
	}
	if !strings.Contains(stdout.String(), "dry-run") {
		t.Fatalf("Wrangler did not complete a dry run: %s", &stdout)
	}
	if !maps.Equal(before, deployArtifactBytes(t, artifact)) {
		t.Fatal("Wrangler dry run changed the supplied artifact")
	}
	if _, err := os.Stat(filepath.Dir(configPath)); !os.IsNotExist(err) {
		t.Fatalf("upload workspace remained: %v", err)
	}
}

func deployArtifactBytes(t *testing.T, artifact string) map[string][sha256.Size]byte {
	t.Helper()
	files := map[string][sha256.Size]byte{}
	if err := filepath.WalkDir(artifact, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[path] = sha256.Sum256(raw)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return files
}
