package secret

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// runOp shells to the 1Password CLI. Secrets travel through op itself; this
// package never logs values.
func runOp(args ...string) (string, error) {
	if _, err := exec.LookPath("op"); err != nil {
		return "", fmt.Errorf("op CLI не найден — установи 1Password CLI и `secret signin`")
	}

	cmd := exec.Command("op", args...)
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("op %s: %w: %s",
			strings.Join(redactArgs(args), " "), err, strings.TrimSpace(errOut.String()))
	}
	return strings.TrimSpace(out.String()), nil
}

// redactArgs hides `key=value` assignments so a failing op call never prints
// the secret value it was given.
func redactArgs(args []string) []string {
	out := make([]string, len(args))
	for i, arg := range args {
		if key, _, found := strings.Cut(arg, "="); found && !strings.HasPrefix(arg, "--") {
			out[i] = key + "=***"
			continue
		}
		out[i] = arg
	}
	return out
}

func opOK(args ...string) bool {
	_, err := runOp(args...)
	return err == nil
}

// opReadField reads one item field, the way the cache always fetched values.
func opReadField(vault, item, field string) (string, error) {
	return runOp("item", "get", item, "--vault", vault, "--fields", "label="+field, "--reveal")
}

// OpPassthrough exposes raw op calls for the list/get passthrough commands.
func OpPassthrough(args ...string) (string, error) {
	return runOp(args...)
}

// EnsureVault creates the vault if possible; an existing vault is fine.
func EnsureVault(vault string) {
	_ = exec.Command("op", "vault", "create", vault).Run()
}

// ItemExists probes for an item in a vault.
func ItemExists(vault, item string) bool {
	return opOK("item", "get", item, "--vault", vault)
}

// CreateItem stores a new API-credential item.
func CreateItem(vault, item, value string) error {
	_, err := runOp("item", "create", "--category=apicredential",
		"--vault="+vault, "--title="+item, "credential="+value)
	return err
}

// EditItem updates the credential of an existing item.
func EditItem(vault, item, value string) error {
	_, err := runOp("item", "edit", item, "--vault="+vault, "credential="+value)
	return err
}
