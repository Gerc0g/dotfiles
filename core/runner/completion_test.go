package runner

import (
	"context"
	"errors"
	"os/exec"
	"testing"
)

func TestTaskProcessExitClassifiesUserCancellation(t *testing.T) {
	killed := errors.New("signal: killed")
	for _, tc := range []struct {
		name       string
		contextErr error
		processErr error
		state      string
		wantErr    error
	}{
		{"user stop", context.Canceled, killed, "interrupted", nil},
		{"stop after process exit", context.Canceled, nil, "interrupted", nil},
		{"normal exit", nil, nil, "completed", nil},
		{"unexpected kill", nil, killed, "failed", killed},
		{"deadline is not user stop", context.DeadlineExceeded, killed, "failed", killed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state, err := taskProcessExit(tc.contextErr, tc.processErr)
			if state != tc.state || !errors.Is(err, tc.wantErr) {
				t.Fatalf("state=%q err=%v; want state=%q err=%v", state, err, tc.state, tc.wantErr)
			}
		})
	}
}

func TestTaskProcessExitRetainsResourceFailure(t *testing.T) {
	err := exec.Command("sh", "-c", "exit 137").Run()
	state, gotErr := taskProcessExit(nil, err)
	if state != "resource-limit" || gotErr == nil {
		t.Fatalf("state=%q err=%v", state, gotErr)
	}
}
