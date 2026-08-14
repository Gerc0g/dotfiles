package secret

import "testing"

// The stub's contract: the error explains the state instead of a silent no-op.
func TestStubContract(t *testing.T) {
	if ErrNotImplemented == nil || ErrNotImplemented.Error() == "" {
		t.Fatal("stub must carry its explanation")
	}
}
