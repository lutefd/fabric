package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCommandsRejectMalformedImmutableEvents(t *testing.T) {
	chdirTemp(t)
	mustRun(t, "init")
	if err := os.WriteFile(filepath.Join(ledgerEventsPath, "evt_bad.json"), []byte("{bad\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"status"},
		{"preflight", "task", "--issue", "VS-123"},
		{"explain", "--issue", "VS-123"},
		{"note", "--candidate", "--issue", "VS-123", "direction"},
	} {
		if err := Run(args); err == nil {
			t.Fatalf("%v accepted malformed immutable event", args)
		}
	}
}

func TestCommandsReturnGeneratedFileAndRuntimeFailures(t *testing.T) {
	t.Run("preflight generated path", func(t *testing.T) {
		chdirTemp(t)
		mustRun(t, "init")
		if err := os.RemoveAll(taskPath); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(taskPath, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := Run([]string{"preflight", "task", "--issue", "VS-123"}); err == nil {
			t.Fatal("preflight ignored generated-file failure")
		}
	})

	t.Run("thread runtime path", func(t *testing.T) {
		chdirTemp(t)
		if err := os.Mkdir(".git", 0o755); err != nil {
			t.Fatal(err)
		}
		mustRun(t, "init")
		runtimeDir, err := sharedRuntimePath(runtimeThreads)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(runtimeDir), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(runtimeDir, []byte("not a directory"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := Run([]string{"thread", "start", "--issue", "VS-123"}); err == nil {
			t.Fatal("thread start ignored runtime-store failure")
		}
	})
}

func TestJSONErrorsUseStableEnvelope(t *testing.T) {
	chdirTemp(t)
	output := captureStdout(t, func() {
		err := Run([]string{"--json", "status"})
		if err == nil || !IsRenderedError(err) {
			t.Fatalf("err = %v", err)
		}
	})
	assertContains(t, output, `"protocol_version":"fabric/1.0"`)
	assertContains(t, output, `"ok":false`)
	assertContains(t, output, `"error"`)
}
