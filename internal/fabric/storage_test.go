package fabric

import (
	"os"
	"testing"
)

func TestStorageAndUtilityBranches(t *testing.T) {
	chdirTemp(t)

	if err := ensureInitialized(); err == nil {
		t.Fatal("ensureInitialized returned nil before init")
	}
	if _, err := loadEvents(); err == nil {
		t.Fatal("loadEvents returned nil before init")
	}
	if _, err := loadThreads(); err == nil {
		t.Fatal("loadThreads returned nil before init")
	}
	if got := repoName(); got == "" {
		t.Fatal("repoName fallback returned empty string")
	}
	var missing []DirectionEvent
	if err := readJSONL("missing.jsonl", &missing); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("parent-file", []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ensureInitialized(); err == nil {
		t.Fatal("ensureInitialized with file parent returned nil error")
	}
	if err := readJSONL("parent-file/child.jsonl", &missing); err == nil {
		t.Fatal("readJSONL with file parent returned nil error")
	}
	if err := os.WriteFile("blank.jsonl", []byte("\n\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := readJSONL("blank.jsonl", &missing); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("bad.jsonl", []byte("{bad\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := readJSONL("bad.jsonl", &missing); err == nil {
		t.Fatal("malformed JSONL returned nil error")
	}
	if err := readJSONL(".", &missing); err == nil {
		t.Fatal("reading directory as JSONL returned nil error")
	}
	if err := appendJSONL("events.jsonl", func() {}); err == nil {
		t.Fatal("appendJSONL accepted an unmarshalable value")
	}
	if err := appendJSONL("parent-file/events.jsonl", DirectionEvent{}); err == nil {
		t.Fatal("appendJSONL with file parent returned nil error")
	}
	if err := appendJSONL(".", DirectionEvent{}); err == nil {
		t.Fatal("appendJSONL to directory returned nil error")
	}

	mustRun(t, "init")
	if got := repoName(); got == "" {
		t.Fatal("repoName returned empty string")
	}
	if err := os.WriteFile(configPath, []byte("budgets:\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := repoName(); got == "" {
		t.Fatal("repoName without config repo returned empty string")
	}
	if err := os.WriteFile(eventsPath, []byte("{bad\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadEvents(); err == nil {
		t.Fatal("loadEvents accepted malformed events ledger")
	}
	if err := os.WriteFile(eventsPath, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(threadsPath, []byte("{bad\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadThreads(); err == nil {
		t.Fatal("loadThreads accepted malformed threads ledger")
	}
	if err := os.WriteFile(threadsPath, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeFileIfMissing("existing.txt", "first"); err != nil {
		t.Fatal(err)
	}
	if err := writeFileIfMissing("existing.txt", "second"); err != nil {
		t.Fatal(err)
	}
	if err := writeFileIfMissing("parent-file/child.txt", "x"); err == nil {
		t.Fatal("writeFileIfMissing with file parent returned nil error")
	}
	if got := mustRead(t, "existing.txt"); got != "first" {
		t.Fatalf("existing file content = %q, want first", got)
	}
	if err := touchIfMissing("touched/file.txt"); err != nil {
		t.Fatal(err)
	}
	if err := touchIfMissing("parent-file/touched.txt"); err == nil {
		t.Fatal("touchIfMissing with file parent returned nil error")
	}
	if err := touchIfMissing("."); err == nil {
		t.Fatal("touchIfMissing directory target returned nil error")
	}
	if err := writeFile("parent-file/written.txt", "x"); err == nil {
		t.Fatal("writeFile with file parent returned nil error")
	}
	if err := writeFile(".", "x"); err == nil {
		t.Fatal("writeFile directory target returned nil error")
	}

	if got := eventNumber(""); got != 0 {
		t.Fatalf("eventNumber empty = %d, want 0", got)
	}
	if got := eventNumber("evt_nope"); got != 0 {
		t.Fatalf("eventNumber malformed = %d, want 0", got)
	}
	if got := nextEventID([]DirectionEvent{{ID: "evt_000009"}}); got != "evt_000010" {
		t.Fatalf("nextEventID = %q, want evt_000010", got)
	}
	if got := issueFromBranch(); got != "" {
		t.Fatalf("issueFromBranch without HEAD = %q, want empty", got)
	}
	if got := emptyAsUnknown(""); got != "unknown" {
		t.Fatalf("emptyAsUnknown empty = %q", got)
	}
	if got := sourceThread(""); got != "note" {
		t.Fatalf("sourceThread empty = %q", got)
	}
	if got := sourceThread("thread-a"); got != "note from thread-a" {
		t.Fatalf("sourceThread populated = %q", got)
	}
	if got := (stringListFlag{"a"}).StringOrNone(); got != "a" {
		t.Fatalf("StringOrNone populated = %q", got)
	}
}

func TestEnsureInitializedReturnsStatErrors(t *testing.T) {
	chdirTemp(t)
	if err := os.WriteFile(".fabric", []byte("file"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ensureInitialized(); err == nil {
		t.Fatal("ensureInitialized with file parent returned nil error")
	}
}
