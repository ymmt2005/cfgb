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
	for _, version := range []string{"24.14.9", "25.0.0", "25.9.0", "22.23.3", "20.19.0"} {
		if nodeInRange(version, constraint) {
			t.Errorf("%s should be rejected", version)
		}
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
