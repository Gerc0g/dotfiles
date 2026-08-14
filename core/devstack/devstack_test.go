package devstack

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDBName(t *testing.T) {
	cases := map[[2]string]string{
		{"agents", "synapse"}:        "agents__synapse",
		{"Shop-App", "My.Repo"}:      "shop_app__myrepo",
		{"aetheria", "Aetheria-App"}: "aetheria__aetheria_app",
	}
	for in, want := range cases {
		if got := DBName(in[0], in[1]); got != want {
			t.Errorf("DBName(%q, %q) = %q, want %q", in[0], in[1], got, want)
		}
	}
}

func TestEnsureEnvrcBlock(t *testing.T) {
	dir := t.TempDir()

	added, envrc, err := EnsureEnvrcBlock(dir, "agents__synapse")
	if err != nil {
		t.Fatal(err)
	}
	if !added {
		t.Fatal("first call must add the block")
	}

	data, err := os.ReadFile(envrc)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.HasPrefix(text, "source_up\n") {
		t.Errorf("fresh .envrc must start with source_up:\n%s", text)
	}
	if !strings.Contains(text, `export DATABASE_URL="postgresql://dev:dev@$H:5432/agents__synapse"`) {
		t.Errorf("DATABASE_URL missing:\n%s", text)
	}
	if !strings.Contains(text, `H="${DEV_STACK_HOST:-localhost}"`) {
		t.Errorf("host resolution must stay literal:\n%s", text)
	}

	// Idempotent: the block is not appended twice.
	added, _, err = EnsureEnvrcBlock(dir, "agents__synapse")
	if err != nil {
		t.Fatal(err)
	}
	if added {
		t.Error("second call must be a no-op")
	}
	if !HasEnvrcBlock(dir) {
		t.Error("HasEnvrcBlock must see the block")
	}

	// An existing .envrc without source_up keeps its content untouched above
	// the block (bash behaviour: source_up only on file creation).
	dir2 := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir2, ".envrc"), []byte("# custom\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := EnsureEnvrcBlock(dir2, "x__y"); err != nil {
		t.Fatal(err)
	}
	data2, _ := os.ReadFile(filepath.Join(dir2, ".envrc"))
	if !strings.HasPrefix(string(data2), "# custom\n") {
		t.Errorf("existing content must stay first:\n%s", data2)
	}
}

func TestResolveCtxExplicitArgs(t *testing.T) {
	ctx, err := ResolveCtx(t.TempDir(), t.TempDir(), "agents", "synapse")
	if err != nil {
		t.Fatal(err)
	}
	if ctx.DB != "agents__synapse" {
		t.Errorf("db: %s", ctx.DB)
	}

	if _, err := ResolveCtx(t.TempDir(), t.TempDir(), "", ""); err == nil {
		t.Error("no args outside root must fail")
	}
}
