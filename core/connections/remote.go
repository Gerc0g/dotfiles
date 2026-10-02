package connections

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// HTTP responses are bounded because connector exports and Git metadata traverse
// the interactive control RPC, not a streaming download channel.
const maxResponseBytes = 2 * 1024 * 1024

var httpTransport http.RoundTripper = &http.Transport{Proxy: nil}

func request(ctx context.Context, method, target, token string, body any) ([]byte, int, error) {
	var data []byte
	var err error
	if body != nil {
		data, err = json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, target, bytes.NewReader(data))
	if err != nil {
		return nil, 0, errors.New("invalid request URL")
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	client := &http.Client{Transport: httpTransport, Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, err := client.Do(req)
	if err != nil {
		return nil, 0, errors.New("connection request failed; check host and network access")
	}
	defer res.Body.Close()
	data, err = io.ReadAll(io.LimitReader(res.Body, maxResponseBytes+1))
	if err != nil {
		return nil, res.StatusCode, errors.New("connection response could not be read")
	}
	if len(data) > maxResponseBytes {
		return nil, res.StatusCode, errors.New("response exceeds the 2 MiB interactive export limit")
	}
	if token != "" {
		data = redactResponse(data, token)
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, res.StatusCode, fmt.Errorf("hosting request returned HTTP %d", res.StatusCode)
	}
	return data, res.StatusCode, nil
}

func redactResponse(data []byte, token string) []byte {
	replace := func(value string) string {
		value = strings.ReplaceAll(value, token, "[redacted]")
		return strings.ReplaceAll(value, base64.StdEncoding.EncodeToString([]byte(token)), "[redacted]")
	}
	// JSON escape sequences must be decoded before scanning values or keys.
	// Otherwise an ordinary escaped slash or unicode character can reintroduce
	// the credential when the task parses the connector's exported body.
	var value any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if decoder.Decode(&value) == nil && decoder.Decode(new(any)) == io.EOF {
		var walk func(any) any
		walk = func(value any) any {
			switch v := value.(type) {
			case string:
				return replace(v)
			case []any:
				for i := range v {
					v[i] = walk(v[i])
				}
				return v
			case map[string]any:
				out := make(map[string]any, len(v))
				for key, item := range v {
					out[replace(key)] = walk(item)
				}
				return out
			default:
				return value
			}
		}
		if clean, err := json.Marshal(walk(value)); err == nil {
			return clean
		}
	}
	return []byte(replace(string(data)))
}
func apiBase(g GitConnection) string {
	if g.Provider == "github" {
		u, _ := url.Parse(g.Host)
		if strings.EqualFold(u.Host, "github.com") {
			return "https://api.github.com"
		}
		return strings.TrimRight(g.Host, "/") + "/api/v3"
	}
	return strings.TrimRight(g.Host, "/") + "/api/v4"
}
func repoURL(g GitConnection, r Repository) string {
	if g.Provider == "github" {
		return apiBase(g) + "/repos/" + r.RemotePath
	}
	return apiBase(g) + "/projects/" + url.PathEscape(r.RemotePath)
}

type remoteRepo struct {
	DefaultBranch string `json:"default_branch"`
	Archived      bool   `json:"archived"`
}

func getRepo(ctx context.Context, g GitConnection, r Repository, token string) (remoteRepo, error) {
	var repo remoteRepo
	data, _, err := request(ctx, "GET", repoURL(g, r), token, nil)
	if err != nil {
		return repo, err
	}
	if err = json.Unmarshal(data, &repo); err != nil || repo.DefaultBranch == "" {
		return repo, errors.New("hosting response did not identify the default branch")
	}
	return repo, nil
}
func allowedBranch(g GitConnection, branch, defaultBranch string) error {
	if err := branchName(branch); err != nil {
		return err
	}
	if branch == defaultBranch || branch == g.Policy.TargetBranch {
		return errors.New("publishing to the default or target branch is forbidden")
	}
	for _, protected := range g.Policy.ProtectedBranches {
		if branch == protected {
			return errors.New("protected branch is forbidden")
		}
	}
	for _, prefix := range g.Policy.BranchPrefixes {
		if strings.HasPrefix(branch, prefix) && len(branch) > len(prefix) {
			return nil
		}
	}
	return errors.New("branch is outside the allowed prefixes")
}
func protectedRemote(ctx context.Context, g GitConnection, r Repository, branch, token string) error {
	endpoint := repoURL(g, r) + "/branches/" + url.PathEscape(branch)
	if g.Provider == "gitlab" {
		endpoint = repoURL(g, r) + "/repository/branches/" + url.PathEscape(branch)
	}
	data, status, err := request(ctx, "GET", endpoint, token, nil)
	if err != nil && status != 404 {
		return err
	}
	if err == nil {
		var b struct {
			Protected bool `json:"protected"`
		}
		if json.Unmarshal(data, &b) != nil {
			return errors.New("invalid branch response")
		}
		if b.Protected {
			return errors.New("hosting reports a protected branch")
		}
	}
	if g.Provider == "github" {
		data, _, err = request(ctx, "GET", repoURL(g, r)+"/rules/branches/"+url.PathEscape(branch), token, nil)
		if err != nil {
			return fmt.Errorf("cannot verify branch rules: %w", err)
		}
		var rules []json.RawMessage
		if json.Unmarshal(data, &rules) != nil {
			return errors.New("invalid branch rules response")
		}
		if len(rules) > 0 {
			return errors.New("hosting branch rules apply; publishing through HQ is denied")
		}
	} else {
		for page := 1; ; page++ {
			data, _, err = request(ctx, "GET", repoURL(g, r)+fmt.Sprintf("/protected_branches?per_page=100&page=%d", page), token, nil)
			if err != nil {
				return fmt.Errorf("cannot verify protected branches: %w", err)
			}
			var rules []struct {
				Name string `json:"name"`
			}
			if json.Unmarshal(data, &rules) != nil {
				return errors.New("invalid protected branch response")
			}
			for _, rule := range rules {
				pattern := "^" + strings.ReplaceAll(regexp.QuoteMeta(rule.Name), `\*`, ".*") + "$"
				matched, _ := regexp.MatchString(pattern, branch)
				if matched {
					return errors.New("hosting protected branch rule applies")
				}
			}
			if len(rules) < 100 {
				break
			}
		}
	}
	return nil
}
func verify(dir string, c Config, id string, environment bool) (Config, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	v := unverified("")
	v.CheckedAt = time.Now().UTC().Format(time.RFC3339)
	if environment {
		for i := range c.Environments {
			e := &c.Environments[i]
			if e.ID != id {
				continue
			}
			token, err := readCredential(dir, e.CredentialRef)
			v = unverified(e.CredentialRef)
			v.CheckedAt = time.Now().UTC().Format(time.RFC3339)
			if err == nil {
				var check *Endpoint
				for j := range e.Requests {
					if e.Requests[j].Verify {
						check = &e.Requests[j]
						break
					}
				}
				if check == nil {
					err = errors.New("choose a read request for verification")
				} else {
					_, err = environmentRequest(ctx, *e, check.ID, map[string]string{}, token)
				}
			}
			if err != nil {
				v.Detail = err.Error()
			} else {
				v.State = "read_verified"
				v.Read = "verified"
				v.Detail = "Request succeeded. Credential backing permissions remain unverified."
			}
			e.Verification = v
			return save(dir, c, "environment.verified", id)
		}
	} else {
		for i := range c.Git {
			g := &c.Git[i]
			if g.ID != id {
				continue
			}
			v = unverified(g.CredentialRef)
			v.CheckedAt = time.Now().UTC().Format(time.RFC3339)
			token, err := readCredential(dir, g.CredentialRef)
			if err == nil {
				count := 0
				for _, r := range c.Repositories {
					if r.ConnectionID != id {
						continue
					}
					count++
					remote, readErr := getRepo(ctx, *g, r, token)
					if readErr != nil {
						err = readErr
						break
					}
					endpoint := repoURL(*g, r) + "/git/trees/" + url.PathEscape(remote.DefaultBranch)
					if g.Provider == "gitlab" {
						endpoint = repoURL(*g, r) + "/repository/tree?per_page=1&ref=" + url.QueryEscape(remote.DefaultBranch)
					}
					if _, _, err = request(ctx, "GET", endpoint, token, nil); err != nil {
						break
					}
				}
				if count == 0 {
					err = errors.New("map at least one registered repository before verification")
				}
			}
			if err != nil {
				v.Detail = err.Error()
			} else {
				v.State = "read_verified"
				v.Read = "verified"
				v.Detail = "Repository code is readable. Branch publication and PR/MR permissions are unverified; no test push was made."
			}
			g.Verification = v
			return save(dir, c, "git.verified", id)
		}
	}
	return c, errors.New("connection does not exist")
}
func environmentRequest(ctx context.Context, e Environment, requestID string, query map[string]string, token string) (any, error) {
	// A fixed GET/HEAD allowlist does not establish infrastructure permissions.
	// Generic HTTP has no trusted provider-specific read-only proof, so even a
	// saved or caller-claimed verification flag cannot enable production access.
	if e.Tier == "prod" {
		return nil, errors.New("HQ_PROD_READ_ONLY_UNVERIFIED: this connector cannot verify infrastructure read-only credentials")
	}
	var endpoint *Endpoint
	for i := range e.Requests {
		if e.Requests[i].ID == requestID {
			endpoint = &e.Requests[i]
		}
	}
	if endpoint == nil {
		return nil, errors.New("request is not in the environment allowlist")
	}
	if endpoint.Method != "GET" && endpoint.Method != "HEAD" {
		return nil, errors.New("environment writes are forbidden")
	}
	u, err := validURL(e.BaseURL)
	if err != nil {
		return nil, err
	}
	u.Path = strings.TrimRight(u.Path, "/") + endpoint.Path
	values := url.Values{}
	for k, v := range query {
		pattern, ok := endpoint.Query[k]
		if !ok {
			return nil, errors.New("query parameter is not allowed")
		}
		re, err := regexp.Compile("^(?:" + pattern + ")$")
		if err != nil || !re.MatchString(v) {
			return nil, errors.New("query value violates the configured constraint")
		}
		values.Set(k, v)
	}
	u.RawQuery = values.Encode()
	if len(u.RawQuery) > 2048 {
		return nil, errors.New("query exceeds the 2048-byte connector limit")
	}
	data, status, err := request(ctx, endpoint.Method, u.String(), token, nil)
	if err != nil {
		return nil, err
	}
	return map[string]any{"status": status, "body": string(data), "credentialPermissions": "unverified"}, nil
}
func checkRequired(ctx context.Context, g GitConnection, r Repository, branch, token string) error {
	if len(g.Policy.RequiredChecks) == 0 {
		return nil
	}
	passed := map[string]bool{}
	if g.Provider == "gitlab" {
		data, _, err := request(ctx, "GET", repoURL(g, r)+"/repository/branches/"+url.PathEscape(branch), token, nil)
		if err != nil {
			return err
		}
		var head struct {
			Commit struct {
				ID string `json:"id"`
			} `json:"commit"`
		}
		if json.Unmarshal(data, &head) != nil || !objectIDPattern.MatchString(head.Commit.ID) {
			return errors.New("hosting did not identify the branch commit")
		}
		for page := 1; ; page++ {
			data, _, err = request(ctx, "GET", repoURL(g, r)+"/repository/commits/"+head.Commit.ID+"/statuses?all=false&ref="+url.QueryEscape(branch)+fmt.Sprintf("&per_page=100&page=%d", page), token, nil)
			if err != nil {
				return err
			}
			var statuses []struct {
				Name   string `json:"name"`
				Status string `json:"status"`
				SHA    string `json:"sha"`
				Ref    string `json:"ref"`
			}
			if json.Unmarshal(data, &statuses) != nil {
				return errors.New("invalid commit statuses response")
			}
			for _, status := range statuses {
				if status.SHA == head.Commit.ID && status.Ref == branch {
					if previous, exists := passed[status.Name]; exists {
						passed[status.Name] = previous && status.Status == "success"
					} else {
						passed[status.Name] = status.Status == "success"
					}
				}
			}
			if len(statuses) < 100 {
				break
			}
		}
		return requirePassed(g.Policy.RequiredChecks, passed)
	}
	for page := 1; ; page++ {
		data, _, err := request(ctx, "GET", repoURL(g, r)+"/commits/"+url.PathEscape(branch)+fmt.Sprintf("/check-runs?per_page=100&page=%d", page), token, nil)
		if err != nil {
			return err
		}
		var result struct {
			Runs []struct {
				Name       string `json:"name"`
				Conclusion string `json:"conclusion"`
			} `json:"check_runs"`
		}
		if json.Unmarshal(data, &result) != nil {
			return errors.New("invalid checks response")
		}
		for _, run := range result.Runs {
			passed[run.Name] = run.Conclusion == "success"
		}
		if len(result.Runs) < 100 {
			break
		}
	}
	data, _, err := request(ctx, "GET", repoURL(g, r)+"/commits/"+url.PathEscape(branch)+"/status", token, nil)
	if err != nil {
		return err
	}
	var result struct {
		Statuses []struct {
			Context string `json:"context"`
			State   string `json:"state"`
		} `json:"statuses"`
	}
	if json.Unmarshal(data, &result) != nil {
		return errors.New("invalid statuses response")
	}
	for _, s := range result.Statuses {
		passed[s.Context] = s.State == "success"
	}
	return requirePassed(g.Policy.RequiredChecks, passed)
}

var objectIDPattern = regexp.MustCompile(`^(?:[a-f0-9]{40}|[a-f0-9]{64})$`)

func requirePassed(required []string, passed map[string]bool) error {
	for _, name := range required {
		if !passed[name] {
			return fmt.Errorf("required check is not successful: %s", name)
		}
	}
	return nil
}
