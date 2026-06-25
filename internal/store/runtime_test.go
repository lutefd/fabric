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

func TestImmutableRuntimeStoreListsDefaultKindsAndRejectsUnknownKind(t *testing.T) {
	store := &ImmutableRuntimeStore{RootDir: t.TempDir()}
	events := []protocol.EventEnvelope{
		runtimeEnvelope(t, protocol.EventThreadScopeChanged),
		runtimeEnvelope(t, protocol.EventProjectionCreated),
		runtimeEnvelope(t, protocol.EventReceiptRecorded),
		runtimeEnvelope(t, protocol.EventRelationCreated),
	}
	for _, event := range events {
		if err := store.PutRuntime(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	listed, err := store.ListRuntime(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != len(events) {
		t.Fatalf("listed events = %d, want %d", len(listed), len(events))
	}
	if _, err := store.ListRuntime(context.Background(), "unknown"); err == nil {
		t.Fatal("unknown runtime kind accepted")
	}
}

func runtimeEnvelope(t *testing.T, eventType string) protocol.EventEnvelope {
	t.Helper()
	now := time.Now().Format(time.RFC3339Nano)
	switch eventType {
	case protocol.EventThreadScopeChanged:
		event, err := protocol.NewEnvelope(eventType, protocol.ActorRef{Kind: "agent"}, protocol.TrustClaim{Level: "agent_asserted"}, protocol.ThreadEvent{Thread: protocol.Thread{
			ThreadID: "thread-1", CreatedAt: now, UpdatedAt: now, Scope: protocol.Scope{Global: true},
		}})
		if err != nil {
			t.Fatal(err)
		}
		return event
	case protocol.EventProjectionCreated:
		projectionID, _ := protocol.NewProjectionID()
		event, err := protocol.NewEnvelope(eventType, protocol.ActorRef{Kind: "agent"}, protocol.TrustClaim{Level: "agent_asserted"}, protocol.ProjectionCreated{Projection: protocol.Projection{
			ProjectionID: projectionID, Purpose: "test", CreatedAt: now, Scope: protocol.Scope{Global: true},
		}})
		if err != nil {
			t.Fatal(err)
		}
		return event
	case protocol.EventReceiptRecorded:
		receiptID, _ := protocol.NewReceiptID()
		projectionID, _ := protocol.NewProjectionID()
		event, err := protocol.NewEnvelope(eventType, protocol.ActorRef{Kind: "agent"}, protocol.TrustClaim{Level: "agent_asserted"}, protocol.ReceiptRecorded{Receipt: protocol.Receipt{
			ReceiptID: receiptID, ProjectionID: projectionID, ThreadID: "thread-1", State: protocol.ReceiptDelivered, OccurredAt: now,
		}})
		if err != nil {
			t.Fatal(err)
		}
		return event
	case protocol.EventRelationCreated:
		relationID, _ := protocol.NewRelationID()
		event, err := protocol.NewEnvelope(eventType, protocol.ActorRef{Kind: "agent"}, protocol.TrustClaim{Level: "agent_asserted"}, protocol.RelationCreated{Relation: protocol.Relation{
			RelationID: relationID, Type: protocol.RelationInformedBy, From: protocol.NodeRef{Kind: "record", ID: "a"}, To: protocol.NodeRef{Kind: "record", ID: "b"}, CreatedAt: now,
		}})
		if err != nil {
			t.Fatal(err)
		}
		return event
	default:
		t.Fatalf("unsupported runtime envelope type %q", eventType)
	}
	return protocol.EventEnvelope{}
}
