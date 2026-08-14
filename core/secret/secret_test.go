package secret

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func makeSecretWorld(t *testing.T) (root string) {
	t.Helper()
	root = t.TempDir()

	repo := filepath.Join(root, "acme", "shop-dir", "backend")
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	for path, content := range map[string]string{
		filepath.Join(root, "acme", ".company-config"):             "slug: acme\ngit_email: dev@acme.test\n",
		filepath.Join(root, "acme", "shop-dir", ".product-config"): "slug: Shop\n",
	} {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestResolveAndNaming(t *testing.T) {
	root := makeSecretWorld(t)

	ctx, err := Resolve(root, filepath.Join(root, "acme", "shop-dir", "backend"), "")
	if err != nil {
		t.Fatal(err)
	}
	if ctx.Company != "acme" || ctx.Product != "Shop" || ctx.Repo != "backend" {
		t.Fatalf("ctx: %+v", ctx)
	}
	if ctx.AutoScope() != ScopeRepo {
		t.Errorf("auto scope: %s", ctx.AutoScope())
	}
	if ctx.Vault() != "Work-acme" {
		t.Errorf("vault: %s", ctx.Vault())
	}

	// Product slug override "Shop" must be slugified to lowercase.
	if got := ctx.ItemName(ScopeRepo, "OPENAI_API_KEY"); got != "shop__backend__OPENAI_API_KEY" {
		t.Errorf("repo item: %s", got)
	}
	if got := ctx.ItemName(ScopeProduct, "TOKEN"); got != "shop__TOKEN" {
		t.Errorf("product item: %s", got)
	}
	if got := ctx.ItemName(ScopeCompany, "TOKEN"); got != "_company__TOKEN" {
		t.Errorf("company item: %s", got)
	}

	// Explicit company pins the context to company scope.
	ctx, err = Resolve(root, t.TempDir(), "acme")
	if err != nil {
		t.Fatal(err)
	}
	if ctx.AutoScope() != ScopeCompany {
		t.Errorf("explicit company scope: %s", ctx.AutoScope())
	}
	if err := ctx.Require(ScopeRepo); err == nil {
		t.Error("repo scope without repo context must fail")
	}
}

func TestEnsureEnvrc(t *testing.T) {
	root := makeSecretWorld(t)
	ctx, err := Resolve(root, filepath.Join(root, "acme", "shop-dir", "backend"), "")
	if err != nil {
		t.Fatal(err)
	}

	added, envrc, err := ctx.EnsureEnvrc(ScopeRepo, "TOKEN", "Work-acme", "shop__backend__TOKEN")
	if err != nil {
		t.Fatal(err)
	}
	if !added {
		t.Fatal("first write must add the line")
	}

	content, err := os.ReadFile(envrc)
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	if !strings.HasPrefix(text, "source_up\n") {
		t.Errorf("repo .envrc must start with source_up:\n%s", text)
	}
	if !strings.Contains(text, `export TOKEN="$(bash "$HOME/dotfiles/scripts/secret-cache.sh" get Work-acme shop__backend__TOKEN credential)"`) {
		t.Errorf("export line missing:\n%s", text)
	}

	// Second call must be a no-op.
	added, _, err = ctx.EnsureEnvrc(ScopeRepo, "TOKEN", "Work-acme", "shop__backend__TOKEN")
	if err != nil {
		t.Fatal(err)
	}
	if added {
		t.Error("duplicate export must not be added")
	}
	again, _ := os.ReadFile(envrc)
	if string(again) != text {
		t.Error("file must be unchanged on repeat")
	}
}

func TestCacheKeyMatchesBash(t *testing.T) {
	// Reference computed with: printf '%s' 'Work-acme|shop__TOKEN|credential' | shasum -a 256
	want := "f23f5128de7ba93f3be490642a3452887e8aa6923866de6703747862ee97ab6b"
	if got := cacheKey("Work-acme", "shop__TOKEN", "credential"); got != want {
		t.Errorf("cache key drifted from bash scheme:\n got %s\nwant %s", got, want)
	}
}

func TestCacheGetFreshAndStale(t *testing.T) {
	calls := 0
	c := &Cache{
		Root: t.TempDir(),
		TTL:  time.Hour,
		read: func(vault, item, field string) (string, error) {
			calls++
			return "value-" + item, nil
		},
	}

	got, err := c.Get("V", "I", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if got != "value-I" || calls != 1 {
		t.Fatalf("first get: %q calls=%d", got, calls)
	}

	// Fresh entry: no second read.
	if _, err := c.Get("V", "I", "", 0); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Errorf("fresh get must not re-read, calls=%d", calls)
	}

	// Stale entry: re-read.
	key := cacheKey("V", "I", "credential")
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(c.valueFile(key), old, old); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Get("V", "I", "", 0); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Errorf("stale get must re-read, calls=%d", calls)
	}

	// Bypass: forced re-read.
	c.Bypass = true
	if _, err := c.Get("V", "I", "", 0); err != nil {
		t.Fatal(err)
	}
	if calls != 3 {
		t.Errorf("bypass must re-read, calls=%d", calls)
	}

	// Clear removes the entry.
	if err := c.Clear("V", "I", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(c.valueFile(key)); !os.IsNotExist(err) {
		t.Error("clear must remove the value file")
	}
}
