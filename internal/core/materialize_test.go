package core

import (
	"encoding/json"
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

func TestMaterializeReportsInvalidPayloadsAndDuplicateCreations(t *testing.T) {
	recordID, _ := protocol.NewRecordID()
	created, _ := protocol.NewEnvelope(protocol.EventRecordCreated,
		protocol.ActorRef{Kind: "human"}, protocol.TrustClaim{Level: "human_confirmed"},
		protocol.RecordCreated{Record: protocol.Record{
			RecordID: recordID, Kind: "direction", CreatedAt: time.Now().Format(time.RFC3339Nano),
			Scope: protocol.Scope{Global: true}, Source: protocol.SourceRef{Type: "human"},
			Text: "direction", Confidence: "human_confirmed", TTL: "until_superseded",
			Status: "active", Durability: "candidate",
		}})
	duplicate := created
	duplicate.EventID, _ = protocol.NewEventID()
	badCreate := created
	badCreate.EventID, _ = protocol.NewEventID()
	badCreate.Payload = json.RawMessage(`{`)
	badRelation, _ := protocol.NewEnvelope(protocol.EventRelationCreated,
		protocol.ActorRef{Kind: "agent"}, protocol.TrustClaim{Level: "agent_asserted"},
		protocol.RelationCreated{})
	badRelation.Payload = json.RawMessage(`{`)

	snapshot := Materialize([]protocol.EventEnvelope{created, duplicate, badCreate, badRelation})
	if len(snapshot.Conflicts) != 3 {
		t.Fatalf("conflicts = %v", snapshot.Conflicts)
	}
}

func TestMaterializeAppliesStateChangeFieldsAcrossChain(t *testing.T) {
	recordID, _ := protocol.NewRecordID()
	created, _ := protocol.NewEnvelope(protocol.EventRecordCreated,
		protocol.ActorRef{Kind: "human"}, protocol.TrustClaim{Level: "human_confirmed"},
		protocol.RecordCreated{Record: protocol.Record{
			RecordID: recordID, Kind: "direction", CreatedAt: time.Now().Format(time.RFC3339Nano),
			Scope: protocol.Scope{Global: true}, Source: protocol.SourceRef{Type: "human"},
			Text: "direction", Confidence: "human_confirmed", TTL: "until_superseded",
			Status: "active", Durability: "candidate",
		}})
	first, _ := protocol.NewEnvelope(protocol.EventRecordStateChanged,
		protocol.ActorRef{Kind: "agent", ID: "one"}, protocol.TrustClaim{Level: "agent_asserted"},
		protocol.RecordStateChanged{RecordID: recordID, Durability: "durable", LifecycleReason: "promoted"})
	first.ParentEventID = created.EventID
	second, _ := protocol.NewEnvelope(protocol.EventRecordStateChanged,
		protocol.ActorRef{Kind: "agent", ID: "two"}, protocol.TrustClaim{Level: "agent_asserted"},
		protocol.RecordStateChanged{RecordID: recordID, ReviewedAt: "2026-06-25T00:00:00Z"})
	second.ParentEventID = first.EventID
	ignored, _ := protocol.NewEnvelope(protocol.EventRecordStateChanged,
		protocol.ActorRef{Kind: "agent"}, protocol.TrustClaim{Level: "agent_asserted"},
		protocol.RecordStateChanged{RecordID: "rec_01978f71-79c7-7d53-a52a-cac034f15868", Status: "discarded"})
	ignored.ParentEventID = second.EventID

	record := Materialize([]protocol.EventEnvelope{created, first, second, ignored}).Records[recordID]
	if record.Record.Durability != "durable" || record.Record.LifecycleReason != "promoted" || record.Record.ReviewedAt != "2026-06-25T00:00:00Z" {
		t.Fatalf("state chain not applied: %#v", record.Record)
	}
	if record.HeadActor.ID != "two" {
		t.Fatalf("head actor = %#v, want second change actor", record.HeadActor)
	}
}

func TestMaterializeSortsRelations(t *testing.T) {
	relation := func(id string) protocol.EventEnvelope {
		event, _ := protocol.NewEnvelope(protocol.EventRelationCreated,
			protocol.ActorRef{Kind: "agent"}, protocol.TrustClaim{Level: "agent_asserted"},
			protocol.RelationCreated{Relation: protocol.Relation{
				RelationID: id, Type: protocol.RelationInformedBy,
				From: protocol.NodeRef{Kind: "record", ID: "a"}, To: protocol.NodeRef{Kind: "record", ID: "b"},
				CreatedAt: time.Now().Format(time.RFC3339Nano),
			}})
		return event
	}
	snapshot := Materialize([]protocol.EventEnvelope{relation("rel_b"), relation("rel_a")})
	if len(snapshot.Relations) != 2 || snapshot.Relations[0].RelationID != "rel_a" {
		t.Fatalf("relations not sorted: %#v", snapshot.Relations)
	}
}

func TestMaterializeDirectionsSortsByCreatedAtAndID(t *testing.T) {
	directionEvent := func(id, created string) protocol.EventEnvelope {
		event, _ := protocol.NewEnvelope(protocol.EventRecordCreated,
			protocol.ActorRef{Kind: "agent"}, protocol.TrustClaim{Level: "agent_asserted"},
			protocol.RecordCreated{Record: protocol.Record{
				RecordID: id, Kind: "direction", CreatedAt: created,
				Scope: protocol.Scope{Global: true}, Source: protocol.SourceRef{Type: "agent"},
				Text: "direction", Confidence: "agent_asserted", TTL: "until_superseded",
				Status: "active", Durability: "candidate",
			}})
		return event
	}
	directions, _ := MaterializeDirections([]protocol.EventEnvelope{
		directionEvent("rec_b", "2026-06-25T00:00:00Z"),
		directionEvent("rec_a", "2026-06-25T00:00:00Z"),
		directionEvent("rec_c", "2026-06-24T00:00:00Z"),
	})
	want := []string{"rec_c", "rec_a", "rec_b"}
	for i, id := range want {
		if directions[i].ID != id {
			t.Fatalf("direction %d = %s, want %s", i, directions[i].ID, id)
		}
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
