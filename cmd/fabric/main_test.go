package main

import (
	"bytes"
	"os"
	"testing"
)

func TestMainWithArgsReturnsZeroOnSuccess(t *testing.T) {
	chdirTemp(t)
	if code := mainWithArgs([]string{"help"}); code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
}

func TestMainFunctionReturnsOnSuccess(t *testing.T) {
	chdirTemp(t)
	oldArgs := os.Args
	os.Args = []string{"fabric", "help"}
	t.Cleanup(func() {
		os.Args = oldArgs
	})
	main()
}

func TestMainWithArgsReturnsOneOnError(t *testing.T) {
	chdirTemp(t)
	stderr := captureStderr(t, func() {
		if code := mainWithArgs([]string{"missing"}); code != 1 {
			t.Fatalf("exit code = %d, want 1", code)
		}
	})
	if !bytes.Contains([]byte(stderr), []byte("fabric:")) {
		t.Fatalf("stderr = %q, want fabric prefix", stderr)
	}
}

func TestMainWithExitCallsExitOnError(t *testing.T) {
	chdirTemp(t)
	var got int
	mainWithExit([]string{"missing"}, func(code int) {
		got = code
	})
	if got != 1 {
		t.Fatalf("exit code = %d, want 1", got)
	}
}

func chdirTemp(t *testing.T) {
	t.Helper()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previous); err != nil {
			t.Fatal(err)
		}
	})
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stderr
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = writer
	defer func() {
		os.Stderr = old
	}()

	fn()
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(reader); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}
