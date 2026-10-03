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
