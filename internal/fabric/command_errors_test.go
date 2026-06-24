package fabric

import (
	"errors"
	"os"
	"testing"
)

func TestCommandLedgerAndGeneratedFileErrors(t *testing.T) {
	t.Run("init directory creation fails", func(t *testing.T) {
		chdirTemp(t)
		if err := os.WriteFile(".fabric", []byte("file"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := Run([]string{"init"}); err == nil {
			t.Fatal("init with .fabric file returned nil error")
		}
	})

	t.Run("init file creation fails", func(t *testing.T) {
		chdirTemp(t)
		if err := os.MkdirAll(".fabric/ledger", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(eventsPath, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := Run([]string{"init"}); err == nil {
			t.Fatal("init with directory event ledger returned nil error")
		}
	})

	t.Run("thread start rejects malformed events and unwritable threads ledger", func(t *testing.T) {
		chdirTemp(t)
		mustRun(t, "init")
		if err := os.WriteFile(eventsPath, []byte("{bad\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := Run([]string{"thread", "start", "--id", "thread-a", "--issue", "VS-123"}); err == nil {
			t.Fatal("thread start accepted malformed events ledger")
		}
		if err := os.WriteFile(eventsPath, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(threadsPath); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(threadsPath, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := Run([]string{"thread", "start", "--id", "thread-a", "--issue", "VS-123"}); err == nil {
			t.Fatal("thread start wrote to directory ledger")
		}
	})

	t.Run("note rejects malformed ledgers and unwritable event ledger", func(t *testing.T) {
		chdirTemp(t)
		mustRun(t, "init")
		if err := os.WriteFile(threadsPath, []byte("{bad\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := Run([]string{"note", "--global", "direction"}); err == nil {
			t.Fatal("note accepted malformed threads ledger")
		}
		if err := os.WriteFile(threadsPath, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(eventsPath, []byte("{bad\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := Run([]string{"note", "--global", "direction"}); err == nil {
			t.Fatal("note accepted malformed events ledger")
		}
		if err := os.Remove(eventsPath); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(eventsPath, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := Run([]string{"note", "--global", "direction"}); err == nil {
			t.Fatal("note wrote to directory events ledger")
		}
	})

	t.Run("note returns append failures", func(t *testing.T) {
		chdirTemp(t)
		mustRun(t, "init")

		withAppendLedger(t, func(path string, value any) error {
			if path == eventsPath {
				return errors.New("append event failed")
			}
			return appendJSONL(path, value)
		})
		if err := Run([]string{"note", "--global", "direction"}); err == nil {
			t.Fatal("note ignored event append failure")
		}

		appendLedger = appendJSONL
		mustRun(t, "thread", "start", "--id", "thread-a", "--issue", "VS-123")
		withAppendLedger(t, func(path string, value any) error {
			if path == threadsPath {
				return errors.New("append thread failed")
			}
			return appendJSONL(path, value)
		})
		if err := Run([]string{"note", "--thread", "thread-a", "--issue", "VS-123", "direction"}); err == nil {
			t.Fatal("note ignored source thread append failure")
		}
	})

	t.Run("review note rejects malformed ledgers and append failures", func(t *testing.T) {
		chdirTemp(t)
		mustRun(t, "init")
		if err := os.WriteFile(eventsPath, []byte("{bad\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := Run([]string{"review", "note", "--pr", "123", "direction"}); err == nil {
			t.Fatal("review note accepted malformed events ledger")
		}
		if err := os.WriteFile(eventsPath, nil, 0o644); err != nil {
			t.Fatal(err)
		}

		withAppendLedger(t, func(path string, value any) error {
			if path == eventsPath {
				return errors.New("append review failed")
			}
			return appendJSONL(path, value)
		})
		if err := Run([]string{"review", "note", "--pr", "123", "direction"}); err == nil {
			t.Fatal("review note ignored event append failure")
		}

		appendLedger = appendJSONL
		if err := os.WriteFile(threadsPath, []byte("{bad\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := Run([]string{"review", "note", "--pr", "123", "direction"}); err == nil {
			t.Fatal("review note accepted malformed threads ledger")
		}
	})

	t.Run("sync rejects malformed ledgers and unwritable generated file", func(t *testing.T) {
		chdirTemp(t)
		mustRun(t, "init")
		if err := os.WriteFile(threadsPath, []byte("{bad\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := Run([]string{"sync", "--thread", "thread-a"}); err == nil {
			t.Fatal("sync accepted malformed threads ledger")
		}
		if err := os.WriteFile(threadsPath, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		mustRun(t, "thread", "start", "--id", "thread-a", "--issue", "VS-123")
		if err := os.WriteFile(eventsPath, []byte("{bad\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := Run([]string{"sync", "--thread", "thread-a"}); err == nil {
			t.Fatal("sync accepted malformed events ledger")
		}
	})

	t.Run("sync fails when delta path is a directory", func(t *testing.T) {
		chdirTemp(t)
		mustRun(t, "init")
		mustRun(t, "thread", "start", "--id", "thread-a", "--issue", "VS-123")
		if err := os.Remove(syncPath); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		if err := os.Mkdir(syncPath, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := Run([]string{"sync", "--thread", "thread-a"}); err == nil {
			t.Fatal("sync wrote no-update delta to a directory")
		}
	})

	t.Run("sync fails when matched delta path is a directory", func(t *testing.T) {
		chdirTemp(t)
		mustRun(t, "init")
		mustRun(t, "thread", "start", "--id", "thread-a", "--issue", "VS-123")
		mustRun(t, "thread", "start", "--id", "thread-b", "--issue", "VS-123")
		mustRun(t, "note", "--durable", "--thread", "thread-a", "--issue", "VS-123", "direction")
		if err := os.Remove(syncPath); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		if err := os.Mkdir(syncPath, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := Run([]string{"sync", "--thread", "thread-b"}); err == nil {
			t.Fatal("sync wrote matched delta to a directory")
		}
	})

	t.Run("sync returns thread append failure", func(t *testing.T) {
		chdirTemp(t)
		mustRun(t, "init")
		mustRun(t, "thread", "start", "--id", "thread-a", "--issue", "VS-123")
		mustRun(t, "thread", "start", "--id", "thread-b", "--issue", "VS-123")
		mustRun(t, "note", "--durable", "--thread", "thread-a", "--issue", "VS-123", "direction")

		withAppendLedger(t, func(path string, value any) error {
			if path == threadsPath {
				return errors.New("append sync failed")
			}
			return appendJSONL(path, value)
		})
		if err := Run([]string{"sync", "--thread", "thread-b"}); err == nil {
			t.Fatal("sync ignored thread append failure")
		}
	})

	t.Run("preflight rejects malformed events and unwritable task direction", func(t *testing.T) {
		chdirTemp(t)
		mustRun(t, "init")
		if err := os.WriteFile(eventsPath, []byte("{bad\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := Run([]string{"preflight", "task", "--issue", "VS-123"}); err == nil {
			t.Fatal("preflight accepted malformed events ledger")
		}
		if err := os.WriteFile(eventsPath, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(taskPath); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		if err := os.Mkdir(taskPath, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := Run([]string{"preflight", "task", "--issue", "VS-123"}); err == nil {
			t.Fatal("preflight wrote task direction to a directory")
		}
	})

	t.Run("continue rejects malformed events and unwritable continuation file", func(t *testing.T) {
		chdirTemp(t)
		mustRun(t, "init")
		if err := os.WriteFile(eventsPath, []byte("{bad\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := Run([]string{"continue", "--pr", "123"}); err == nil {
			t.Fatal("continue accepted malformed events ledger")
		}
		if err := os.WriteFile(eventsPath, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(continuePath); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		if err := os.Mkdir(continuePath, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := Run([]string{"continue", "--pr", "123"}); err == nil {
			t.Fatal("continue wrote continuation context to a directory")
		}
	})

	t.Run("continue returns thread append failures", func(t *testing.T) {
		chdirTemp(t)
		mustRun(t, "init")

		withAppendLedger(t, func(path string, value any) error {
			if path == threadsPath {
				return errors.New("append continuation thread failed")
			}
			return appendJSONL(path, value)
		})
		if err := Run([]string{"continue", "--pr", "123", "--thread", "thread-c"}); err == nil {
			t.Fatal("continue ignored thread append failure")
		}
	})

	t.Run("explain rejects malformed ledgers", func(t *testing.T) {
		chdirTemp(t)
		mustRun(t, "init")
		if err := os.WriteFile(eventsPath, []byte("{bad\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := Run([]string{"explain", "--issue", "VS-123"}); err == nil {
			t.Fatal("explain accepted malformed events ledger")
		}
		if err := os.WriteFile(eventsPath, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(threadsPath, []byte("{bad\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := Run([]string{"explain", "--issue", "VS-123"}); err == nil {
			t.Fatal("explain accepted malformed threads ledger")
		}
	})
}

func withAppendLedger(t *testing.T, fn func(string, any) error) {
	t.Helper()
	previous := appendLedger
	appendLedger = fn
	t.Cleanup(func() {
		appendLedger = previous
	})
}
