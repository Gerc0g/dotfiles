package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Gerc0g/dotfiles/core/entity"
	"github.com/charmbracelet/fang"
	"github.com/spf13/cobra"
)

func TestMachineReadableErrorRetainsConflictMarker(t *testing.T) {
	var stderr bytes.Buffer
	root := &cobra.Command{Use: "hq", RunE: func(*cobra.Command, []string) error { return errors.New(entity.RevisionConflict + ": reload the card") }}
	root.SetArgs([]string{})
	root.SetOut(new(bytes.Buffer))
	root.SetErr(&stderr)
	err := fang.Execute(context.Background(), root, fang.WithErrorHandler(renderError))
	if err == nil || !strings.Contains(stderr.String(), entity.RevisionConflict) {
		t.Fatalf("machine conflict marker lost: %q (%v)", stderr.String(), err)
	}
}
