package connections

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestServiceGitIgnoresInheritedHelpersHooksAndConfiguration(t *testing.T) {
	dir := t.TempDir()
	empty := filepath.Join(dir, "empty")
	if err := os.Mkdir(empty, 0700); err != nil {
		t.Fatal(err)
	}
	hostile := t.TempDir()
	marker := filepath.Join(hostile, "executed")
	hook := filepath.Join(hostile, "reference-transaction")
	if err := os.WriteFile(hook, []byte("#!/bin/sh\ntouch '"+marker+"'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(hostile, "config")
	if err := os.WriteFile(config, []byte("[core]\n hooksPath = "+hostile+"\n[credential]\n helper = !touch "+marker+"\n[http]\n sslVerify = false\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", config)
	t.Setenv("GIT_CONFIG_SYSTEM", config)
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "core.hooksPath")
	t.Setenv("GIT_CONFIG_VALUE_0", hostile)
	t.Setenv("GIT_CONFIG_PARAMETERS", "'http.sslVerify=false'")
	run := func(args ...string) string {
		t.Helper()
		out, err := serviceGit(context.Background(), dir, "", args...)
		if err != nil {
			t.Fatal(err)
		}
		return strings.TrimSpace(string(out))
	}
	for _, tc := range []struct{ key, want string }{{"credential.helper", ""}, {"core.hooksPath", empty}, {"http.sslVerify", "true"}, {"http.followRedirects", "false"}, {"protocol.file.allow", "never"}} {
		if got := run("config", "--get", tc.key); got != tc.want {
			t.Fatalf("unsafe %s", tc.key)
		}
	}
	// A real reference transaction must not execute an inherited hook.
	repo := filepath.Join(dir, "repo.git")
	run("init", "--bare", "--template="+empty, repo)
	tree := run("--git-dir="+repo, "mktree")
	commit := run("--git-dir="+repo, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.test", "commit-tree", tree, "-m", "fixture")
	run("--git-dir="+repo, "update-ref", "refs/heads/work/test", commit)
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("inherited hook or helper ran")
	}
	authorization := "Authorization: Basic " + base64.StdEncoding.EncodeToString([]byte("fixture:token"))
	out, err := serviceGit(context.Background(), dir, authorization, "config", "--get", "http.extraHeader")
	if err != nil || strings.TrimSpace(string(out)) != authorization {
		t.Fatal("controlled Git authentication was not applied")
	}
}
func TestBundleCannotSelectAnotherBranch(t *testing.T) {
	dir := t.TempDir()
	empty := filepath.Join(dir, "empty")
	if err := os.Mkdir(empty, 0700); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) string {
		t.Helper()
		out, err := serviceGit(context.Background(), dir, "", args...)
		if err != nil {
			t.Fatal(err)
		}
		return strings.TrimSpace(string(out))
	}
	repo := filepath.Join(dir, "repo.git")
	run("init", "--bare", "--template="+empty, repo)
	tree := run("--git-dir="+repo, "mktree")
	commit := run("--git-dir="+repo, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.test", "commit-tree", tree, "-m", "fixture")
	run("--git-dir="+repo, "update-ref", "refs/heads/main", commit)
	bundle := filepath.Join(dir, "source.bundle")
	run("--git-dir="+repo, "bundle", "create", bundle, "refs/heads/main")
	data, err := os.ReadFile(bundle)
	if err != nil {
		t.Fatal(err)
	}
	_, err = transfer(context.Background(), dir, GitConnection{Provider: "github", Host: "https://never-contact.example"}, Repository{RemotePath: "team/repo"}, "secret", "git.push", branchArgs{Branch: "work/test", BundleBase64: base64.StdEncoding.EncodeToString(data)})
	if err == nil || !strings.Contains(err.Error(), "only the requested branch") {
		t.Fatalf("unexpected bundle result: %v", err)
	}
}
