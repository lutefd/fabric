package store

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/lutefd/fabric/protocol"
)

func testEnvelope(t *testing.T) protocol.EventEnvelope {
	t.Helper()
	recordID, err := protocol.NewRecordID()
	if err != nil {
		t.Fatal(err)
	}
	event, err := protocol.NewEnvelope(protocol.EventRecordCreated,
		protocol.ActorRef{Kind: "human"}, protocol.TrustClaim{Level: "human_confirmed"},
		protocol.RecordCreated{Record: protocol.Record{
			RecordID: recordID, Kind: "direction", CreatedAt: time.Now().Format(time.RFC3339Nano),
			Scope: protocol.Scope{Global: true}, Source: protocol.SourceRef{Type: "human"},
			Text: "direction", Confidence: "human_confirmed", TTL: "until_superseded",
			Status: "active", Durability: "durable",
		}})
	if err != nil {
		t.Fatal(err)
	}
	return event
}

func TestImmutableFileStoreIsIdempotentAndCreateOnly(t *testing.T) {
	dir := t.TempDir()
	event := testEnvelope(t)
	store := NewImmutableFileStore(dir)
	if err := store.Put(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	if err := store.Put(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	events, err := store.List(context.Background())
	if err != nil || len(events) != 1 {
		t.Fatalf("events=%d err=%v", len(events), err)
	}
	event.EventType = protocol.EventThreadStarted
	if err := store.Put(context.Background(), event); err == nil {
		t.Fatal("different content replaced an immutable event")
	}
}

func TestImmutableFileStoreRequiresWriteDir(t *testing.T) {
	store := NewImmutableFileStore("")
	if err := store.Put(context.Background(), testEnvelope(t)); err == nil {
		t.Fatal("Put succeeded without a write directory")
	}
}

func TestWriteImmutableErrorBranches(t *testing.T) {
	event := testEnvelope(t)
	if err := WriteImmutable(t.TempDir(), protocol.EventEnvelope{}); err == nil {
		t.Fatal("invalid event accepted")
	}

	mkdirErr := errors.New("mkdir failed")
	withStoreHooks(t, storeHooks{mkdirAll: func(string, os.FileMode) error { return mkdirErr }})
	if err := WriteImmutable(t.TempDir(), event); !errors.Is(err, mkdirErr) {
		t.Fatalf("mkdir err = %v, want %v", err, mkdirErr)
	}

	marshalErr := errors.New("marshal failed")
	withStoreHooks(t, storeHooks{marshalIndent: func(any, string, string) ([]byte, error) { return nil, marshalErr }})
	if err := WriteImmutable(t.TempDir(), event); !errors.Is(err, marshalErr) {
		t.Fatalf("marshal err = %v, want %v", err, marshalErr)
	}

	readErr := errors.New("read failed")
	withStoreHooks(t, storeHooks{readFile: func(string) ([]byte, error) { return nil, readErr }})
	if err := WriteImmutable(t.TempDir(), event); !errors.Is(err, readErr) {
		t.Fatalf("read err = %v, want %v", err, readErr)
	}

	createErr := errors.New("create failed")
	withStoreHooks(t, storeHooks{createTemp: func(string, string) (tempFile, error) { return nil, createErr }})
	if err := WriteImmutable(t.TempDir(), event); !errors.Is(err, createErr) {
		t.Fatalf("create err = %v, want %v", err, createErr)
	}

	for name, temp := range map[string]*fakeTempFile{
		"write": {writeErr: errors.New("write failed")},
		"sync":  {syncErr: errors.New("sync failed")},
		"close": {closeErr: errors.New("close failed")},
	} {
		t.Run(name, func(t *testing.T) {
			withStoreHooks(t, storeHooks{createTemp: func(string, string) (tempFile, error) { return temp, nil }})
			if err := WriteImmutable(t.TempDir(), event); err == nil {
				t.Fatal("WriteImmutable succeeded with failing temp file")
			}
		})
	}

	linkErr := errors.New("link failed")
	withStoreHooks(t, storeHooks{
		createTemp: func(string, string) (tempFile, error) {
			return &fakeTempFile{name: filepath.Join(t.TempDir(), "tmp")}, nil
		},
		linkFile: func(string, string) error { return linkErr },
	})
	if err := WriteImmutable(t.TempDir(), event); !errors.Is(err, linkErr) {
		t.Fatalf("link err = %v, want %v", err, linkErr)
	}
}

func TestWriteImmutableHandlesConcurrentExistingLink(t *testing.T) {
	dir := t.TempDir()
	event := testEnvelope(t)
	encoded := mustMarshalIndentedEvent(t, event)
	withStoreHooks(t, storeHooks{
		createTemp: func(string, string) (tempFile, error) { return &fakeTempFile{name: filepath.Join(dir, "tmp")}, nil },
		linkFile: func(_, target string) error {
			if err := os.WriteFile(target, encoded, 0o644); err != nil {
				t.Fatal(err)
			}
			return os.ErrExist
		},
	})
	if err := WriteImmutable(dir, event); err != nil {
		t.Fatal(err)
	}
}

func TestWriteImmutableRejectsExistingDifferentContent(t *testing.T) {
	dir := t.TempDir()
	event := testEnvelope(t)
	if err := WriteImmutable(dir, event); err != nil {
		t.Fatal(err)
	}
	event.Actor.ID = "different"
	if err := WriteImmutable(dir, event); err == nil {
		t.Fatal("different immutable content accepted")
	}
}

func TestLedgerRoutesDurableActiveAndSharedEvents(t *testing.T) {
	durableDir := t.TempDir()
	activeDir := t.TempDir()
	sharedDir := t.TempDir()
	ledger := Ledger{DurableDir: durableDir, ActiveDir: activeDir, SharedDir: sharedDir}

	durableEvent := testEnvelope(t)
	if err := ledger.Put(durableEvent, true); err != nil {
		t.Fatal(err)
	}
	liveEvent := testEnvelope(t)
	if err := ledger.Put(liveEvent, false); err != nil {
		t.Fatal(err)
	}

	durableEvents, _, err := Load(durableDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(durableEvents) != 1 {
		t.Fatalf("durable events = %d, want 1", len(durableEvents))
	}
	activeEvents, _, err := Load(activeDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(activeEvents) != 0 {
		t.Fatalf("active events with shared mirror = %d, want 0", len(activeEvents))
	}
	events, conflicts, err := ledger.List()
	if err != nil || len(conflicts) != 0 || len(events) != 2 {
		t.Fatalf("ledger.List events=%d conflicts=%v err=%v", len(events), conflicts, err)
	}

	activeOnly := Ledger{ActiveDir: t.TempDir()}
	if err := activeOnly.Put(testEnvelope(t), false); err != nil {
		t.Fatal(err)
	}
	activeEvents, _, err = Load(activeOnly.ActiveDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(activeEvents) != 1 {
		t.Fatalf("active-only live events = %d, want 1", len(activeEvents))
	}
}

func TestLedgerPutErrorBranches(t *testing.T) {
	ledger := Ledger{DurableDir: t.TempDir(), ActiveDir: t.TempDir(), SharedDir: t.TempDir()}
	if err := ledger.Put(protocol.EventEnvelope{}, true); err == nil {
		t.Fatal("Ledger accepted invalid event")
	}

	writeErr := errors.New("write failed")
	withStoreHooks(t, storeHooks{writeImmutable: func(string, protocol.EventEnvelope) error { return writeErr }})
	if err := ledger.Put(testEnvelope(t), true); !errors.Is(err, writeErr) {
		t.Fatalf("shared write err = %v, want %v", err, writeErr)
	}

	calls := 0
	withStoreHooks(t, storeHooks{writeImmutable: func(string, protocol.EventEnvelope) error {
		calls++
		if calls == 2 {
			return writeErr
		}
		return nil
	}})
	if err := ledger.Put(testEnvelope(t), true); !errors.Is(err, writeErr) {
		t.Fatalf("durable write err = %v, want %v", err, writeErr)
	}

	withStoreHooks(t, storeHooks{writeImmutable: func(string, protocol.EventEnvelope) error { return writeErr }})
	if err := (Ledger{ActiveDir: t.TempDir()}).Put(testEnvelope(t), false); !errors.Is(err, writeErr) {
		t.Fatalf("active write err = %v, want %v", err, writeErr)
	}
}

func TestImmutableStoreIgnoresCrashTempFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".fabric-event-orphan"), []byte("partial"), 0o644); err != nil {
		t.Fatal(err)
	}
	events, conflicts, err := Load(dir)
	if err != nil || len(events) != 0 || len(conflicts) != 0 {
		t.Fatalf("events=%v conflicts=%v err=%v", events, conflicts, err)
	}
}

func TestLoadErrorBranchesAndConflicts(t *testing.T) {
	readDirErr := errors.New("readdir failed")
	withStoreHooks(t, storeHooks{readDir: func(string) ([]os.DirEntry, error) { return nil, readDirErr }})
	if _, _, err := Load(t.TempDir()); !errors.Is(err, readDirErr) {
		t.Fatalf("readDir err = %v, want %v", err, readDirErr)
	}

	readErr := errors.New("read failed")
	withStoreHooks(t, storeHooks{readFile: func(string) ([]byte, error) { return nil, readErr }})
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "event.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Load(dir); !errors.Is(err, readErr) {
		t.Fatalf("readFile err = %v, want %v", err, readErr)
	}

	dir = t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "event.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	resetStoreHooks()
	if _, _, err := Load(dir); err == nil {
		t.Fatal("invalid event JSON accepted")
	}

	resetStoreHooks()
	left := t.TempDir()
	right := t.TempDir()
	event := testEnvelope(t)
	if err := WriteImmutable(left, event); err != nil {
		t.Fatal(err)
	}
	event.Actor.ID = "different"
	if err := os.WriteFile(filepath.Join(right, event.EventID+".json"), mustMarshalIndentedEvent(t, event), 0o644); err != nil {
		t.Fatal(err)
	}
	events, conflicts, err := Load("", left, "missing", right)
	if err != nil || len(events) != 1 || len(conflicts) != 1 {
		t.Fatalf("events=%d conflicts=%v err=%v", len(events), conflicts, err)
	}

	sorted := t.TempDir()
	first := testEnvelope(t)
	second := testEnvelope(t)
	second.OccurredAt = first.OccurredAt
	if second.EventID < first.EventID {
		first, second = second, first
	}
	if err := WriteImmutable(sorted, second); err != nil {
		t.Fatal(err)
	}
	if err := WriteImmutable(sorted, first); err != nil {
		t.Fatal(err)
	}
	events, _, err = Load(sorted)
	if err != nil {
		t.Fatal(err)
	}
	if events[0].EventID != first.EventID {
		t.Fatalf("events not sorted by event ID on time tie: %#v", events)
	}
}

func TestImmutableStoreProcessHelper(t *testing.T) {
	dir := os.Getenv("FABRIC_TEST_EVENT_DIR")
	if dir == "" {
		return
	}
	if err := WriteImmutable(dir, testEnvelope(t)); err != nil {
		t.Fatal(err)
	}
}

func TestImmutableStoreSupportsMultipleProcesses(t *testing.T) {
	dir := t.TempDir()
	const processCount = 6
	commands := make([]*exec.Cmd, 0, processCount)
	for index := 0; index < processCount; index++ {
		command := exec.Command(os.Args[0], "-test.run=^TestImmutableStoreProcessHelper$")
		command.Env = append(os.Environ(), "FABRIC_TEST_EVENT_DIR="+dir)
		if err := command.Start(); err != nil {
			t.Fatal(err)
		}
		commands = append(commands, command)
	}
	for _, command := range commands {
		if err := command.Wait(); err != nil {
			t.Fatalf("helper process failed: %v", err)
		}
	}
	events, conflicts, err := Load(dir)
	if err != nil || len(conflicts) != 0 || len(events) != processCount {
		t.Fatalf("events=%d conflicts=%v err=%v", len(events), conflicts, err)
	}
}

type storeHooks struct {
	writeImmutable func(string, protocol.EventEnvelope) error
	mkdirAll       func(string, os.FileMode) error
	marshalIndent  func(any, string, string) ([]byte, error)
	readFile       func(string) ([]byte, error)
	createTemp     func(string, string) (tempFile, error)
	removeFile     func(string) error
	linkFile       func(string, string) error
	readDir        func(string) ([]os.DirEntry, error)
}

func withStoreHooks(t *testing.T, hooks storeHooks) {
	t.Helper()
	resetStoreHooks()
	if hooks.writeImmutable != nil {
		writeImmutable = hooks.writeImmutable
	}
	if hooks.mkdirAll != nil {
		mkdirAll = hooks.mkdirAll
	}
	if hooks.marshalIndent != nil {
		marshalIndent = hooks.marshalIndent
	}
	if hooks.readFile != nil {
		readFile = hooks.readFile
	}
	if hooks.createTemp != nil {
		createTemp = hooks.createTemp
	}
	if hooks.removeFile != nil {
		removeFile = hooks.removeFile
	}
	if hooks.linkFile != nil {
		linkFile = hooks.linkFile
	}
	if hooks.readDir != nil {
		readDir = hooks.readDir
	}
	t.Cleanup(resetStoreHooks)
}

func resetStoreHooks() {
	writeImmutable = writeImmutableFile
	mkdirAll = os.MkdirAll
	marshalIndent = json.MarshalIndent
	readFile = os.ReadFile
	createTemp = func(dir, pattern string) (tempFile, error) { return os.CreateTemp(dir, pattern) }
	removeFile = os.Remove
	linkFile = os.Link
	readDir = os.ReadDir
}

type fakeTempFile struct {
	name     string
	writeErr error
	syncErr  error
	closeErr error
}

func (f *fakeTempFile) Name() string {
	if f.name == "" {
		return "fake-temp"
	}
	return f.name
}

func (f *fakeTempFile) Write([]byte) (int, error) {
	if f.writeErr != nil {
		return 0, f.writeErr
	}
	return 1, nil
}

func (f *fakeTempFile) Sync() error {
	return f.syncErr
}

func (f *fakeTempFile) Close() error {
	return f.closeErr
}

func mustMarshalIndentedEvent(t *testing.T, event protocol.EventEnvelope) []byte {
	t.Helper()
	encoded, err := json.MarshalIndent(event, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return append(encoded, '\n')
}
