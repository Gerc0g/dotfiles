package server

import (
	"errors"
	"testing"
)

// The stub's contract: an empty registry that refuses mutations with the
// explanation, never with a silent no-op.
func TestStubContract(t *testing.T) {
	r, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := r.List(); len(got) != 0 {
		t.Errorf("empty registry expected, got %v", got)
	}
	if err := r.Connect("box"); !errors.Is(err, ErrNotImplemented) {
		t.Errorf("Connect: want ErrNotImplemented, got %v", err)
	}
	if err := r.Remove("box"); !errors.Is(err, ErrNotImplemented) {
		t.Errorf("Remove: want ErrNotImplemented, got %v", err)
	}
}
