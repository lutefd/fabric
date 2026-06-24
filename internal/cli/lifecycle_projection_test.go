package cli

import (
	"encoding/json"
	"testing"

	"github.com/lutefd/fabric/protocol"
)

func TestLifecycleWithdrawalIsExplicitlyDelivered(t *testing.T) {
	chdirTemp(t)
	mustRun(t, "init")
	mustRun(t, "thread", "start", "--id", "thread-a", "--issue", "FAB-1")
	mustRun(t, "thread", "start", "--id", "thread-b", "--issue", "FAB-1")
	mustRun(t, "note", "--candidate", "--thread", "thread-a", "Original direction")
	recordID := recordIDAt(t, 0)
	mustRun(t, "sync", "--thread", "thread-b")

	mustRun(t, "expire", recordID, "--reason", "The task completed")
	mustRun(t, "sync", "--thread", "thread-b")
	delta := mustRead(t, syncPath)
	assertContains(t, delta, "[expired] Original direction")
	assertContains(t, delta, "Lifecycle reason: The task completed")

	graph, err := explainGraph(protocol.NodeRef{Kind: "record", ID: recordID}, "both", nil, 2)
	if err != nil {
		t.Fatal(err)
	}
	for _, detail := range graph.NodeDetails {
		if detail.Record == nil || detail.Record.Record.RecordID != recordID {
			continue
		}
		if detail.Record.Actor.Kind != "human" || detail.Record.HeadActor.Kind != "tool" {
			t.Fatalf("creation/head attribution = %#v", detail.Record)
		}
		return
	}
	t.Fatal("record detail not found")
}

func TestCompetingLifecycleChildrenReachProjection(t *testing.T) {
	chdirTemp(t)
	mustRun(t, "init")
	mustRun(t, "thread", "start", "--id", "thread-a", "--issue", "FAB-1")
	mustRun(t, "thread", "start", "--id", "thread-b", "--issue", "FAB-1")
	mustRun(t, "note", "--candidate", "--thread", "thread-a", "Direction with conflict")
	mustRun(t, "sync", "--thread", "thread-b")
	records, err := loadEvents()
	if err != nil {
		t.Fatal(err)
	}
	base := records[0]
	repository, err := directionRepository()
	if err != nil {
		t.Fatal(err)
	}
	for _, status := range []string{StatusExpired, StatusDiscarded} {
		envelope, err := protocol.NewEnvelope(protocol.EventRecordStateChanged,
			protocol.ActorRef{Kind: "human"}, protocol.TrustClaim{Level: "human_confirmed"},
			protocol.RecordStateChanged{RecordID: base.ID, Status: status})
		if err != nil {
			t.Fatal(err)
		}
		envelope.ParentEventID = base.HeadEventID
		if err := repository.Ledger.Put(envelope, true); err != nil {
			t.Fatal(err)
		}
	}

	mustRun(t, "sync", "--thread", "thread-b")
	delta := mustRead(t, syncPath)
	assertContains(t, delta, "[conflict] Direction with conflict")
	assertContains(t, delta, "Competing revisions:")

	events, err := loadRuntimeEvents(runtimeProjections)
	if err != nil {
		t.Fatal(err)
	}
	var payload protocol.ProjectionCreated
	if err := json.Unmarshal(events[len(events)-1].Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Projection.Conflicts) != 1 || len(payload.Projection.EventIDs) != 3 || len(payload.Projection.RecordIDs) != 1 {
		t.Fatalf("conflict projection = %#v", payload.Projection)
	}
}
