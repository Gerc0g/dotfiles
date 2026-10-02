package connections

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/Gerc0g/dotfiles/core/world"
)

func decode(raw json.RawMessage, out any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return fmt.Errorf("invalid connection arguments: %w", err)
	}
	if d.Decode(new(any)) != io.EOF {
		return errors.New("invalid trailing JSON")
	}
	return nil
}
func registered(company, repo string) error {
	if err := world.ValidateSegment(company); err != nil {
		return err
	}
	root, err := world.Root()
	if err != nil {
		return err
	}
	p, err := world.SafePath(root, company, ".company-config")
	if err != nil {
		return err
	}
	if _, err = os.Stat(p); err != nil {
		return errors.New("company is not registered")
	}
	if repo == "" {
		return nil
	}
	parts := strings.Split(repo, "/")
	if len(parts) != 3 || parts[0] != company {
		return errors.New("repository is outside the bound company")
	}
	for _, part := range parts {
		if err = world.ValidateSegment(part); err != nil {
			return err
		}
	}
	for i, marker := range []string{".company-config", ".product-config", ".git"} {
		p, err = world.SafePath(root, append(append([]string{}, parts[:i+1]...), marker)...)
		if err != nil {
			return err
		}
		if _, err = os.Stat(p); err != nil {
			return errors.New("repository is not registered")
		}
	}
	return nil
}
func storage(company string) (string, error) {
	if err := registered(company, ""); err != nil {
		return "", err
	}
	root := os.Getenv("HQ_DATA_ROOT")
	if root == "" {
		return "", errors.New("HQ_DATA_ROOT must point to private service storage")
	}
	dir, err := world.SafePath(root, "connections", company)
	if err != nil {
		return "", err
	}
	credentialDir, err := world.SafePath(dir, "credentials")
	if err != nil {
		return "", err
	}
	for _, d := range []string{filepath.Dir(dir), dir, credentialDir} {
		if err = os.MkdirAll(d, 0700); err != nil {
			return "", err
		}
		if err = os.Chmod(d, 0700); err != nil {
			return "", err
		}
	}
	return dir, nil
}
func atomicJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".write-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(append(data, '\n'))
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(f.Name(), path)
}
func availableRepositories(company string) []string {
	refs := []string{}
	tree, err := world.ScanDefault()
	if err != nil {
		return refs
	}
	for _, r := range tree.Repos(company, "") {
		if registered(company, r.Ref()) == nil {
			refs = append(refs, r.Ref())
		}
	}
	return refs
}
func load(dir, company string) (Config, error) {
	c := Config{CompanyID: company, Git: []GitConnection{}, Repositories: []Repository{}, Environments: []Environment{}, History: []Event{}}
	configPath, err := world.SafePath(dir, "config.json")
	if err != nil {
		return c, err
	}
	data, err := os.ReadFile(configPath)
	if os.IsNotExist(err) {
		c.AvailableRepositories = availableRepositories(company)
		return c, nil
	}
	if err != nil {
		return c, err
	}
	err = decode(data, &c)
	c.AvailableRepositories = availableRepositories(company)
	return c, err
}
func locked(company string, fn func(string, Config) (any, error)) (any, error) {
	dir, err := storage(company)
	if err != nil {
		return nil, err
	}
	lockPath, err := world.SafePath(dir, ".lock")
	if err != nil {
		return nil, err
	}
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return nil, err
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	c, err := load(dir, company)
	if err != nil {
		return nil, err
	}
	return fn(dir, c)
}
func snapshot(company string) (Config, error) {
	v, err := locked(company, func(_ string, c Config) (any, error) { return c, nil })
	if err != nil {
		return Config{}, err
	}
	return v.(Config), nil
}
func unverified(ref string) Verification {
	state := "unverified"
	if ref == "" {
		state = "disconnected"
	}
	return Verification{State: state, Read: "unverified", BranchPush: "unverified", PullRequests: "unverified", CredentialPermissions: "unverified"}
}
func save(dir string, c Config, action, id string) (Config, error) {
	c.AvailableRepositories = availableRepositories(c.CompanyID)
	c.Revision++
	c.History = append(c.History, Event{At: time.Now().UTC().Format(time.RFC3339), Action: action, ConnectionID: id})
	return c, atomicJSON(filepath.Join(dir, "config.json"), c)
}
func validURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" {
		return nil, errors.New("connection URL must be HTTPS without credentials, query or fragment")
	}
	return u, nil
}
func validate(c Config) error {
	ids := map[string]bool{}
	for _, g := range c.Git {
		if err := world.ValidateSegment(g.ID); err != nil {
			return err
		}
		if ids[g.ID] {
			return errors.New("duplicate connection ID")
		}
		ids[g.ID] = true
		if g.Provider != "github" && g.Provider != "gitlab" {
			return errors.New("supported Git providers: github, gitlab")
		}
		u, err := validURL(g.Host)
		if err != nil {
			return err
		}
		if u.Path != "" && u.Path != "/" {
			return errors.New("Git host must not contain a path")
		}
		if g.Namespace == "" || !remotePathPattern.MatchString(g.Namespace) || strings.Contains(g.Namespace, "..") {
			return errors.New("invalid namespace")
		}
		if err = branchName(g.Policy.TargetBranch); err != nil {
			return fmt.Errorf("target branch: %w", err)
		}
		for _, prefix := range g.Policy.BranchPrefixes {
			if prefix == "" || strings.HasPrefix(prefix, "-") || strings.ContainsAny(prefix, "~^:?*[\\\x00\n\r ") || strings.Contains(prefix, "..") {
				return errors.New("invalid branch prefix")
			}
		}
		if g.Policy.BranchPush && len(g.Policy.BranchPrefixes) == 0 {
			return errors.New("branch publication requires an explicit allowed prefix")
		}
		for _, branch := range g.Policy.ProtectedBranches {
			if err = branchName(branch); err != nil {
				return err
			}
		}
	}
	repos := map[string]bool{}
	for _, r := range c.Repositories {
		if err := registered(c.CompanyID, r.RepoID); err != nil {
			return err
		}
		if repos[r.RepoID] {
			return errors.New("duplicate repository mapping")
		}
		repos[r.RepoID] = true
		var found *GitConnection
		for i := range c.Git {
			if c.Git[i].ID == r.ConnectionID {
				found = &c.Git[i]
			}
		}
		if found == nil {
			return errors.New("repository connection is not in this company")
		}
		if !remotePathPattern.MatchString(r.RemotePath) || strings.Contains(r.RemotePath, "..") || !strings.HasPrefix(r.RemotePath, found.Namespace+"/") {
			return errors.New("repository path is outside the configured namespace")
		}
	}
	for _, e := range c.Environments {
		if err := world.ValidateSegment(e.ID); err != nil {
			return err
		}
		if ids[e.ID] {
			return errors.New("duplicate connection ID")
		}
		ids[e.ID] = true
		if e.Tier != "dev" && e.Tier != "stage" && e.Tier != "prod" {
			return errors.New("environment tier must be dev, stage or prod")
		}
		if e.Kind != "http" {
			return errors.New("only narrowly scoped HTTP read connectors are supported")
		}
		if _, err := validURL(e.BaseURL); err != nil {
			return err
		}
		reqs := map[string]bool{}
		for _, r := range e.Requests {
			if err := world.ValidateSegment(r.ID); err != nil {
				return err
			}
			if reqs[r.ID] {
				return errors.New("duplicate request ID")
			}
			reqs[r.ID] = true
			if r.Method != "GET" && r.Method != "HEAD" {
				return errors.New("environment connectors permit GET and HEAD only")
			}
			if !strings.HasPrefix(r.Path, "/") || strings.HasPrefix(r.Path, "//") || strings.ContainsAny(r.Path, "?#%\\\r\n") || strings.Contains(r.Path, "..") {
				return errors.New("request path must be a fixed absolute path on the configured host")
			}
			for key, pattern := range r.Query {
				if !queryKeyPattern.MatchString(key) || len(pattern) > 2048 {
					return errors.New("invalid query constraint")
				}
				if _, err := regexp.Compile("^(?:" + pattern + ")$"); err != nil {
					return errors.New("invalid query constraint")
				}
			}
		}
	}
	return nil
}

var remotePathPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+(?:[./-]?[A-Za-z0-9_-]+)*$`)
var queryKeyPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]*$`)

func branchName(branch string) error {
	if branch == "" || branch == "@" || strings.HasPrefix(branch, "-") || strings.HasPrefix(branch, "/") || strings.HasSuffix(branch, "/") || strings.HasSuffix(branch, ".") || strings.Contains(branch, "..") || strings.Contains(branch, "@{") || strings.Contains(branch, "//") || strings.ContainsAny(branch, "~^:?*[\\\x00\n\r\t ") {
		return errors.New("invalid branch name")
	}
	for _, part := range strings.Split(branch, "/") {
		if strings.HasPrefix(part, ".") || strings.HasSuffix(part, ".lock") {
			return errors.New("invalid branch name")
		}
	}
	return nil
}

type ownerArgs struct {
	CompanyID    string  `json:"companyId"`
	Revision     uint64  `json:"revision"`
	Config       *Config `json:"config,omitempty"`
	ConnectionID string  `json:"connectionId,omitempty"`
	Token        string  `json:"token,omitempty"`
	ExpiresAt    string  `json:"expiresAt,omitempty"`
}
type credential struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expiresAt,omitempty"`
}

func readCredential(dir, ref string) (string, error) {
	if ref == "" {
		return "", errors.New("connection is disconnected: add a credential")
	}
	if err := world.ValidateSegment(ref); err != nil {
		return "", err
	}
	credentialPath, err := world.SafePath(dir, "credentials", ref+".json")
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(credentialPath)
	if err != nil {
		return "", errors.New("credential unavailable")
	}
	var c credential
	if decode(data, &c) != nil {
		return "", errors.New("credential unavailable")
	}
	if c.ExpiresAt != "" {
		expires, err := time.Parse(time.RFC3339, c.ExpiresAt)
		if err != nil || !time.Now().Before(expires) {
			return "", errors.New("credential expired")
		}
	}
	return c.Token, nil
}
func Dispatch(operation string, args json.RawMessage) (any, error) {
	var a ownerArgs
	if err := decode(args, &a); err != nil {
		return nil, err
	}
	return locked(a.CompanyID, func(dir string, c Config) (any, error) {
		if operation == "connections.get" {
			return c, nil
		}
		if a.Revision != c.Revision {
			return nil, errors.New("connection revision conflict: reload before saving")
		}
		switch operation {
		case "connections.save":
			if a.Config == nil || a.Config.CompanyID != a.CompanyID {
				return nil, errors.New("company mismatch")
			}
			next := *a.Config
			if err := validate(next); err != nil {
				return nil, err
			}
			for i := range next.Git {
				g := &next.Git[i]
				ref, expiry := "", ""
				for _, old := range c.Git {
					if old.ID == g.ID {
						if old.CredentialRef != "" && (old.Host != g.Host || old.Provider != g.Provider) {
							return nil, errors.New("remove the credential before changing its hosting destination")
						}
						ref, expiry = old.CredentialRef, old.ExpiresAt
					}
				}
				if g.CredentialRef != ref {
					return nil, errors.New("credential references are service-managed")
				}
				g.ExpiresAt = expiry
				g.Verification = unverified(ref)
			}
			for i := range next.Environments {
				e := &next.Environments[i]
				ref, expiry := "", ""
				for _, old := range c.Environments {
					if old.ID == e.ID {
						if old.CredentialRef != "" && old.BaseURL != e.BaseURL {
							return nil, errors.New("remove the credential before changing its application destination")
						}
						ref, expiry = old.CredentialRef, old.ExpiresAt
					}
				}
				if e.CredentialRef != ref {
					return nil, errors.New("credential references are service-managed")
				}
				e.ExpiresAt = expiry
				e.Verification = unverified(ref)
			}
			next.Revision = c.Revision
			next.History = c.History
			result, err := save(dir, next, "configuration.saved", "")
			if err != nil {
				return nil, err
			}
			kept := map[string]bool{}
			for _, g := range next.Git {
				kept[g.CredentialRef] = true
			}
			for _, e := range next.Environments {
				kept[e.CredentialRef] = true
			}
			for _, g := range c.Git {
				if g.CredentialRef != "" && !kept[g.CredentialRef] {
					if err = removeCredential(dir, g.CredentialRef); err != nil {
						return nil, err
					}
				}
			}
			for _, e := range c.Environments {
				if e.CredentialRef != "" && !kept[e.CredentialRef] {
					if err = removeCredential(dir, e.CredentialRef); err != nil {
						return nil, err
					}
				}
			}
			return result, nil
		case "connections.credential.put", "connections.credential.revoke":
			var ref, expiry *string
			var verification *Verification
			for i := range c.Git {
				if c.Git[i].ID == a.ConnectionID {
					ref = &c.Git[i].CredentialRef
					expiry = &c.Git[i].ExpiresAt
					verification = &c.Git[i].Verification
				}
			}
			for i := range c.Environments {
				if c.Environments[i].ID == a.ConnectionID {
					ref = &c.Environments[i].CredentialRef
					expiry = &c.Environments[i].ExpiresAt
					verification = &c.Environments[i].Verification
				}
			}
			if ref == nil {
				return nil, errors.New("connection does not exist")
			}
			old := *ref
			if operation == "connections.credential.put" {
				if len(a.Token) == 0 || strings.ContainsAny(a.Token, "\r\n\x00") {
					return nil, errors.New("credential must be a nonempty single-line token")
				}
				if a.ExpiresAt != "" {
					expires, err := time.Parse(time.RFC3339, a.ExpiresAt)
					if err != nil || !time.Now().Before(expires) {
						return nil, errors.New("credential expiry must be a future RFC3339 timestamp")
					}
				}
				var id [16]byte
				if _, err := rand.Read(id[:]); err != nil {
					return nil, err
				}
				*ref = hex.EncodeToString(id[:])
				*expiry = a.ExpiresAt
				if err := atomicJSON(filepath.Join(dir, "credentials", *ref+".json"), credential{a.Token, a.ExpiresAt}); err != nil {
					return nil, err
				}
			} else {
				*ref = ""
				*expiry = ""
			}
			*verification = unverified(*ref)
			result, err := save(dir, c, strings.TrimPrefix(operation, "connections."), a.ConnectionID)
			if err == nil && old != "" {
				err = removeCredential(dir, old)
			}
			return result, err
		case "connections.verify", "connections.environment.verify":
			return verify(dir, c, a.ConnectionID, operation == "connections.environment.verify")
		default:
			return nil, errors.New("unsupported connection operation")
		}
	})
}

func removeCredential(dir, ref string) error {
	filename, err := world.SafePath(dir, "credentials", ref+".json")
	if err != nil {
		return err
	}
	if err = os.Remove(filename); err != nil && !os.IsNotExist(err) {
		return errors.New("configuration saved but the previous local credential could not be deleted")
	}
	return nil
}
