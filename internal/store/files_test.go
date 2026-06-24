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
