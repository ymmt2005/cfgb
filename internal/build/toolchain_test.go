package build

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	cfgb "github.com/ymmt2005/cfgb"
)

func TestPackageManager(t *testing.T) {
	t.Setenv("CFGB_PACKAGE_MANAGER", "")
	if got, err := packageManager(); err != nil || got != "npm" {
		t.Fatalf("default = %s, %v", got, err)
	}
	t.Setenv("CFGB_PACKAGE_MANAGER", "npm")
	if got, err := packageManager(); err != nil || got != "npm" {
		t.Fatalf("npm = %s, %v", got, err)
	}
	t.Setenv("CFGB_PACKAGE_MANAGER", "pnpm")
	if got, err := packageManager(); err != nil || got != "pnpm" {
		t.Fatalf("pnpm = %s, %v", got, err)
	}
	t.Setenv("CFGB_PACKAGE_MANAGER", "yarn")
	if _, err := packageManager(); err == nil {
		t.Fatal("expected yarn to be rejected")
	}
}

func TestNodeRange(t *testing.T) {
	req, err := loadRequirements()
	if err != nil {
		t.Fatal(err)
	}
	const constraint = ">=24.15.0 <25 || >=26.0.0"
	if req.NodeRange != constraint {
		t.Fatalf("nodeRange = %s", req.NodeRange)
	}
	for _, version := range []string{"24.15.0", "24.21.0", "26.0.0", "26.10.0", "27.1.0"} {
		if !nodeInRange(version, constraint) {
			t.Errorf("%s should be allowed", version)
		}
	}
	for _, version := range []string{"24.14.9", "25.0.0", "25.9.0", "22.23.3", "20.19.0", "24.15.0-rc.1", "24.21.0-rc.1", "25.0.0-rc.1", "26.0.0-rc.1", "12.0.0garbage", "24.15"} {
		if nodeInRange(version, constraint) {
			t.Errorf("%s should be rejected", version)
		}
	}
	if _, err := parseSemver("12.0.0garbage"); err == nil {
		t.Fatal("malformed version was accepted")
	}
	if err := requireMinimumVersion("npm", "12.0.0garbage", minimumNpmVersion, "12.2.0"); err == nil {
		t.Fatal("malformed npm version was accepted")
	}
	if err := requireMinimumVersion("npm", "12.0.0-rc.1", minimumNpmVersion, "12.2.0"); err == nil {
		t.Fatal("npm prerelease met the stable floor")
	}
	if err := requireMinimumVersion("npm", "12.1.0-rc.1", minimumNpmVersion, "12.2.0"); err == nil {
		t.Fatal("npm prerelease above the floor was accepted")
	}
	if err := requireMinimumVersion("pnpm", "11.1.0-rc.1", minimumPnpmVersion, "12.8.1"); err == nil {
		t.Fatal("pnpm prerelease above the floor was accepted")
	}
	if err := requireMinimumVersion("npm", "12.2.0+build.1", minimumNpmVersion, "12.2.0"); err != nil {
		t.Fatal(err)
	}
}

func TestSemverPrecedence(t *testing.T) {
	ordered := []string{
		"1.0.0-alpha",
		"1.0.0-alpha.1",
		"1.0.0-alpha.beta",
		"1.0.0-beta",
		"1.0.0-beta.2",
		"1.0.0-beta.11",
		"1.0.0-rc.1",
		"1.0.0",
	}
	for i := 0; i < len(ordered)-1; i++ {
		left, err := parseSemver(ordered[i])
		if err != nil {
			t.Fatal(err)
		}
		right, err := parseSemver(ordered[i+1])
		if err != nil {
			t.Fatal(err)
		}
		if compareSemver(left, right) >= 0 {
			t.Fatalf("%s should be lower than %s", ordered[i], ordered[i+1])
		}
	}
	release, err := parseSemver("24.15.0")
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := parseSemver("24.15.0+build.1")
	if err != nil {
		t.Fatal(err)
	}
	if compareSemver(release, metadata) != 0 {
		t.Fatal("build metadata changed precedence")
	}
	const prereleaseRange = ">=24.15.0-rc.1 <25"
	if !nodeInRange("24.15.0-rc.2", prereleaseRange) || !nodeInRange("24.15.0", prereleaseRange) {
		t.Fatal("prerelease range should accept a later prerelease of the same version and the release")
	}
	if nodeInRange("24.15.0-rc.0", prereleaseRange) || nodeInRange("24.16.0-rc.1", prereleaseRange) {
		t.Fatal("prerelease range accepted an earlier or different prerelease")
	}
}

func TestLockfileHashes(t *testing.T) {
	req, err := loadRequirements()
	if err != nil {
		t.Fatal(err)
	}
	if req.PackageManager != "npm" {
		t.Fatalf("default package manager = %s", req.PackageManager)
	}
	if got := fileHash(t, "renderer/package-lock.json"); got != req.LockfileHash {
		t.Fatalf("package-lock.json hash = %s", got)
	}
	if got := fileHash(t, "renderer/pnpm-lock.yaml"); got != req.PnpmLockfileHash {
		t.Fatalf("pnpm-lock.yaml hash = %s", got)
	}
}

func fileHash(t *testing.T, name string) string {
	t.Helper()
	raw, err := cfgb.FS.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
