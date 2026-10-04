package build

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDirectoryCopiesUseAncestorIdentity(t *testing.T) {
	for _, rooted := range []bool{false, true} {
		name := "filesystem destination"
		if rooted {
			name = "rooted destination"
		}
		t.Run(name, func(t *testing.T) {
			source := t.TempDir()
			deep := "shared"
			for range 80 {
				deep = filepath.Join(deep, "d")
			}
			if err := os.MkdirAll(filepath.Join(source, deep), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(source, deep, "leaf.txt"), []byte("leaf"), 0o644); err != nil {
				t.Fatal(err)
			}
			for _, alias := range []string{"alias-a", "alias-b"} {
				if err := os.Symlink("shared", filepath.Join(source, alias)); err != nil {
					t.Fatal(err)
				}
			}
			root, err := os.OpenRoot(source)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := root.Close(); err != nil {
					t.Error(err)
				}
			})
			copyTree := func(destination string) error {
				if !rooted {
					return copyFromRoot(root, ".", filepath.Join(destination, "copy"))
				}
				target, err := os.OpenRoot(destination)
				if err != nil {
					return err
				}
				err = copyRootToRoot(root, ".", target, "copy")
				return errors.Join(err, target.Close())
			}
			destination := t.TempDir()
			if err := copyTree(destination); err != nil {
				t.Fatal(err)
			}
			for _, branch := range []string{"shared", "alias-a", "alias-b"} {
				leaf := filepath.Join(destination, "copy", branch, strings.TrimPrefix(deep, "shared"+string(filepath.Separator)), "leaf.txt")
				if raw, err := os.ReadFile(leaf); err != nil || string(raw) != "leaf" {
					t.Fatalf("copied %s: %q, %v", branch, raw, err)
				}
			}
			for _, loop := range []string{"root", "nested"} {
				t.Run(loop+" cycle", func(t *testing.T) {
					link, target := filepath.Join(source, "loop"), "."
					if loop == "nested" {
						link, target = filepath.Join(source, "shared", "loop"), ".."
					}
					if err := os.Symlink(target, link); err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() {
						if err := os.Remove(link); err != nil {
							t.Error(err)
						}
					})
					if err := copyTree(t.TempDir()); err == nil || !strings.Contains(err.Error(), "directory cycle:") {
						t.Fatalf("cycle not identified: %v", err)
					}
				})
			}
		})
	}
}
