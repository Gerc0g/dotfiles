package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestSetupRejectsUnknownModeBeforeWorkstationSetup(t *testing.T) {
	cmd := NewRoot()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"setup", "--mode", "unknown", "--dry-run"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "workstation or server") {
		t.Fatalf("unexpected mode handling: %v", err)
	}
}

func TestSetupServerRequiresReleaseInputsBeforeLookingUpUser(t *testing.T) {
	cmd := NewRoot()
	cmd.SetArgs([]string{"setup", "--mode", "server", "--dry-run", "--user", "nonexistent-hq-fixture-user"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "--bundle and --private-url") {
		t.Fatalf("server installer did not validate its inputs: %v", err)
	}
}

func TestSetupDoesNotAcceptServerInputsInWorkstationMode(t *testing.T) {
	cmd := NewRoot()
	cmd.SetArgs([]string{"setup", "--bundle", "/tmp/release", "--dry-run"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "require --mode server") {
		t.Fatalf("silently ignored server input: %v", err)
	}
}
