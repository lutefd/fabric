package main

import (
	"os"
	"testing"
)

func TestMainRunsSuccessfulCommand(t *testing.T) {
	oldArgs := os.Args
	os.Args = []string{"fabric", "version"}
	defer func() {
		os.Args = oldArgs
	}()

	main()
}

func TestMainWithArgsReturnsExecuteCode(t *testing.T) {
	if code := mainWithArgs([]string{"version"}); code != 0 {
		t.Fatalf("mainWithArgs(version) = %d, want 0", code)
	}
}

func TestMainWithExitOnlyExitsOnFailure(t *testing.T) {
	exited := false
	mainWithExit([]string{"version"}, func(int) {
		exited = true
	})
	if exited {
		t.Fatal("mainWithExit exited for successful command")
	}

	var exitCode int
	mainWithExit([]string{"unknown-command"}, func(code int) {
		exitCode = code
	})
	if exitCode == 0 {
		t.Fatal("mainWithExit did not exit for failed command")
	}
}
