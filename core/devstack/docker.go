package devstack

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// DockerRunning reports whether the docker daemon answers.
func DockerRunning() bool {
	return exec.Command("docker", "ps").Run() == nil
}

// Compose runs docker compose against the stack definition with the caller's
// stdio, so logs -f and up progress stream normally.
func Compose(args ...string) error {
	file, err := ComposeFile()
	if err != nil {
		return err
	}
	cmd := exec.Command("docker", append([]string{"compose", "-f", file}, args...)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker compose %s: %w", strings.Join(args, " "), err)
	}
	return nil
}

// Interactive execs into a stack container with a TTY (psql, redis-cli, ...).
func Interactive(container string, args ...string) error {
	full := append([]string{"exec", "-it", container}, args...)
	cmd := exec.Command("docker", full...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// psqlIn runs SQL from stdin on dev-postgres. Piping via stdin avoids every
// quoting pitfall of passing SQL as an argument. -d postgres: the maintenance
// DB (psql -U dev alone would target a database literally named "dev").
func psqlIn(sql string, extra ...string) (string, error) {
	args := append([]string{"exec", "-i", "dev-postgres", "psql", "-U", "dev", "-d", "postgres"}, extra...)
	cmd := exec.Command("docker", args...)
	cmd.Stdin = strings.NewReader(sql)

	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("psql: %w: %s", err, strings.TrimSpace(errOut.String()))
	}
	return strings.TrimSpace(out.String()), nil
}

// DBExists probes for a database on dev-postgres.
func DBExists(name string) bool {
	out, err := psqlIn(fmt.Sprintf("SELECT 1 FROM pg_database WHERE datname='%s';", name), "-tA")
	return err == nil && strings.Contains(out, "1")
}

// CreateDB creates a database; it fails if the database exists.
func CreateDB(name string) error {
	_, err := psqlIn(fmt.Sprintf(`CREATE DATABASE "%s";`, name))
	return err
}

// DropDB drops a database.
func DropDB(name string) error {
	_, err := psqlIn(fmt.Sprintf(`DROP DATABASE "%s";`, name))
	return err
}

// EnsureDB is the idempotent CreateDB.
func EnsureDB(name string, out io.Writer) error {
	if DBExists(name) {
		fmt.Fprintf(out, "✓ БД %q уже есть\n", name)
		return nil
	}
	if err := CreateDB(name); err != nil {
		return fmt.Errorf("создание БД %q: %w", name, err)
	}
	fmt.Fprintf(out, "✓ БД %q создана\n", name)
	return nil
}
