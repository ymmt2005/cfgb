package build

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	cfgb "github.com/ymmt2005/cfgb"
)

func installRenderer(dir string, stdout, stderr io.Writer, tc toolchainCheck) error {
	if os.Getenv("CFGB_DEPENDENCY_CACHE") == "0" {
		return installRendererDirect(dir, stdout, stderr, tc)
	}
	entry, err := cachedDependencies(dir, stdout, stderr, tc)
	if err != nil {
		var installErr *dependencyInstallError
		if errors.As(err, &installErr) {
			// A package-manager failure is a build failure, not a cache miss.
			return err
		}
		return installWithoutCache(dir, stdout, stderr, tc, err)
	}
	if err := os.Symlink(filepath.Join(entry, "node_modules"), filepath.Join(dir, "node_modules")); err != nil {
		return installWithoutCache(dir, stdout, stderr, tc, fmt.Errorf("link cached dependencies: %w", err))
	}
	if _, err := fmt.Fprintln(stdout, "using cached renderer dependencies"); err != nil {
		return fmt.Errorf("write build progress: %w", err)
	}
	return nil
}

// Cache I/O/probe/link failures are reported before a normal frozen install.
// This optional accelerator never converts a failed read into a valid entry.
func installWithoutCache(dir string, stdout, stderr io.Writer, tc toolchainCheck, cause error) error {
	if _, err := fmt.Fprintf(stderr, "warning: renderer dependency cache unavailable: %v; installing in this build workspace\n", cause); err != nil {
		return errors.Join(cause, fmt.Errorf("write cache warning: %w", err))
	}
	return installRendererDirect(dir, stdout, stderr, tc)
}

type dependencyInstallError struct{ err error }

func (e *dependencyInstallError) Error() string { return e.err.Error() }
func (e *dependencyInstallError) Unwrap() error { return e.err }

func dependencyCacheDir() (string, error) {
	base := os.Getenv("CFGB_CACHE_DIR")
	if base == "" {
		userCache, err := os.UserCacheDir()
		if err != nil {
			return "", fmt.Errorf("find user cache directory: %w", err)
		}
		base = filepath.Join(userCache, "cfgb")
	}
	base, err := filepath.Abs(base)
	if err != nil {
		return "", fmt.Errorf("resolve dependency cache directory: %w", err)
	}
	return filepath.Join(base, "dependencies"), nil
}

func dependencyInputNames(manager string) []string {
	if manager == "pnpm" {
		return []string{"package.json", "pnpm-lock.yaml", "pnpm-workspace.yaml"}
	}
	return []string{"package.json", "package-lock.json"}
}

func dependencyCacheKey(dir string, tc toolchainCheck) (string, error) {
	// Probe Node itself: its platform need not match the CLI's architecture.
	platform, err := output("node", "-p", `JSON.stringify({platform:process.platform,arch:process.arch,abi:process.versions.modules,glibc:process.report.getReport().header.glibcVersionRuntime||null})`)
	if err != nil {
		return "", fmt.Errorf("probe dependency platform: %w", err)
	}
	cmd := exec.Command(tc.PackageManager, "config", "list", "--json")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "COREPACK_ENABLE_AUTO_PIN=0")
	raw, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("read %s install configuration: %w", tc.PackageManager, err)
	}
	var configuration map[string]json.RawMessage
	if err := json.Unmarshal(raw, &configuration); err != nil {
		return "", fmt.Errorf("decode %s install configuration: %w", tc.PackageManager, err)
	}
	// Storage/reporting settings do not change the installed dependency bytes.
	for _, name := range []string{"cache", "logs-dir", "timing", "progress", "loglevel", "fund", "audit"} {
		delete(configuration, name)
	}
	inputs := make(map[string][]byte)
	for _, name := range dependencyInputNames(tc.PackageManager) {
		raw, err := cfgb.FS.ReadFile("renderer/" + name)
		if err != nil {
			return "", fmt.Errorf("read dependency input %s: %w", name, err)
		}
		inputs[name] = raw
	}
	return hashDependencyInputs(tc, platform, os.Getenv("NODE_ENV"), configuration, inputs)
}

func hashDependencyInputs(tc toolchainCheck, platform, nodeEnv string, configuration map[string]json.RawMessage, inputs map[string][]byte) (string, error) {
	// Configuration may contain secrets. Only its digest is retained, never the
	// configuration itself; neither it nor platform probe output is printed.
	configJSON, err := json.Marshal(configuration)
	if err != nil {
		return "", fmt.Errorf("encode installer configuration: %w", err)
	}
	// pnpm can serialize nested registry/build-policy maps in different orders.
	// Canonicalize nested objects too; preserve configuration number precision.
	var normalized any
	decoder := json.NewDecoder(bytes.NewReader(configJSON))
	decoder.UseNumber()
	if err := decoder.Decode(&normalized); err != nil {
		return "", fmt.Errorf("normalize installer configuration: %w", err)
	}
	raw, err := json.Marshal(struct {
		Version       int
		Toolchain     toolchainCheck
		Platform      string
		NodeEnv       string
		Configuration any
		Inputs        map[string][]byte
	}{1, tc, platform, nodeEnv, normalized, inputs})
	if err != nil {
		return "", fmt.Errorf("encode dependency cache inputs: %w", err)
	}
	return fmt.Sprintf("%x", sha256.Sum256(raw)), nil
}

func cachedDependencies(dir string, stdout, stderr io.Writer, tc toolchainCheck) (entry string, err error) {
	root, err := dependencyCacheDir()
	if err != nil {
		return "", err
	}
	key, err := dependencyCacheKey(dir, tc)
	if err != nil {
		return "", err
	}
	entry = filepath.Join(root, key)
	ready, err := dependencyEntryReady(entry, key)
	if err != nil || ready {
		return entry, err
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", fmt.Errorf("create dependency cache: %w", err)
	}
	staging, err := os.MkdirTemp(root, ".install-*")
	if err != nil {
		return "", fmt.Errorf("create dependency install directory: %w", err)
	}
	defer func() {
		if cleanupErr := os.RemoveAll(staging); cleanupErr != nil {
			err = errors.Join(err, fmt.Errorf("remove dependency install directory: %w", cleanupErr))
		}
	}()
	for _, name := range dependencyInputNames(tc.PackageManager) {
		raw, err := cfgb.FS.ReadFile("renderer/" + name)
		if err != nil {
			return "", fmt.Errorf("read dependency input %s: %w", name, err)
		}
		if err := os.WriteFile(filepath.Join(staging, name), raw, 0o644); err != nil {
			return "", fmt.Errorf("write dependency input %s: %w", name, err)
		}
	}
	if err := installRendererDirect(staging, stdout, stderr, tc); err != nil {
		return "", &dependencyInstallError{err}
	}
	if err := os.WriteFile(filepath.Join(staging, "ready"), []byte(key+"\n"), 0o644); err != nil {
		return "", fmt.Errorf("complete dependency cache: %w", err)
	}
	if _, err := dependencyEntryReady(staging, key); err != nil {
		return "", err
	}
	if err := os.Rename(staging, entry); err != nil {
		// Concurrent misses install separately and publish atomically. Never
		// overwrite a completed tree or reuse an in-progress installation.
		ready, checkErr := dependencyEntryReady(entry, key)
		if checkErr != nil || !ready {
			return "", errors.Join(fmt.Errorf("publish dependency cache: %w", err), checkErr)
		}
	}
	return entry, nil
}

func dependencyEntryReady(entry, key string) (bool, error) {
	info, err := os.Lstat(entry)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("inspect dependency cache: %w", err)
	}
	if !info.IsDir() {
		return false, fmt.Errorf("dependency cache entry is not a directory: %s", entry)
	}
	raw, err := os.ReadFile(filepath.Join(entry, "ready"))
	if err != nil {
		return false, fmt.Errorf("read dependency cache completion: %w", err)
	}
	if !bytes.Equal(raw, []byte(key+"\n")) {
		return false, fmt.Errorf("incomplete dependency cache entry: %s", entry)
	}
	for _, name := range []string{"astro", "pagefind", "wrangler"} {
		info, err := os.Stat(filepath.Join(entry, "node_modules", ".bin", name))
		if err != nil {
			return false, fmt.Errorf("inspect cached %s: %w", name, err)
		}
		if !info.Mode().IsRegular() {
			return false, fmt.Errorf("cached %s is not a file", name)
		}
	}
	return true, nil
}
