package connections

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type readArgs struct{}
type branchArgs struct {
	Branch       string `json:"branch"`
	BundleBase64 string `json:"bundleBase64,omitempty"`
}
type pullArgs struct {
	Branch string `json:"branch,omitempty"`
	Number int    `json:"number,omitempty"`
	Title  string `json:"title"`
	Body   string `json:"body"`
}
type environmentArgs struct {
	EnvironmentID string            `json:"environmentId"`
	RequestID     string            `json:"requestId"`
	Query         map[string]string `json:"query"`
}

func Execute(ctx context.Context, binding Binding, operation string, args json.RawMessage) (any, error) {
	if binding.RepoID == "" {
		return nil, errors.New("a registered repository binding is required")
	}
	if err := registered(binding.CompanyID, binding.RepoID); err != nil {
		return nil, err
	}
	// Keep the configuration lock through the remote operation: a successful
	// revocation or policy save must not be followed by a queued stale write.
	return locked(binding.CompanyID, func(dir string, c Config) (any, error) {
		if err := validate(c); err != nil {
			return nil, err
		}
		if operation == "environment.request" {
			var a environmentArgs
			if err := decode(args, &a); err != nil {
				return nil, err
			}
			for _, e := range c.Environments {
				if e.ID == a.EnvironmentID {
					token, err := readCredential(dir, e.CredentialRef)
					if err != nil {
						return nil, err
					}
					return environmentRequest(ctx, e, a.RequestID, a.Query, token)
				}
			}
			return nil, errors.New("environment is not in the bound company")
		}
		var r Repository
		for _, candidate := range c.Repositories {
			if candidate.RepoID == binding.RepoID {
				r = candidate
			}
		}
		if r.RepoID == "" {
			return nil, errors.New("repository has no company Git mapping")
		}
		var g GitConnection
		for _, candidate := range c.Git {
			if candidate.ID == r.ConnectionID {
				g = candidate
			}
		}
		if !g.Policy.Read {
			return nil, errors.New("repository reading is disabled")
		}
		switch operation {
		case "git.read":
			var a readArgs
			if err := decode(args, &a); err != nil {
				return nil, err
			}
		case "git.fetch":
			var a branchArgs
			if err := decode(args, &a); err != nil {
				return nil, err
			}
			if a.BundleBase64 != "" {
				return nil, errors.New("fetch does not accept a bundle")
			}
			if err := branchName(a.Branch); err != nil {
				return nil, err
			}
		case "git.push":
			if !g.Policy.BranchPush {
				return nil, errors.New("branch publication is disabled")
			}
			var a branchArgs
			if err := decode(args, &a); err != nil {
				return nil, err
			}
			if err := allowedBranch(g, a.Branch, ""); err != nil {
				return nil, err
			}
		case "git.pr.create", "git.pr.update":
			if !g.Policy.PullRequests {
				return nil, errors.New("PR/MR operations are disabled")
			}
			var a pullArgs
			if err := decode(args, &a); err != nil {
				return nil, err
			}
			if strings.TrimSpace(a.Title) == "" {
				return nil, errors.New("PR/MR title is required")
			}
			if operation == "git.pr.create" {
				if err := allowedBranch(g, a.Branch, ""); err != nil {
					return nil, err
				}
				if a.Number != 0 {
					return nil, errors.New("create does not accept a PR/MR number")
				}
			} else if a.Number < 1 || a.Branch != "" {
				return nil, errors.New("update requires a positive PR/MR number and no branch override")
			}
		default:
			return nil, errors.New("unsupported bound connection operation")
		}
		token, err := readCredential(dir, g.CredentialRef)
		if err != nil {
			return nil, err
		}
		ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
		repo, err := getRepo(ctx, g, r, token)
		if err != nil {
			return nil, err
		}
		if operation == "git.read" {
			return map[string]any{"repoId": r.RepoID, "remotePath": r.RemotePath, "provider": g.Provider, "defaultBranch": repo.DefaultBranch, "archived": repo.Archived, "policy": g.Policy, "authorName": g.AuthorName, "authorEmail": g.AuthorEmail}, nil
		}
		if operation == "git.fetch" || operation == "git.push" {
			var a branchArgs
			if err = decode(args, &a); err != nil {
				return nil, err
			}
			if operation == "git.push" {
				if repo.Archived {
					return nil, errors.New("repository is archived")
				}
				if err = allowedBranch(g, a.Branch, repo.DefaultBranch); err != nil {
					return nil, err
				}
				if err = protectedRemote(ctx, g, r, a.Branch, token); err != nil {
					return nil, err
				}
			}
			return transfer(ctx, dir, g, r, token, operation, a)
		}
		if repo.Archived {
			return nil, errors.New("repository is archived")
		}
		var a pullArgs
		if err = decode(args, &a); err != nil {
			return nil, err
		}
		return pullRequest(ctx, g, r, repo, token, operation, a)
	})
}

func pullRequest(ctx context.Context, g GitConnection, r Repository, repo remoteRepo, token, operation string, a pullArgs) (any, error) {
	endpoint := repoURL(g, r) + "/pulls"
	if g.Provider == "gitlab" {
		endpoint = repoURL(g, r) + "/merge_requests"
	}
	method := "POST"
	if operation == "git.pr.update" {
		endpoint += "/" + strconv.Itoa(a.Number)
		data, _, err := request(ctx, "GET", endpoint, token, nil)
		if err != nil {
			return nil, err
		}
		var pr struct {
			Head struct {
				Ref  string `json:"ref"`
				Repo struct {
					FullName string `json:"full_name"`
				} `json:"repo"`
			} `json:"head"`
			Base struct {
				Ref string `json:"ref"`
			} `json:"base"`
			SourceBranch    string `json:"source_branch"`
			TargetBranch    string `json:"target_branch"`
			SourceProjectID int    `json:"source_project_id"`
			TargetProjectID int    `json:"target_project_id"`
		}
		if json.Unmarshal(data, &pr) != nil {
			return nil, errors.New("invalid PR/MR response")
		}
		if g.Provider == "github" {
			if !strings.EqualFold(pr.Head.Repo.FullName, r.RemotePath) || pr.Base.Ref != g.Policy.TargetBranch {
				return nil, errors.New("PR is outside the bound repository or target branch")
			}
			a.Branch = pr.Head.Ref
			method = "PATCH"
		} else {
			if pr.SourceProjectID == 0 || pr.SourceProjectID != pr.TargetProjectID || pr.TargetBranch != g.Policy.TargetBranch {
				return nil, errors.New("MR is outside the bound repository or target branch")
			}
			a.Branch = pr.SourceBranch
			method = "PUT"
		}
	}
	if err := allowedBranch(g, a.Branch, repo.DefaultBranch); err != nil {
		return nil, err
	}
	if err := protectedRemote(ctx, g, r, a.Branch, token); err != nil {
		return nil, err
	}
	if err := checkRequired(ctx, g, r, a.Branch, token); err != nil {
		return nil, err
	}
	body := a.Body
	if body == "" {
		body = g.Policy.DescriptionTemplate
	}
	payload := map[string]any{"title": a.Title, "body": body}
	if g.Provider == "github" {
		if method == "POST" {
			payload["head"] = a.Branch
			payload["base"] = g.Policy.TargetBranch
			payload["draft"] = g.Policy.Draft
			payload["maintainer_can_modify"] = false
		}
	} else {
		delete(payload, "body")
		payload["description"] = body
		if g.Policy.Draft && !strings.HasPrefix(a.Title, "Draft:") {
			payload["title"] = "Draft: " + a.Title
		}
		if method == "POST" {
			payload["source_branch"] = a.Branch
			payload["target_branch"] = g.Policy.TargetBranch
			payload["remove_source_branch"] = false
		}
	}
	data, _, err := request(ctx, method, endpoint, token, payload)
	if err != nil {
		return nil, err
	}
	var result struct {
		Number  int    `json:"number"`
		IID     int    `json:"iid"`
		HTMLURL string `json:"html_url"`
		WebURL  string `json:"web_url"`
	}
	if json.Unmarshal(data, &result) != nil {
		return nil, errors.New("hosting operation completed but response was invalid; inspect hosting before retrying")
	}
	if g.Provider == "gitlab" {
		result.Number = result.IID
		result.HTMLURL = result.WebURL
	}
	return map[string]any{"number": result.Number, "url": result.HTMLURL}, nil
}

// transfer never touches Binding.WorktreePath. A task supplies only a complete
// bundle of objects, not local Git config, hooks, remotes, helpers or paths.
func transfer(ctx context.Context, dir string, g GitConnection, r Repository, token, operation string, a branchArgs) (any, error) {
	temp, err := os.MkdirTemp(dir, "git-operation-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(temp)
	if err = os.Mkdir(filepath.Join(temp, "empty"), 0700); err != nil {
		return nil, err
	}
	repository := filepath.Join(temp, "repo.git")
	run := func(auth bool, arguments ...string) ([]byte, error) {
		authorization := ""
		if auth {
			username := "x-access-token"
			if g.Provider == "gitlab" {
				username = "oauth2"
			}
			authorization = "Authorization: Basic " + base64.StdEncoding.EncodeToString([]byte(username+":"+token))
		}
		return serviceGit(ctx, temp, authorization, arguments...)
	}
	if _, err = run(false, "init", "--bare", "--template="+filepath.Join(temp, "empty"), repository); err != nil {
		return nil, err
	}
	remote := strings.TrimRight(g.Host, "/") + "/" + r.RemotePath + ".git"
	ref := "refs/heads/" + a.Branch
	bundle := filepath.Join(temp, "transfer.bundle")
	if operation == "git.fetch" {
		if _, err = run(true, "--git-dir="+repository, "fetch", "--no-tags", "--no-recurse-submodules", remote, ref+":"+ref); err != nil {
			return nil, err
		}
		if _, err = run(false, "--git-dir="+repository, "bundle", "create", bundle, ref); err != nil {
			return nil, err
		}
		stat, err := os.Stat(bundle)
		if err != nil {
			return nil, err
		}
		if stat.Size() > 64*1024*1024 {
			return nil, errors.New("bundle exceeds the 64 MiB interactive transfer limit")
		}
		data, err := os.ReadFile(bundle)
		if err != nil {
			return nil, err
		}
		return map[string]any{"branch": a.Branch, "bundleBase64": base64.StdEncoding.EncodeToString(data)}, nil
	}
	if len(a.BundleBase64) > base64.StdEncoding.EncodedLen(64*1024*1024) {
		return nil, errors.New("bundle exceeds the 64 MiB interactive transfer limit")
	}
	data, err := base64.StdEncoding.DecodeString(a.BundleBase64)
	if err != nil || len(data) == 0 {
		return nil, errors.New("a complete base64 Git bundle is required")
	}
	if err = os.WriteFile(bundle, data, 0600); err != nil {
		return nil, err
	}
	output, err := run(false, "--git-dir="+repository, "bundle", "list-heads", bundle)
	if err != nil {
		return nil, err
	}
	fields := strings.Fields(string(output))
	if len(fields) != 2 || fields[1] != ref {
		return nil, errors.New("bundle must contain only the requested branch ref")
	}
	if _, err = run(false, "--git-dir="+repository, "bundle", "verify", bundle); err != nil {
		return nil, errors.New("bundle must contain the complete branch history")
	}
	if _, err = run(false, "--git-dir="+repository, "bundle", "unbundle", bundle); err != nil {
		return nil, err
	}
	if _, err = run(false, "--git-dir="+repository, "update-ref", ref, fields[0]); err != nil {
		return nil, err
	}
	if _, err = run(false, "--git-dir="+repository, "fsck", "--strict", "--no-reflogs"); err != nil {
		return nil, err
	}
	if _, err = run(true, "--git-dir="+repository, "push", "--porcelain", "--no-verify", "--recurse-submodules=no", remote, ref+":"+ref); err != nil {
		return nil, err
	}
	return map[string]any{"branch": a.Branch, "published": true}, nil
}

// serviceGit has a closed environment and never uses the task's Git directory.
func serviceGit(ctx context.Context, directory, authorization string, arguments ...string) ([]byte, error) {
	settings := [][2]string{
		{"core.hooksPath", filepath.Join(directory, "empty")}, {"credential.helper", ""},
		{"http.followRedirects", "false"}, {"http.sslVerify", "true"}, {"protocol.file.allow", "never"},
		{"fetch.fsckObjects", "true"}, {"transfer.fsckObjects", "true"}, {"core.attributesFile", "/dev/null"},
	}
	if authorization != "" {
		settings = append(settings, [2]string{"http.extraHeader", authorization})
	}
	cmd := exec.CommandContext(ctx, "git", arguments...)
	cmd.Dir = directory
	cmd.Env = []string{"PATH=/usr/bin:/bin:/usr/local/bin:/opt/homebrew/bin", "HOME=" + directory, "XDG_CONFIG_HOME=" + directory,
		"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=/bin/false", "GIT_ALLOW_PROTOCOL=https",
		"GIT_CONFIG_COUNT=" + strconv.Itoa(len(settings))}
	for i, entry := range settings {
		cmd.Env = append(cmd.Env, "GIT_CONFIG_KEY_"+strconv.Itoa(i)+"="+entry[0], "GIT_CONFIG_VALUE_"+strconv.Itoa(i)+"="+entry[1])
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, errors.New("Git transfer failed; check credentials, branch state and network access")
	}
	return output, nil
}
