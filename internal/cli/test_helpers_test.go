package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func recordIDAt(t *testing.T, index int) string {
	t.Helper()
	events, err := loadEvents()
	if err != nil {
		t.Fatal(err)
	}
	if index < 0 || index >= len(events) {
		t.Fatalf("record index %d out of range for %d records", index, len(events))
	}
	return events[index].ID
}

func assertDelivered(t *testing.T, threadID, recordID string) {
	t.Helper()
	receipts, err := loadReceipts()
	if err != nil {
		t.Fatal(err)
	}
	for _, receipt := range receipts {
		if receipt.ThreadID != threadID {
			continue
		}
		for _, id := range receipt.RecordIDs {
			if id == recordID {
				return
			}
		}
	}
	t.Fatalf("record %s was not delivered to thread %s", recordID, threadID)
}

func assertNotDelivered(t *testing.T, threadID, recordID string) {
	t.Helper()
	receipts, err := loadReceipts()
	if err != nil {
		t.Fatal(err)
	}
	for _, receipt := range receipts {
		if receipt.ThreadID != threadID {
			continue
		}
		for _, id := range receipt.RecordIDs {
			if id == recordID {
				t.Fatalf("record %s was unexpectedly delivered to thread %s", recordID, threadID)
			}
		}
	}
}

func readJSONDir(t *testing.T, dir string) string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	var contents strings.Builder
	for _, path := range paths {
		contents.WriteString(mustRead(t, path))
		contents.WriteByte('\n')
	}
	return contents.String()
}

func chdirTemp(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
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

func mustRun(t *testing.T, args ...string) {
	t.Helper()
	if err := Run(args); err != nil {
		t.Fatalf("Run(%q): %v", strings.Join(args, " "), err)
	}
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func assertContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Fatalf("expected %q to contain %q", haystack, needle)
	}
}

func assertNotContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if strings.Contains(haystack, needle) {
		t.Fatalf("expected %q not to contain %q", haystack, needle)
	}
}

func assertOrder(t *testing.T, haystack, before, after string) {
	t.Helper()
	beforeIndex := strings.Index(haystack, before)
	afterIndex := strings.Index(haystack, after)
	if beforeIndex == -1 || afterIndex == -1 || beforeIndex >= afterIndex {
		t.Fatalf("expected %q to appear before %q in %q", before, after, haystack)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = writer
	defer func() {
		os.Stdout = old
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
