// Package serversetup installs a verified local HQ release without importing
// credentials, replacing user data, or changing network exposure.
package serversetup

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"

	"github.com/Gerc0g/dotfiles/core/world"
)

type File struct {
	SHA256 string `json:"sha256"`
	Mode   uint32 `json:"mode"`
}
type Manifest struct {
	SchemaVersion int             `json:"schemaVersion"`
	Version       string          `json:"version"`
	Platform      string          `json:"platform"`
	Files         map[string]File `json:"files"`
	CodexImage    string          `json:"codexImage,omitempty"`
}
type Options struct {
	Bundle     string
	PrivateURL string
	RootDir    string
	User       string
	UID        int
	GID        int
}
type Action struct {
	Path      string `json:"path"`
	Operation string `json:"operation"`
}
type Report struct {
	Version   string   `json:"version"`
	Applied   bool     `json:"applied"`
	Actions   []Action `json:"actions"`
	NextSteps []string `json:"nextSteps"`
	Previous  string   `json:"previous,omitempty"`
}

func Run(options Options, dryRun bool) (Report, error) {
	o, err := normalize(options)
	if err != nil {
		return Report{}, err
	}
	manifest, manifestBytes, err := verifyBundle(o.Bundle)
	if err != nil {
		return Report{}, err
	}
	if options.RootDir == "" && !dryRun && (runtime.GOOS != "linux" || os.Geteuid() != 0) {
		return Report{}, errors.New("server apply requires Linux root; preview with --dry-run")
	}
	if options.RootDir == "" && !dryRun && manifest.Platform != "linux-"+runtime.GOARCH {
		return Report{}, errors.New("release platform does not match this server")
	}
	report := Report{Version: manifest.Version, Actions: []Action{}, NextSteps: activationSteps(o)}
	releaseRel := "usr/local/lib/hq/releases/" + manifest.Version
	managed := managedFiles(o, manifest.Platform)
	dirs := []string{"srv/hq-data", "srv/hq-data/runner", "srv/hq-data/runner/workspaces", "srv/hq-data/runner/workspaces/ordinary", "srv/projects", "home/" + o.User + "/Desktop/WikiPedik", "home/" + o.User + "/Desktop/WikiPedik/research", "home/" + o.User + "/Desktop/WikiPedik/dev", "home/" + o.User + "/.happier", "home/" + o.User + "/.local/bin"}
	for _, rel := range dirs {
		if _, err := safeDestination(o.RootDir, rel); err != nil {
			return Report{}, err
		}
		report.Actions = append(report.Actions, Action{Path: "/" + rel, Operation: "ensure private owner directory (preserve contents)"})
	}
	for rel := range managed {
		if _, err := safeDestination(o.RootDir, rel); err != nil {
			return Report{}, err
		}
		report.Actions = append(report.Actions, Action{Path: "/" + rel, Operation: "install managed runtime configuration"})
	}
	release, err := safeDestination(o.RootDir, releaseRel)
	if err != nil {
		return Report{}, err
	}
	report.Actions = append(report.Actions, Action{Path: "/" + releaseRel, Operation: "verify and install immutable release"})
	links := map[string]string{"usr/local/lib/hq/current": "releases/" + manifest.Version, "home/" + o.User + "/.local/bin/hq": "/usr/local/bin/hq", "home/" + o.User + "/.happier/cli/current": "/usr/local/lib/hq/current/cli"}
	for rel := range links {
		if _, err := safeLinkDestination(o.RootDir, rel); err != nil {
			return Report{}, err
		}
		report.Actions = append(report.Actions, Action{Path: "/" + rel, Operation: "point at verified release"})
	}
	current := filepath.Join(o.RootDir, "usr/local/lib/hq/current")
	old, err := os.Readlink(current)
	if err != nil && !os.IsNotExist(err) {
		return Report{}, errors.New("HQ current is not a managed symlink")
	}
	if old != "" && (!strings.HasPrefix(old, "releases/") || world.ValidateSegment(strings.TrimPrefix(old, "releases/")) != nil) {
		return Report{}, errors.New("HQ current must reference a managed releases/<version> target")
	}
	if _, err := safeLinkDestination(o.RootDir, "usr/local/lib/hq/previous"); err != nil {
		return Report{}, err
	}
	report.Previous = old
	if _, err := os.Stat(release); err == nil {
		if err := verifyInstalled(release, manifest, manifestBytes); err != nil {
			return Report{}, err
		}
	} else if !os.IsNotExist(err) {
		return Report{}, err
	}
	sort.Slice(report.Actions, func(i, j int) bool { return report.Actions[i].Path < report.Actions[j].Path })
	if dryRun {
		return report, nil
	}
	if err := installRelease(o, manifest, manifestBytes, release); err != nil {
		return Report{}, err
	}
	for _, rel := range dirs {
		dest, _ := safeDestination(o.RootDir, rel)
		if err := os.MkdirAll(dest, 0700); err != nil {
			return report, err
		}
		if err := os.Chmod(dest, 0700); err != nil {
			return report, err
		}
		if err := os.Chown(dest, o.UID, o.GID); err != nil {
			return report, err
		}
	}
	// New user-home ancestors must be owned by the daemon user. Existing login
	// and database trees are never walked, copied, removed or recursively chowned.
	for _, rel := range []string{"home/" + o.User, "home/" + o.User + "/Desktop", "home/" + o.User + "/.local", "home/" + o.User + "/.config", "home/" + o.User + "/.config/systemd", "home/" + o.User + "/.config/systemd/user", "home/" + o.User + "/.happier/cli"} {
		dest, _ := safeDestination(o.RootDir, rel)
		if err := os.MkdirAll(dest, 0700); err != nil {
			return report, err
		}
		if err := os.Chown(dest, o.UID, o.GID); err != nil {
			return report, err
		}
	}
	for rel, content := range managed {
		dest, _ := safeDestination(o.RootDir, rel)
		if err := writeManaged(dest, []byte(content.content), content.mode, content.preserve); err != nil {
			return report, err
		}
		if strings.HasPrefix(rel, "home/"+o.User+"/") {
			if err := os.Chown(dest, o.UID, o.GID); err != nil {
				return report, err
			}
		}
	}
	if old != "" && old != "releases/"+manifest.Version {
		previous, _ := safeLinkDestination(o.RootDir, "usr/local/lib/hq/previous")
		if err := replaceSymlink(previous, old); err != nil {
			return report, err
		}
	}
	for rel, target := range links {
		dest, _ := safeLinkDestination(o.RootDir, rel)
		if err := replaceSymlink(dest, target); err != nil {
			return report, err
		}
		if strings.HasPrefix(rel, "home/") {
			if err := os.Lchown(dest, o.UID, o.GID); err != nil {
				return report, err
			}
		}
	}
	report.Applied = true
	return report, nil
}
func normalize(o Options) (Options, error) {
	if o.Bundle == "" {
		return o, errors.New("a verified local --bundle directory is required")
	}
	if o.RootDir == "" {
		o.RootDir = "/"
	}
	if !filepath.IsAbs(o.RootDir) {
		return o, errors.New("root directory must be absolute")
	}
	if o.User == "" {
		o.User = "agent"
	}
	if world.ValidateSegment(o.User) != nil {
		return o, errors.New("invalid daemon user")
	}
	if o.UID <= 0 || o.GID < 0 {
		return o, errors.New("daemon uid/gid are required")
	}
	u, e := url.Parse(o.PrivateURL)
	if e != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" && u.Path != "/" {
		return o, errors.New("private URL must be an HTTPS origin without credentials")
	}
	if ip := net.ParseIP(u.Hostname()); ip != nil && !ip.IsPrivate() && !ip.IsLoopback() {
		return o, errors.New("public IP is not a private control URL")
	}
	if strings.Contains(o.PrivateURL, "%") || strings.ContainsAny(o.PrivateURL, "\r\n\x00") {
		return o, errors.New("invalid private URL")
	}
	if net.ParseIP(u.Hostname()) == nil && !regexp.MustCompile(`^[A-Za-z0-9.-]+$`).MatchString(u.Hostname()) {
		return o, errors.New("invalid private hostname")
	}
	return o, nil
}
func safeDestination(root, rel string) (string, error) {
	return world.SafePath(root, strings.Split(filepath.ToSlash(rel), "/")...)
}
func safeLinkDestination(root, rel string) (string, error) {
	parent, err := safeDestination(root, path.Dir(rel))
	if err != nil {
		return "", err
	}
	dest := filepath.Join(parent, path.Base(rel))
	info, err := os.Lstat(dest)
	if err == nil && info.Mode()&os.ModeSymlink == 0 {
		return "", fmt.Errorf("refusing to replace unmanaged file %s", rel)
	}
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	return dest, nil
}
func sum(data []byte) string { value := sha256.Sum256(data); return hex.EncodeToString(value[:]) }
func validPayloadPath(name string) bool {
	if !fs.ValidPath(name) || name == "." || strings.Contains(name, "\\") {
		return false
	}
	if name != "hq" && !strings.HasPrefix(name, "cli/") && !strings.HasPrefix(name, "relay/") && !strings.HasPrefix(name, "ui/") && !strings.HasPrefix(name, "skills/") && !strings.HasPrefix(name, "templates/") && !strings.HasPrefix(name, "codex/") {
		return false
	}
	for _, part := range strings.Split(name, "/") {
		lower := strings.ToLower(part)
		if lower == ".git" || lower == ".happier" || lower == ".codex" || lower == "auth.json" || lower == "access.key" || strings.HasPrefix(lower, ".env") || lower == "credentials.json" {
			return false
		}
	}
	return true
}
func verifyBundle(dir string) (Manifest, []byte, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return Manifest{}, nil, err
	}
	defer root.Close()
	data, err := root.ReadFile("manifest.json")
	if err != nil {
		return Manifest{}, nil, err
	}
	var m Manifest
	if err = json.Unmarshal(data, &m); err != nil {
		return m, nil, err
	}
	if m.SchemaVersion != 1 || world.ValidateSegment(m.Version) != nil || m.Platform != "linux-amd64" && m.Platform != "linux-arm64" {
		return m, nil, errors.New("unsupported release manifest")
	}
	for _, name := range []string{"hq", "cli/happier", "relay/happier-server", "codex/bin/codex", "ui/index.html"} {
		item, ok := m.Files[name]
		if !ok {
			return m, nil, fmt.Errorf("release lacks %s", name)
		}
		if name != "ui/index.html" && item.Mode != 0755 {
			return m, nil, fmt.Errorf("release entrypoint is not executable: %s", name)
		}
	}
	for _, name := range []string{"templates/AGENTS.md.company.tmpl", "templates/AGENTS.md.product.tmpl", "templates/AGENTS.md.repo.tmpl"} {
		if _, ok := m.Files[name]; !ok {
			return m, nil, fmt.Errorf("release lacks %s", name)
		}
	}
	for _, name := range []string{"relay/node_modules/@prisma/client/package.json", "relay/node_modules/.prisma/client/index.js", "relay/node_modules/.prisma/client/" + relayEngine(m.Platform), "relay/prisma/sqlite/migrations/migration_lock.toml"} {
		if _, ok := m.Files[name]; !ok {
			return m, nil, fmt.Errorf("relay runtime closure lacks %s", name)
		}
	}
	if m.CodexImage != "" && !regexp.MustCompile(`^[A-Za-z0-9./:_-]+@sha256:[a-f0-9]{64}$`).MatchString(m.CodexImage) {
		return m, nil, errors.New("Codex image must be pinned by digest")
	}
	skills, migrations := false, false
	for name, item := range m.Files {
		if !validPayloadPath(name) || item.Mode != 0644 && item.Mode != 0755 {
			return m, nil, fmt.Errorf("unsafe release entry %s", name)
		}
		if strings.HasPrefix(name, "skills/") && strings.HasSuffix(name, "/SKILL.md") {
			skills = true
		}
		if strings.HasPrefix(name, "relay/prisma/sqlite/migrations/") && strings.HasSuffix(name, "/migration.sql") {
			migrations = true
		}
		bytes, err := root.ReadFile(name)
		if err != nil || sum(bytes) != item.SHA256 {
			return m, nil, fmt.Errorf("release checksum mismatch: %s", name)
		}
	}
	if !skills {
		return m, nil, errors.New("release lacks skills")
	}
	if !migrations {
		return m, nil, errors.New("relay runtime closure lacks SQLite migrations")
	}
	err = fs.WalkDir(root.FS(), ".", func(name string, entry fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return errors.New("release symlinks must be materialized by the trusted producer")
		}
		if entry.IsDir() || name == "manifest.json" {
			return nil
		}
		if _, ok := m.Files[name]; !ok {
			return fmt.Errorf("unlisted release file %s", name)
		}
		return nil
	})
	return m, data, err
}
func verifyInstalled(dir string, m Manifest, manifest []byte) error {
	_, actual, err := verifyBundle(dir)
	if err != nil {
		return fmt.Errorf("installed immutable release is damaged: %w", err)
	}
	if string(actual) != string(manifest) {
		return errors.New("release version already exists with another manifest")
	}
	return nil
}

func installRelease(o Options, m Manifest, manifest []byte, dest string) error {
	if _, err := os.Stat(dest); err == nil {
		return nil
	}
	parent := filepath.Dir(dest)
	if err := os.MkdirAll(parent, 0755); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(parent, ".hq-stage-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	if err := os.Chmod(stage, 0755); err != nil {
		return err
	}
	root, err := os.OpenRoot(o.Bundle)
	if err != nil {
		return err
	}
	defer root.Close()
	for name, item := range m.Files {
		data, err := root.ReadFile(name)
		if err != nil || sum(data) != item.SHA256 {
			return errors.New("bundle changed while copying")
		}
		out := filepath.Join(stage, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(out), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(out, data, fs.FileMode(item.Mode)); err != nil {
			return err
		}
	}
	if err := os.WriteFile(filepath.Join(stage, "manifest.json"), manifest, 0644); err != nil {
		return err
	}
	return os.Rename(stage, dest)
}
func writeManaged(dest string, data []byte, mode fs.FileMode, preserve bool) error {
	old, err := os.ReadFile(dest)
	if err == nil && (preserve || string(old) == string(data)) {
		return nil
	}
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(dest), ".hq-config-")
	if err != nil {
		return err
	}
	name := temp.Name()
	defer os.Remove(name)
	if _, err = temp.Write(data); err != nil {
		temp.Close()
		return err
	}
	if err = temp.Chmod(mode); err != nil {
		temp.Close()
		return err
	}
	if err = temp.Close(); err != nil {
		return err
	}
	return os.Rename(name, dest)
}
func replaceSymlink(dest, target string) error {
	if old, e := os.Readlink(dest); e == nil && old == target {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(dest), ".hq-link-")
	if err != nil {
		return err
	}
	name := temp.Name()
	temp.Close()
	os.Remove(name)
	defer os.Remove(name)
	if err = os.Symlink(target, name); err != nil {
		return err
	}
	return os.Rename(name, dest)
}
