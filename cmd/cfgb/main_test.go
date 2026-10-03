package main

import (
	"bytes"
	"testing"

	cfgb "github.com/ymmt2005/cfgb"
	"github.com/ymmt2005/cfgb/internal/version"
)

func TestVersion(t *testing.T) {
	var out, err bytes.Buffer
	if code := run([]string{"version"}, &out, &err); code != 0 {
		t.Fatalf("exit %d: %s", code, err.String())
	}
	want := "cfgb " + version.Version + "\n"
	if out.String() != want {
		t.Fatalf("got %q, want %q", out.String(), want)
	}
}

func TestUnknownCommand(t *testing.T) {
	var out, err bytes.Buffer
	if code := run([]string{"deploy"}, &out, &err); code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
}

func TestEmbeddedAssets(t *testing.T) {
	for _, name := range []string{
		"schemas/cfgb.schema.json",
		"schemas/article.schema.json",
		"prompts/summary-ja-v1.txt",
		"prompts/summary-en-v1.txt",
		"toolchain-requirements.json",
		"renderer/package.json",
		"renderer/package-lock.json",
		"renderer/pnpm-lock.yaml",
		"renderer/pnpm-workspace.yaml",
		"renderer/astro.config.mjs",
		"renderer/src/content.config.ts",
	} {
		if _, err := cfgb.FS.ReadFile(name); err != nil {
			t.Errorf("missing embedded %s: %v", name, err)
		}
	}
}
