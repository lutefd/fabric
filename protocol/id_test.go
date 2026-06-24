package protocol

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestTypedIDsAndEnvelopeValidation(t *testing.T) {
	ids := []struct {
		prefix string
		newID  func() (string, error)
	}{
		{"evt_", NewEventID}, {"rec_", NewRecordID}, {"thr_", NewThreadID},
		{"prj_", NewProjectionID}, {"rel_", NewRelationID}, {"rcp_", NewReceiptID},
	}
	seen := map[string]bool{}
	for _, item := range ids {
		id, err := item.newID()
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(id, item.prefix) || seen[id] {
			t.Fatalf("invalid or duplicate ID %q", id)
		}
		seen[id] = true
	}

	recordID, _ := NewRecordID()
	event, err := NewEnvelope(EventRecordCreated, ActorRef{Kind: "human"}, TrustClaim{Level: "human_confirmed"}, RecordCreated{Record: Record{
		RecordID: recordID, Kind: "direction", CreatedAt: time.Now().Format(time.RFC3339Nano),
		Scope: Scope{Global: true}, Source: SourceRef{Type: "human"},
		Text: "Use immutable events.", Confidence: "human_confirmed", TTL: "until_superseded",
		Status: "active", Durability: "durable",
	}})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(event)
	if err != nil || !json.Valid(raw) {
		t.Fatalf("invalid envelope JSON: %v", err)
	}
	if err := event.Validate(); err != nil {
		t.Fatalf("valid envelope rejected: %v", err)
	}
	if ValidTypedID("evt_not-a-uuid", "evt") {
		t.Fatal("malformed typed ID accepted")
	}
}

func TestDerivedRelationIDsAreStableAndTyped(t *testing.T) {
	receiptID, err := NewReceiptID()
	if err != nil {
		t.Fatal(err)
	}
	first, err := DeriveRelationID(receiptID, "record:one")
	if err != nil {
		t.Fatal(err)
	}
	again, _ := DeriveRelationID(receiptID, "record:one")
	second, _ := DeriveRelationID(receiptID, "record:two")
	if first != again || first == second || !ValidTypedID(first, "rel") {
		t.Fatalf("derived IDs first=%q again=%q second=%q", first, again, second)
	}
}
