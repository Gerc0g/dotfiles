// Package connections owns company connection policy and secret-bearing operations.
// Only the owner control plane may call Dispatch. Agents use Execute with a
// binding supplied by the trusted runner, never arguments from a session.
package connections

type Config struct {
	AvailableRepositories []string        `json:"availableRepositories"`
	CompanyID             string          `json:"companyId"`
	Revision              uint64          `json:"revision"`
	Git                   []GitConnection `json:"git"`
	Repositories          []Repository    `json:"repositories"`
	Environments          []Environment   `json:"environments"`
	History               []Event         `json:"history"`
}
type Event struct {
	At           string `json:"at"`
	Action       string `json:"action"`
	ConnectionID string `json:"connectionId,omitempty"`
}
type GitConnection struct {
	ID            string       `json:"id"`
	Provider      string       `json:"provider"`
	Host          string       `json:"host"`
	Namespace     string       `json:"namespace"`
	AuthorName    string       `json:"authorName"`
	AuthorEmail   string       `json:"authorEmail"`
	CredentialRef string       `json:"credentialRef"`
	ExpiresAt     string       `json:"expiresAt,omitempty"`
	Policy        Policy       `json:"policy"`
	Verification  Verification `json:"verification"`
}
type Policy struct {
	Read                bool     `json:"read"`
	BranchPush          bool     `json:"branchPush"`
	PullRequests        bool     `json:"pullRequests"`
	BranchPrefixes      []string `json:"branchPrefixes"`
	ProtectedBranches   []string `json:"protectedBranches"`
	TargetBranch        string   `json:"targetBranch"`
	Draft               bool     `json:"draft"`
	DescriptionLanguage string   `json:"descriptionLanguage"`
	DescriptionTemplate string   `json:"descriptionTemplate"`
	RequiredChecks      []string `json:"requiredChecks"`
}
type Repository struct {
	RepoID       string `json:"repoId"`
	ConnectionID string `json:"connectionId"`
	RemotePath   string `json:"remotePath"`
}
type Verification struct {
	State                 string `json:"state"`
	Read                  string `json:"read"`
	BranchPush            string `json:"branchPush"`
	PullRequests          string `json:"pullRequests"`
	CredentialPermissions string `json:"credentialPermissions"`
	CheckedAt             string `json:"checkedAt,omitempty"`
	Detail                string `json:"detail,omitempty"`
}
type Environment struct {
	ID            string       `json:"id"`
	Tier          string       `json:"tier"`
	Kind          string       `json:"kind"`
	BaseURL       string       `json:"baseUrl"`
	CredentialRef string       `json:"credentialRef"`
	ExpiresAt     string       `json:"expiresAt,omitempty"`
	Requests      []Endpoint   `json:"requests"`
	Verification  Verification `json:"verification"`
}
type Endpoint struct {
	ID     string            `json:"id"`
	Method string            `json:"method"`
	Path   string            `json:"path"`
	Query  map[string]string `json:"query"`
	Verify bool              `json:"verify"`
}

// Binding is constructed by the trusted runtime from its registered task.
// WorktreePath is deliberately never opened by this secret-bearing package.
type Binding struct {
	CompanyID    string
	RepoID       string
	WorktreePath string
}
