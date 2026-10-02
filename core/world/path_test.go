package world

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateSegment(t *testing.T) {
	for _, value := range []string{"acme", "Aetheria-AI", "app.v2", "repo_1"} {
		if err := ValidateSegment(value); err != nil {
			t.Errorf("%q: %v", value, err)
		}
	}
	for _, value := range []string{"", ".", "..", "../escape", "/tmp/escape", "a/b", `a\b`, "-option", "bad\nslug", "has space", ".hidden"} {
		if err := ValidateSegment(value); err == nil {
			t.Errorf("accepted %q", value)
		}
	}
}

func TestSafePathRejectsTraversalAndSymlinks(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "missing"), filepath.Join(root, "dangling")); err != nil {
		t.Fatal(err)
	}
	for _, parts := range [][]string{{"..", "escape"}, {"a/b"}, {"escape", "repo"}, {"dangling"}, {""}, {"."}, {`a\b`}} {
		if got, err := SafePath(root, parts...); err == nil {
			t.Errorf("accepted %q: %s", parts, got)
		}
	}
	got, err := SafePath(root, "company", "product", ".product-config")
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(resolved, "company", "product", ".product-config"); got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestSafePathSupportsConfiguredRootSymlinkAndMissingRoot(t *testing.T) {
	base, actual := t.TempDir(), t.TempDir()
	link := filepath.Join(base, "workspace")
	if err := os.Symlink(actual, link); err != nil {
		t.Fatal(err)
	}
	got, err := SafePath(filepath.Join(link, "new-root"), "repo")
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(actual)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(resolved, "new-root", "repo"); got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}
