package store

import (
	"context"
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
