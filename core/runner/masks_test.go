package runner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSecretsAndLocalProviderStateAreMaskedWithoutModifyingSources(t *testing.T) {
	root, _ := filepath.EvalSymlinks(t.TempDir())
	os.WriteFile(filepath.Join(root, ".env.local"), []byte("private"), 0600)
	os.Mkdir(filepath.Join(root, ".codex"), 0700)
	os.WriteFile(filepath.Join(root, "actual-credential"), []byte("private"), 0600)
	os.Symlink("actual-credential", filepath.Join(root, ".netrc"))
	masks, err := secretMasks(root)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{filepath.Join(root, ".env.local"): false, filepath.Join(root, ".codex"): true, filepath.Join(root, "actual-credential"): false}
	if len(masks) != len(want) {
		t.Fatalf("%+v", masks)
	}
	for _, m := range masks {
		directory, ok := want[m.Path]
		if !ok || m.Directory != directory {
			t.Fatalf("unexpected %+v", m)
		}
	}
	raw, _ := os.ReadFile(filepath.Join(root, ".env.local"))
	if string(raw) != "private" {
		t.Fatal("source credential modified")
	}
}

func TestProductMasksCredentialSymlinkTargetsInMountedSibling(t *testing.T) {
	root, _ := filepath.EvalSymlinks(t.TempDir())
	one, two := filepath.Join(root, "one"), filepath.Join(root, "two")
	os.Mkdir(one, 0700)
	os.Mkdir(two, 0700)
	os.WriteFile(filepath.Join(two, "private-data"), []byte("secret"), 0600)
	os.Symlink("../two/private-data", filepath.Join(one, ".env"))
	masks, err := secretMasks(one, two)
	if err != nil || len(masks) != 1 || masks[0].Path != filepath.Join(two, "private-data") {
		t.Fatalf("%+v %v", masks, err)
	}
}

func TestNestedRepositoryGitMetadataIsHidden(t *testing.T) {
	root, _ := filepath.EvalSymlinks(t.TempDir())
	for _, path := range []string{".git/config", "vendor/nested/.git/config"} {
		p := filepath.Join(root, path)
		os.MkdirAll(filepath.Dir(p), 0700)
		os.WriteFile(p, []byte("credential"), 0600)
	}
	masks, err := secretMasks(root)
	if err != nil || len(masks) != 1 || masks[0].Path != filepath.Join(root, "vendor/nested/.git") || !masks[0].Directory {
		t.Fatalf("%+v %v", masks, err)
	}
}
