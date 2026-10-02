package onboard

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Gerc0g/dotfiles/core/editor"
	"github.com/Gerc0g/dotfiles/core/world"
)

// sandbox points the world at a temp root and keeps the editor's project list
// out of the real one. The real templates are used on purpose: rendering them
// is most of what this package does, and a fixture copy would drift.
func sandbox(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv(world.RootEnv, root)
	t.Setenv(editor.ProjectsFileEnv, filepath.Join(t.TempDir(), "projects.json"))
	fakeHome(t)
	return root
}

func TestVCSHostResolvesShorthand(t *testing.T) {
	cases := map[string][2]string{
		"github":             {"github", "github.com"},
		"gitlab":             {"gitlab", "gitlab.com"},
		"bitbucket":          {"bitbucket", "bitbucket.org"},
		"local":              {"local", "local"},
		"gitlab:git.acme.io": {"gitlab", "git.acme.io"},
	}
	for raw, want := range cases {
		vcs, host, err := vcsHost(raw)
		if err != nil {
			t.Errorf("%s: %v", raw, err)
			continue
		}
		if vcs != want[0] || host != want[1] {
			t.Errorf("%s → %s/%s, want %s/%s", raw, vcs, host, want[0], want[1])
		}
	}
	if _, _, err := vcsHost("perforce"); err == nil {
		t.Error("неизвестный VCS обязан быть отказом, а не молчаливым дефолтом")
	}
}

// Unknown variables stay literal: the templates carry $VARS the platform does
// not fill, and swallowing them would silently blank out instructions.
func TestExpandTemplateLeavesUnknownVarsAlone(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tpl")
	if err := os.WriteFile(path, []byte("co=$CO host=$HOST keep=$UNKNOWN\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := expandTemplate(path, map[string]string{"CO": "acme", "HOST": "git.acme.io"})
	if err != nil {
		t.Fatal(err)
	}
	if want := "co=acme host=git.acme.io keep=$UNKNOWN\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestCompanyCreatesWorkspaceAndRefusesDuplicate(t *testing.T) {
	root := sandbox(t)
	var out bytes.Buffer

	opts := CompanyOptions{
		Slug: "acme", VCS: "gitlab:git.acme.io",
		Namespace: "acme/dev", Email: "dev@acme.io",
		SkipExternal: true,
	}
	if err := Company(opts, &out); err != nil {
		t.Fatal(err)
	}

	dir := filepath.Join(root, "acme")
	for _, name := range []string{"AGENTS.md", ".company-config", ".envrc", "README.md"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("нет %s: %v", name, err)
		}
	}

	config := read(t, filepath.Join(dir, ".company-config"))
	for _, want := range []string{
		"slug: acme", "vcs: gitlab", "host: git.acme.io",
		"namespace: acme/dev", "ssh_host: gitlab-acme", "git_email: dev@acme.io",
	} {
		if !strings.Contains(config, want) {
			t.Errorf(".company-config без %q:\n%s", want, config)
		}
	}

	// The git identity is the reason companies are separate at all.
	if envrc := read(t, filepath.Join(dir, ".envrc")); !strings.Contains(envrc, "dev@acme.io") {
		t.Errorf(".envrc не несёт почту компании:\n%s", envrc)
	}

	// A second run must not overwrite a live company.
	if err := Company(opts, &out); err == nil {
		t.Error("повторное создание компании должно отказывать, а не перетирать")
	}
}

// SkipExternal keeps SSH setup off the real machine. The
// home directory here is a fresh one, so anything written outside the
// workspace shows up as a directory that should not exist.
func TestCompanySkipExternalTouchesNothingOutside(t *testing.T) {
	root := sandbox(t)
	home := fakeHome(t)

	var out bytes.Buffer
	// A gitlab company is the case that wants an SSH identity; local would
	// skip the keygen on its own and prove nothing.
	err := Company(CompanyOptions{
		Slug: "probe", VCS: "gitlab", Namespace: "n", Email: "a@b.c", SkipExternal: true}, &out)
	if err != nil {
		t.Fatal(err)
	}

	if _, statErr := os.Stat(filepath.Join(home, ".ssh")); statErr == nil {
		t.Error("SkipExternal всё равно создал ~/.ssh")
	}
	if strings.Contains(out.String(), "vault") {
		t.Errorf("SkipExternal всё равно ходил в 1Password:\n%s", out.String())
	}
	if _, statErr := os.Stat(filepath.Join(root, "probe")); statErr != nil {
		t.Errorf("компания не создана: %v", statErr)
	}
}

// fakeHome gives the test its own home directory, carrying a copy of the real
// templates so rendering still exercises what ships.
func fakeHome(t *testing.T) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	source := filepath.Join(filepath.Dir(file), "..", "..", "templates")
	entries, err := os.ReadDir(source)
	if err != nil {
		t.Skipf("шаблоны платформы недоступны: %v", err)
	}

	home := t.TempDir()
	target := filepath.Join(home, "dotfiles", "templates")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		body, err := os.ReadFile(filepath.Join(source, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(target, entry.Name()), body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", home)
	return home
}

func TestProductNeedsAnExistingCompany(t *testing.T) {
	sandbox(t)
	var out bytes.Buffer
	err := Product(ProductOptions{Company: "ghost", Product: "app"}, &out)
	if err == nil {
		t.Fatal("продукт в несуществующей компании должен отказывать")
	}
	if !strings.Contains(err.Error(), "ghost") {
		t.Errorf("ошибка не называет компанию: %v", err)
	}
}

func TestProductCreatesRepoFromLocalCompany(t *testing.T) {
	root := sandbox(t)
	var out bytes.Buffer

	// A local company clones nothing: git init instead of the network.
	if err := Company(CompanyOptions{
		Slug: "solo", VCS: "local", Namespace: "solo", SkipExternal: true}, &out); err != nil {
		t.Fatal(err)
	}
	if err := Product(ProductOptions{
		Company: "solo", Product: "tools", Repos: []string{"cli"}}, &out); err != nil {
		t.Fatal(err)
	}

	prod := filepath.Join(root, "solo", "tools")
	for _, name := range []string{"AGENTS.md", ".product-config", ".envrc"} {
		if _, err := os.Stat(filepath.Join(prod, name)); err != nil {
			t.Errorf("нет %s продукта: %v", name, err)
		}
	}
	if config := read(t, filepath.Join(prod, ".product-config")); !strings.Contains(config, "slug: tools") {
		t.Errorf(".product-config неверен:\n%s", config)
	}

	repo := filepath.Join(prod, "cli")
	for _, name := range []string{".git", "AGENTS.md", ".envrc"} {
		if _, err := os.Stat(filepath.Join(repo, name)); err != nil {
			t.Errorf("репозиторий без %s: %v", name, err)
		}
	}
}

// Re-running must not clobber a repo someone has been working in.
func TestProductKeepsExistingRepoFiles(t *testing.T) {
	root := sandbox(t)
	var out bytes.Buffer

	if err := Company(CompanyOptions{
		Slug: "keep", VCS: "local", Namespace: "keep", SkipExternal: true}, &out); err != nil {
		t.Fatal(err)
	}
	if err := Product(ProductOptions{
		Company: "keep", Product: "p", Repos: []string{"r"}}, &out); err != nil {
		t.Fatal(err)
	}

	agents := filepath.Join(root, "keep", "p", "r", "AGENTS.md")
	if err := os.WriteFile(agents, []byte("# правки человека\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Product(ProductOptions{
		Company: "keep", Product: "p", Repos: []string{"r"}}, &out); err != nil {
		t.Fatal(err)
	}
	if got := read(t, agents); got != "# правки человека\n" {
		t.Errorf("повторный запуск затёр AGENTS.md репозитория:\n%s", got)
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
