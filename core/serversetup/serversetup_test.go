package serversetup

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func bundle(t *testing.T, version string) string {
	t.Helper()
	dir := t.TempDir()
	m := Manifest{Version: version, SchemaVersion: 1, Platform: "linux-amd64", Files: map[string]File{}}
	for name, content := range map[string]string{"hq": "binary", "codex/bin/codex": "provider", "cli/happier": "cli", "relay/happier-server": "relay", "relay/node_modules/@prisma/client/package.json": "{}", "relay/node_modules/.prisma/client/index.js": "module fixture", "relay/node_modules/.prisma/client/libquery_engine-debian-openssl-3.0.x.so.node": "engine fixture", "relay/prisma/sqlite/migrations/migration_lock.toml": "provider = \"sqlite\"", "relay/prisma/sqlite/migrations/0001/migration.sql": "-- fixture", "ui/index.html": "<html></html>", "skills/research/SKILL.md": "---\nname: research\n---\n", "templates/AGENTS.md.company.tmpl": "company", "templates/AGENTS.md.product.tmpl": "product", "templates/AGENTS.md.repo.tmpl": "repo"} {
		file := filepath.Join(dir, name)
		os.MkdirAll(filepath.Dir(file), 0755)
		os.WriteFile(file, []byte(content), 0755)
		sum := sha256.Sum256([]byte(content))
		m.Files[name] = File{SHA256: hex.EncodeToString(sum[:]), Mode: 0755}
	}
	data, _ := json.Marshal(m)
	os.WriteFile(filepath.Join(dir, "manifest.json"), data, 0644)
	return dir
}
func snap(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	filepath.WalkDir(root, func(p string, d os.DirEntry, e error) error {
		if e != nil {
			t.Fatal(e)
		}
		rel, _ := filepath.Rel(root, p)
		if d.Type()&os.ModeSymlink != 0 {
			v, _ := os.Readlink(p)
			out[rel] = "link:" + v
		} else if !d.IsDir() {
			v, _ := os.ReadFile(p)
			out[rel] = string(v)
		} else {
			out[rel] = "dir"
		}
		return nil
	})
	return out
}
func opts(t *testing.T, version string) Options {
	uid, gid := os.Getuid(), os.Getgid()
	if uid == 0 {
		uid, gid = 1001, 1001
	}
	return Options{Bundle: bundle(t, version), RootDir: t.TempDir(), PrivateURL: "https://hq.vpn.example", User: "agent", UID: uid, GID: gid}
}
func TestPreservesRelayAndFixesOwnerEnvironment(t *testing.T) {
	o := opts(t, "v1")
	relay := filepath.Join(o.RootDir, "etc/systemd/system/happier-server.service")
	os.MkdirAll(filepath.Dir(relay), 0755)
	os.WriteFile(relay, []byte("existing private relay"), 0644)
	if _, err := Run(o, false); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(relay); string(data) != "existing private relay" {
		t.Fatal("existing relay unit overwritten")
	}
	for _, name := range []string{"usr/local/bin/hq", "home/agent/.config/systemd/user/happier-daemon.default.service.d/90-hq-release.conf"} {
		data, _ := os.ReadFile(filepath.Join(o.RootDir, name))
		for _, expected := range []string{"HOME=/home/agent", "DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/", "HQ_CODEX_AUTH_FILE=/home/agent/.codex/auth.json", "HQ_PROJECTS_FILE=/home/agent/.local/state/hq/projects.json", "HQ_SKILLS_ROOT=/usr/local/lib/hq/current/skills", "HQ_CODEX_BINARY=/usr/local/lib/hq/current/codex/bin/codex", "HQ_TEMPLATES_ROOT=/usr/local/lib/hq/current/templates"} {
			if !strings.Contains(string(data), expected) {
				t.Errorf("%s lacks %s", name, expected)
			}
		}
	}
	for _, name := range []string{"happier-daemon.default.service", "happier-daemon.default.service.d/90-hq-release.conf"} {
		data, _ := os.ReadFile(filepath.Join(o.RootDir, "home/agent/.config/systemd/user", name))
		if !strings.Contains(string(data), "\nKillMode=process\n") {
			t.Errorf("%s must preserve detached sessions when the daemon restarts", name)
		}
	}
	dropin, _ := os.ReadFile(filepath.Join(o.RootDir, "home/agent/.config/systemd/user/happier-daemon.default.service.d/90-hq-release.conf"))
	for _, expected := range []string{"Environment=HAPPIER_SERVER_URL=https://hq.vpn.example", "Environment=HAPPIER_LOCAL_SERVER_URL=http://127.0.0.1:3005"} {
		if !strings.Contains(string(dropin), expected) {
			t.Errorf("daemon drop-in lacks %s", expected)
		}
	}
	info, _ := os.Stat(filepath.Join(o.RootDir, "usr/local/lib/hq/releases/v1"))
	if info.Mode().Perm() != 0755 {
		t.Fatalf("daemon cannot traverse release: %o", info.Mode().Perm())
	}
}
func TestFreshRelayUsesCompleteReleaseRuntime(t *testing.T) {
	o := opts(t, "v1")
	if _, err := Run(o, false); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(o.RootDir, "etc/systemd/system/happier-server.service"))
	for _, expected := range []string{"NODE_PATH=/usr/local/lib/hq/current/relay/node_modules", "PRISMA_QUERY_ENGINE_LIBRARY=/usr/local/lib/hq/current/relay/node_modules/.prisma/client/libquery_engine-debian-openssl-3.0.x.so.node", "HAPPIER_SQLITE_MIGRATIONS_DIR=/usr/local/lib/hq/current/relay/prisma/sqlite/migrations", "Environment=PUBLIC_URL=https://hq.vpn.example"} {
		if !strings.Contains(string(data), expected) {
			t.Errorf("fresh relay lacks %s", expected)
		}
	}
}
func TestRejectsUnmanagedRollbackTargetBeforeWriting(t *testing.T) {
	o := opts(t, "v1")
	dir := filepath.Join(o.RootDir, "usr/local/lib/hq")
	os.MkdirAll(dir, 0755)
	os.Symlink("/tmp/unmanaged", filepath.Join(dir, "current"))
	before := snap(t, o.RootDir)
	if _, err := Run(o, false); err == nil {
		t.Fatal("unmanaged current target accepted")
	}
	if !reflect.DeepEqual(before, snap(t, o.RootDir)) {
		t.Fatal("invalid update changed filesystem")
	}
}
func TestDryRunIsReadOnly(t *testing.T) {
	o := opts(t, "v1")
	before := snap(t, o.RootDir)
	r, e := Run(o, true)
	if e != nil {
		t.Fatal(e)
	}
	if len(r.Actions) == 0 || r.Applied {
		t.Fatal(r)
	}
	if !reflect.DeepEqual(before, snap(t, o.RootDir)) {
		t.Fatal("dry run wrote filesystem")
	}
}
func TestChecksumFailureWritesNothing(t *testing.T) {
	o := opts(t, "v1")
	os.WriteFile(filepath.Join(o.Bundle, "hq"), []byte("tampered"), 0755)
	before := snap(t, o.RootDir)
	if _, e := Run(o, false); e == nil {
		t.Fatal("bad checksum accepted")
	}
	if !reflect.DeepEqual(before, snap(t, o.RootDir)) {
		t.Fatal("invalid bundle wrote filesystem")
	}
}
func TestIncompleteRelayWritesNothing(t *testing.T) {
	o := opts(t, "v1")
	name := "relay/node_modules/.prisma/client/libquery_engine-debian-openssl-3.0.x.so.node"
	data, _ := os.ReadFile(filepath.Join(o.Bundle, "manifest.json"))
	var manifest Manifest
	json.Unmarshal(data, &manifest)
	delete(manifest.Files, name)
	os.Remove(filepath.Join(o.Bundle, name))
	data, _ = json.Marshal(manifest)
	os.WriteFile(filepath.Join(o.Bundle, "manifest.json"), data, 0644)
	before := snap(t, o.RootDir)
	if _, err := Run(o, false); err == nil || !strings.Contains(err.Error(), "relay") {
		t.Fatalf("incomplete relay accepted: %v", err)
	}
	if !reflect.DeepEqual(before, snap(t, o.RootDir)) {
		t.Fatal("incomplete bundle wrote filesystem")
	}
}
func TestInstallUpdateIdempotentAndDataPreserved(t *testing.T) {
	o := opts(t, "v1")
	data := filepath.Join(o.RootDir, "srv/hq-data/company/secret")
	os.MkdirAll(filepath.Dir(data), 0700)
	os.WriteFile(data, []byte("keep"), 0600)
	auth := filepath.Join(o.RootDir, "home/agent/.happier/access.key")
	os.MkdirAll(filepath.Dir(auth), 0700)
	os.WriteFile(auth, []byte("keep-auth"), 0600)
	if _, e := Run(o, false); e != nil {
		t.Fatal(e)
	}
	once := snap(t, o.RootDir)
	if _, e := Run(o, false); e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(once, snap(t, o.RootDir)) {
		t.Fatal("second apply changed state")
	}
	o.Bundle = bundle(t, "v2")
	if _, e := Run(o, false); e != nil {
		t.Fatal(e)
	}
	if b, _ := os.ReadFile(data); string(b) != "keep" {
		t.Fatal("data overwritten")
	}
	if b, _ := os.ReadFile(auth); string(b) != "keep-auth" {
		t.Fatal("auth overwritten")
	}
	prev, _ := os.Readlink(filepath.Join(o.RootDir, "usr/local/lib/hq/previous"))
	if prev != "releases/v1" {
		t.Fatalf("no rollback target: %s", prev)
	}
	current, _ := os.Readlink(filepath.Join(o.RootDir, "usr/local/lib/hq/current"))
	if current != "releases/v2" {
		t.Fatal(current)
	}
}
func TestRejectsCredentialAndTraversalBundle(t *testing.T) {
	for _, name := range []string{"../escape", "cli/auth.json", "ui/.env.production"} {
		t.Run(name, func(t *testing.T) {
			o := opts(t, "v1")
			data, _ := os.ReadFile(filepath.Join(o.Bundle, "manifest.json"))
			var m Manifest
			json.Unmarshal(data, &m)
			m.Files[name] = File{SHA256: "bad", Mode: 0644}
			data, _ = json.Marshal(m)
			os.WriteFile(filepath.Join(o.Bundle, "manifest.json"), data, 0644)
			if _, e := Run(o, false); e == nil {
				t.Fatal("unsafe bundle accepted")
			}
		})
	}
}
