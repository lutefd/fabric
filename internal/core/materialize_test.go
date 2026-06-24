package core

import (
	"testing"
	"time"

	"github.com/lutefd/fabric/protocol"
)

func TestMaterializeReportsCompetingStateChildren(t *testing.T) {
	recordID, err := protocol.NewRecordID()
	if err != nil {
		t.Fatal(err)
	}
	created, _ := protocol.NewEnvelope(protocol.EventRecordCreated,
		protocol.ActorRef{Kind: "human"}, protocol.TrustClaim{Level: "human_confirmed"},
		protocol.RecordCreated{Record: protocol.Record{
			RecordID: recordID, Kind: "direction", CreatedAt: time.Now().Format(time.RFC3339Nano),
			Scope: protocol.Scope{Global: true}, Source: protocol.SourceRef{Type: "human"},
			Text: "direction", Confidence: "human_confirmed", TTL: "until_superseded",
			Status: "active", Durability: "candidate",
		}})
	change := func(status string) protocol.EventEnvelope {
		event, _ := protocol.NewEnvelope(protocol.EventRecordStateChanged,
			protocol.ActorRef{Kind: "human"}, protocol.TrustClaim{Level: "human_confirmed"},
			protocol.RecordStateChanged{RecordID: recordID, Status: status})
		event.ParentEventID = created.EventID
		return event
	}
	snapshot := Materialize([]protocol.EventEnvelope{created, change("expired"), change("discarded")})
	if len(snapshot.Conflicts) != 1 {
		t.Fatalf("conflicts = %v", snapshot.Conflicts)
	}
	if snapshot.Records[recordID].Record.Status != "active" {
		t.Fatal("conflicting children should not be applied")
	}
	conflict := snapshot.Records[recordID].Conflict
	if conflict == nil || len(conflict.CompetingEventIDs) != 2 || conflict.ParentEventID != created.EventID {
		t.Fatalf("structured conflict = %#v", conflict)
	}
}

func TestMaterializePreservesCreationAndLifecycleAttribution(t *testing.T) {
	recordID, _ := protocol.NewRecordID()
	created, _ := protocol.NewEnvelope(protocol.EventRecordCreated,
		protocol.ActorRef{Kind: "human", ID: "human-1"}, protocol.TrustClaim{Level: "human_confirmed"},
		protocol.RecordCreated{Record: protocol.Record{
			RecordID: recordID, Kind: "direction", CreatedAt: time.Now().Format(time.RFC3339Nano),
			Scope: protocol.Scope{Global: true}, Source: protocol.SourceRef{Type: "human"},
			Text: "direction", Confidence: "human_confirmed", TTL: "until_superseded",
			Status: "active", Durability: "candidate",
		}})
	changed, _ := protocol.NewEnvelope(protocol.EventRecordStateChanged,
		protocol.ActorRef{Kind: "tool", ID: "fabric-cli"}, protocol.TrustClaim{Level: "tool_verified"},
		protocol.RecordStateChanged{RecordID: recordID, Status: "expired"})
	changed.ParentEventID = created.EventID

	record := Materialize([]protocol.EventEnvelope{created, changed}).Records[recordID]
	if record.Actor.Kind != "human" || record.Trust.Level != "human_confirmed" {
		t.Fatalf("creation attribution changed: %#v %#v", record.Actor, record.Trust)
	}
	if record.HeadActor.Kind != "tool" || record.HeadTrust.Level != "tool_verified" {
		t.Fatalf("head attribution = %#v %#v", record.HeadActor, record.HeadTrust)
	}
}
