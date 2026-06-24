package cli

import (
	"bytes"
	"testing"
)

func TestExecuteReturnsProcessStatus(t *testing.T) {
	var stderr bytes.Buffer
	if code := Execute([]string{"help"}, &stderr); code != 0 {
		t.Fatalf("help exit code = %d, want 0", code)
	}
	if code := Execute([]string{"missing"}, &stderr); code != 1 {
		t.Fatalf("invalid command exit code = %d, want 1", code)
	}
	if !bytes.Contains(stderr.Bytes(), []byte("fabric:")) {
		t.Fatalf("stderr = %q, want fabric prefix", stderr.String())
	}
}
