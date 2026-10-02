package audit

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestUnknownOperationNamesCannotBecomeASecretLog(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HQ_DATA_ROOT", root)
	if err := Record("connections.private_token_value", "HQ_OPERATION_UNKNOWN"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "control.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte("private_token_value")) {
		t.Fatal("unknown operation payload was logged")
	}
}
