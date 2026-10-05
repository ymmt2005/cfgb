package build

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

func TestDependencyCompatibility(t *testing.T) {
	tc := toolchainCheck{Node: "24.21.0", PackageManager: "npm", NpmVersion: "12.2.0"}
	configuration := map[string]json.RawMessage{"install-strategy": json.RawMessage(`"hoisted"`)}
	inputs := map[string][]byte{"package.json": []byte("package"), "package-lock.json": []byte("lock")}
	key, err := hashDependencyInputs(tc, "linux-x64-glibc", "", configuration, inputs)
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range []string{"node", "manager", "manager version", "platform", "node env", "configuration", "package", "lockfile", "pnpm script policy"} {
		t.Run(change, func(t *testing.T) {
			other := tc
			platform, nodeEnv := "linux-x64-glibc", ""
			config := map[string]json.RawMessage{"install-strategy": json.RawMessage(`"hoisted"`)}
			files := map[string][]byte{"package.json": []byte("package"), "package-lock.json": []byte("lock")}
			switch change {
			case "node":
				other.Node = "26.10.0"
			case "manager":
				other.PackageManager = "pnpm"
				other.PnpmVersion = "12.8.1"
			case "manager version":
				other.NpmVersion = "12.3.0"
			case "platform":
				platform = "linux-arm64-musl"
			case "node env":
				nodeEnv = "production"
			case "configuration":
				config["ignore-scripts"] = json.RawMessage(`true`)
			case "package":
				files["package.json"] = []byte("changed script policy")
			case "lockfile":
				files["package-lock.json"] = []byte("changed dependency")
			case "pnpm script policy":
				files["pnpm-workspace.yaml"] = []byte("changed build policy")
			}
			got, err := hashDependencyInputs(other, platform, nodeEnv, config, files)
			if err != nil {
				t.Fatal(err)
			}
			if got == key {
				t.Fatalf("%s reused incompatible dependencies", change)
			}
		})
	}
}

func TestDependencyConfigurationOrder(t *testing.T) {
	tc := toolchainCheck{Node: "24.21.0", PackageManager: "pnpm", PnpmVersion: "12.8.1"}
	var expected string
	for _, raw := range []string{
		`{"default":"https://registry.npmjs.org/","@jsr":"https://npm.jsr.io/"}`,
		`{"@jsr":"https://npm.jsr.io/","default":"https://registry.npmjs.org/"}`,
	} {
		key, err := hashDependencyInputs(tc, "linux-x64", "", map[string]json.RawMessage{"registries": json.RawMessage(raw)}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if expected != "" && key != expected {
			t.Fatal("nested installer configuration ordering changed the dependency cache key")
		}
		expected = key
	}
}

// The lightweight installer exercises actual subprocess/filesystem boundaries.
// Native npm/pnpm installations and renderer execution are covered by corpus tests.
func dependencyTestTools(t *testing.T) (toolchainCheck, string, string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("installer fixtures use POSIX shell scripts")
	}
	tools, cache, log := t.TempDir(), filepath.Join(t.TempDir(), "cache"), filepath.Join(t.TempDir(), "installs")
	t.Setenv("CFGB_CACHE_DIR", cache)
	t.Setenv("CFGB_DEPENDENCY_CACHE", "")
	t.Setenv("CFGB_TEST_INSTALL_LOG", log)
	t.Setenv("CFGB_TEST_INSTALL_FAIL", "")
	t.Setenv("CFGB_TEST_CONFIGURATION", `{"install-strategy":"hoisted"}`)
	for name, body := range map[string]string{
		"node": "#!/bin/sh\nprintf '%s\\n' '{\"platform\":\"linux\",\"arch\":\"x64\"}'\n",
		"npm": `#!/bin/sh
set -eu
if [ "$1" = config ]; then
  printf '%s\n' "$CFGB_TEST_CONFIGURATION"
  exit 0
fi
[ "$1" = ci ] || exit 99
printf 'install\n' >> "$CFGB_TEST_INSTALL_LOG"
mkdir -p node_modules/.bin
if [ -n "$CFGB_TEST_INSTALL_FAIL" ]; then exit 23; fi
for name in astro pagefind wrangler; do printf 'fixture\n' > "node_modules/.bin/$name"; done
`,
	} {
		if err := os.WriteFile(filepath.Join(tools, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", tools+string(os.PathListSeparator)+os.Getenv("PATH"))
	return toolchainCheck{Node: "24.21.0", PackageManager: "npm", NpmVersion: "12.2.0"}, cache, log
}

func TestDependencyCacheColdWarmAndRemoval(t *testing.T) {
	tc, cache, log := dependencyTestTools(t)
	first, second := t.TempDir(), t.TempDir()
	var messages bytes.Buffer
	for _, dir := range []string{first, second} {
		if err := installRenderer(dir, &messages, io.Discard, tc); err != nil {
			t.Fatal(err)
		}
	}
	firstTarget, err := os.Readlink(filepath.Join(first, "node_modules"))
	if err != nil {
		t.Fatal(err)
	}
	secondTarget, err := os.Readlink(filepath.Join(second, "node_modules"))
	if err != nil || firstTarget != secondTarget {
		t.Fatalf("warm build did not share the completed installation: %s, %v", secondTarget, err)
	}
	installs, err := os.ReadFile(log)
	if err != nil || string(installs) != "install\n" {
		t.Fatalf("warm build installed again: %q, %v", installs, err)
	}
	if err := os.RemoveAll(first); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(secondTarget, ".bin", "astro")); err != nil {
		t.Fatalf("workspace cleanup removed shared dependencies: %v", err)
	}
	if err := os.RemoveAll(cache); err != nil {
		t.Fatal(err)
	}
	if err := installRenderer(t.TempDir(), io.Discard, io.Discard, tc); err != nil {
		t.Fatal(err)
	}
	installs, err = os.ReadFile(log)
	if err != nil || string(installs) != "install\ninstall\n" {
		t.Fatalf("removed cache was not rebuilt: %q, %v", installs, err)
	}
}

func TestDependencyCacheConcurrentMisses(t *testing.T) {
	tc, cache, _ := dependencyTestTools(t)
	dirs := []string{t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir()}
	var wg sync.WaitGroup
	errors := make(chan error, len(dirs))
	for _, dir := range dirs {
		wg.Go(func() { errors <- installRenderer(dir, io.Discard, io.Discard, tc) })
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	var target string
	for _, dir := range dirs {
		link, err := os.Readlink(filepath.Join(dir, "node_modules"))
		if err != nil || (target != "" && target != link) {
			t.Fatalf("concurrent builds did not use one completed installation: %s, %v", link, err)
		}
		target = link
	}
	entries, err := os.ReadDir(filepath.Join(cache, "dependencies"))
	if err != nil || len(entries) != 1 || strings.HasPrefix(entries[0].Name(), ".install-") {
		t.Fatalf("unfinished or duplicate cache installations: %v, %v", entries, err)
	}
}

func TestDependencyCacheFailedInstall(t *testing.T) {
	tc, cache, log := dependencyTestTools(t)
	t.Setenv("CFGB_TEST_INSTALL_FAIL", "1")
	err := installRenderer(t.TempDir(), io.Discard, io.Discard, tc)
	var installErr *dependencyInstallError
	if !errors.As(err, &installErr) {
		t.Fatalf("installer failure was hidden: %v", err)
	}
	entries, err := os.ReadDir(filepath.Join(cache, "dependencies"))
	if err != nil || len(entries) != 0 {
		t.Fatalf("failed install was retained: %v, %v", entries, err)
	}
	installs, err := os.ReadFile(log)
	if err != nil || string(installs) != "install\n" {
		t.Fatalf("failed installer was retried as a cache miss: %q, %v", installs, err)
	}
}

func TestDependencyCacheFallback(t *testing.T) {
	for _, situation := range []string{"cache path is a file", "incomplete entry", "missing executable", "disabled"} {
		t.Run(situation, func(t *testing.T) {
			tc, cache, _ := dependencyTestTools(t)
			dir := t.TempDir()
			switch situation {
			case "cache path is a file":
				if err := os.WriteFile(cache, []byte("keep this file"), 0o644); err != nil {
					t.Fatal(err)
				}
			case "incomplete entry", "missing executable":
				entry, err := cachedDependencies(dir, io.Discard, io.Discard, tc)
				if err != nil {
					t.Fatal(err)
				}
				name := "ready"
				if situation == "missing executable" {
					name = filepath.Join("node_modules", ".bin", "astro")
				}
				if err := os.Remove(filepath.Join(entry, name)); err != nil {
					t.Fatal(err)
				}
			case "disabled":
				t.Setenv("CFGB_DEPENDENCY_CACHE", "0")
			}
			var warning bytes.Buffer
			if err := installRenderer(dir, io.Discard, &warning, tc); err != nil {
				t.Fatal(err)
			}
			info, err := os.Lstat(filepath.Join(dir, "node_modules"))
			if err != nil || !info.IsDir() {
				t.Fatalf("fallback did not install in this workspace: %v, %v", info, err)
			}
			if situation != "disabled" && !strings.Contains(warning.String(), "cache unavailable") {
				t.Fatalf("cache failure was silent: %s", warning.String())
			}
		})
	}
}

func TestDependencyCacheWarningFailure(t *testing.T) {
	tc, cache, _ := dependencyTestTools(t)
	if err := os.WriteFile(cache, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	cause := errors.New("warning write failed")
	err := installRenderer(t.TempDir(), io.Discard, progressWriter(func([]byte) (int, error) { return 0, cause }), tc)
	if !errors.Is(err, cause) {
		t.Fatalf("warning failure was discarded: %v", err)
	}
}
