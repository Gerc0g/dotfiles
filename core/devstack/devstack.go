// Package devstack controls the shared local dev infrastructure and wires
// projects into it.
//
// The stack is one docker-compose file with fixed service names and ports.
// Projects connect through a managed block in their .envrc that resolves
// endpoints from DEV_STACK_HOST at `cd` time, so the same block keeps working
// when the stack moves to another box.
package devstack

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Gerc0g/dotfiles/core/workspace"
)

// HostEnv resolves endpoints; project .envrc blocks read it literally.
const HostEnv = "DEV_STACK_HOST"

// Host reports the stack host, defaulting to localhost.
func Host() string {
	if h := os.Getenv(HostEnv); h != "" {
		return h
	}
	return "localhost"
}

// ComposeFile is the single source of the stack definition.
func ComposeFile() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, "dotfiles", "services", "dev-stack", "docker-compose.yml"), nil
}

// Probe is one reachability check for doctor.
type Probe struct {
	Name string
	Port int
}

// Probes lists the services doctor pings. Not the full stack: the ones a
// project actually talks to.
var Probes = []Probe{
	{"postgres", 5432},
	{"redis", 6379},
	{"qdrant", 6333},
	{"clickhouse", 8123},
	{"minio", 9000},
	{"otel", 4317},
	{"grafana", 3030},
	{"langfuse", 3001},
	{"ollama", 11434},
}

// ProbeResult is one doctor line.
type ProbeResult struct {
	Probe
	Reachable bool
}

// ProbeHost checks every service port with a short timeout.
func ProbeHost(host string) []ProbeResult {
	out := make([]ProbeResult, 0, len(Probes))
	for _, probe := range Probes {
		addr := net.JoinHostPort(host, fmt.Sprintf("%d", probe.Port))
		conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
		if err == nil {
			conn.Close()
		}
		out = append(out, ProbeResult{Probe: probe, Reachable: err == nil})
	}
	return out
}

// Ctx is the project context a connect/doctor run works with.
type Ctx struct {
	Product string
	Repo    string
	DB      string
}

var dbUnsafe = regexp.MustCompile(`[^a-z0-9_]+`)

// DBName renders the project database name: lowercase, dashes to
// underscores, everything else dropped — the scheme `<product>__<repo>`.
func DBName(product, repo string) string {
	raw := strings.ToLower(product + "__" + repo)
	raw = strings.ReplaceAll(raw, "-", "_")
	return dbUnsafe.ReplaceAllString(raw, "")
}

// ResolveCtx derives product/repo from explicit args or the directory.
func ResolveCtx(root, dir, product, repo string) (Ctx, error) {
	if product == "" || repo == "" {
		loc, err := workspace.Locate(root, dir)
		if err != nil {
			return Ctx{}, fmt.Errorf("%w — либо передай <product> <repo>", err)
		}
		if loc.Product == "" || loc.Repo == "" {
			return Ctx{}, fmt.Errorf("каталог %s не внутри репозитория — передай <product> <repo>", dir)
		}
		product, repo = loc.Product, loc.Repo
	}
	return Ctx{Product: product, Repo: repo, DB: DBName(product, repo)}, nil
}

const blockMarker = ">>> dev-stack"

// EnvrcBlock renders the managed .envrc block. Only the DB name is
// substituted; $H and ${DEV_STACK_HOST} stay literal so direnv resolves the
// host at `cd` time.
func EnvrcBlock(db string) string {
	return fmt.Sprintf(`# >>> dev-stack (managed: dev-stack connect) >>>
H="${DEV_STACK_HOST:-localhost}"
export DATABASE_URL="postgresql://dev:dev@$H:5432/%s"
export REDIS_URL="redis://$H:6379/0"
export QDRANT_URL="http://$H:6333"
export CLICKHOUSE_URL="http://dev:dev@$H:8123"
export S3_ENDPOINT_URL="http://$H:9000"   # MinIO; creds key=dev secret=devsecret123 (set AWS_* per SDK if needed)
export OTEL_EXPORTER_OTLP_ENDPOINT="http://$H:4317"   # telemetry is PUSHED via OTLP (Prometheus does not scrape you)
export LANGFUSE_HOST="http://$H:3001"
export OLLAMA_HOST="http://$H:11434"
# <<< dev-stack <<<
`, db)
}

// EnsureEnvrcBlock appends the managed block to dir/.envrc, creating the file
// with source_up when it does not exist. Reports whether the block was added.
func EnsureEnvrcBlock(dir, db string) (added bool, envrc string, err error) {
	envrc = filepath.Join(dir, ".envrc")

	data, err := os.ReadFile(envrc)
	switch {
	case err == nil:
		if strings.Contains(string(data), blockMarker) {
			return false, envrc, nil
		}
	case os.IsNotExist(err):
		data = []byte("source_up\n")
	default:
		return false, envrc, fmt.Errorf("чтение %s: %w", envrc, err)
	}

	content := string(data) + "\n" + EnvrcBlock(db)
	if err := os.WriteFile(envrc, []byte(content), 0o644); err != nil {
		return false, envrc, fmt.Errorf("запись %s: %w", envrc, err)
	}
	return true, envrc, nil
}

// HasEnvrcBlock reports whether dir/.envrc carries the managed block.
func HasEnvrcBlock(dir string) bool {
	data, err := os.ReadFile(filepath.Join(dir, ".envrc"))
	return err == nil && strings.Contains(string(data), blockMarker)
}
