package store

import (
	"context"
	"testing"
	"time"

	"github.com/lutefd/fabric/protocol"
)

func TestImmutableRuntimeStoreRoutesProtocolEvents(t *testing.T) {
	store := &ImmutableRuntimeStore{RootDir: t.TempDir()}
	thread := protocol.Thread{
		ThreadID: "provider-thread", CreatedAt: time.Now().Format(time.RFC3339Nano),
		UpdatedAt: time.Now().Format(time.RFC3339Nano), Scope: protocol.Scope{Issue: "FAB-1"},
	}
	event, err := protocol.NewEnvelope(protocol.EventThreadStarted,
		protocol.ActorRef{Kind: "agent"}, protocol.TrustClaim{Level: "agent_asserted"},
		protocol.ThreadEvent{Thread: thread})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.PutRuntime(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	events, err := store.ListRuntime(context.Background(), RuntimeThreads)
	if err != nil || len(events) != 1 || events[0].EventID != event.EventID {
		t.Fatalf("events=%#v err=%v", events, err)
	}

	recordEvent := testEnvelope(t)
	if err := store.PutRuntime(context.Background(), recordEvent); err == nil {
		t.Fatal("runtime store accepted a repository record event")
	}
}
