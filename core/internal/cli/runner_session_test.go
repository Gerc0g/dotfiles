package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunnerRejectsUnsafeSessionOptionsBeforeStartingRuntime(t *testing.T) {
	command := newRunnerCmd()
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetErr(&output)
	command.SetArgs([]string{"app-server", "--directory", "/not-a-workspace", "--session-options", `{"v":1,"auth":{"token":"not-allowed"}}`})
	err := command.Execute()
	if err == nil || !strings.Contains(err.Error(), "HQ_SESSION_OPTIONS_INVALID") {
		t.Fatalf("expected typed override rejection, got %v", err)
	}
}
