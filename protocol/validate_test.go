package protocol

import (
	"encoding/json"
	"testing"
	"time"
)

func validRecordEnvelope(t *testing.T) EventEnvelope {
	t.Helper()
	recordID, err := NewRecordID()
	if err != nil {
		t.Fatal(err)
	}
	event, err := NewEnvelope(EventRecordCreated,
		ActorRef{Kind: "human", ID: "local-user"},
		TrustClaim{Level: "human_confirmed", Basis: "test"},
		RecordCreated{Record: Record{
			RecordID: recordID, Kind: "decision", CreatedAt: time.Now().Format(time.RFC3339Nano),
			Scope: Scope{Areas: []string{"protocol"}}, Source: SourceRef{Type: "human"},
			Text: "Use immutable events.", Confidence: "human_confirmed", TTL: "until_superseded",
			Status: "active", Durability: "durable",
		}})
	if err != nil {
		t.Fatal(err)
	}
	return event
}

func TestExtensionsSurviveRoundTrip(t *testing.T) {
	event := validRecordEnvelope(t)
	event.Extensions = map[string]json.RawMessage{"example.dev/provider": json.RawMessage(`{"opaque":true}`)}
	raw, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	var decoded EventEnvelope
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if string(decoded.Extensions["example.dev/provider"]) != `{"opaque":true}` {
		t.Fatalf("extension changed: %s", decoded.Extensions["example.dev/provider"])
	}
	if err := decoded.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestValidationRejectsIncompleteRecord(t *testing.T) {
	event := validRecordEnvelope(t)
	event.Payload = json.RawMessage(`{"record":{"record_id":"rec_invalid","text":"x"}}`)
	if err := event.Validate(); err == nil {
		t.Fatal("incomplete record was accepted")
	}
}

func TestDecodeEventRejectsUnknownFieldsOutsideExtensions(t *testing.T) {
	event := validRecordEnvelope(t)
	raw, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]any
	if err := json.Unmarshal(raw, &object); err != nil {
		t.Fatal(err)
	}
	object["provider_magic"] = true
	raw, _ = json.Marshal(object)
	if _, err := DecodeEvent(raw); err == nil {
		t.Fatal("unknown top-level field was accepted")
	}
}

func TestNodeKeyIncludesProviderWhenPresent(t *testing.T) {
	if got := (NodeRef{Kind: "record", ID: "rec-1"}).Key(); got != "record:rec-1" {
		t.Fatalf("key without provider = %q", got)
	}
	if got := (NodeRef{Kind: "action", Provider: "codex", ID: "opaque"}).Key(); got != "action:codex:opaque" {
		t.Fatalf("key with provider = %q", got)
	}
}

func TestKnownValuePredicates(t *testing.T) {
	for _, value := range []string{EventRecordCreated, EventRecordStateChanged, EventRelationCreated, EventThreadStarted, EventThreadScopeChanged, EventProjectionCreated, EventReceiptRecorded} {
		if !KnownEventType(value) {
			t.Fatalf("KnownEventType(%q) = false", value)
		}
	}
	if KnownEventType("missing") {
		t.Fatal("unknown event type accepted")
	}

	for _, value := range []string{RelationDerivedFrom, RelationInformedBy, RelationImplements, RelationSupersedes, RelationChallenges, RelationResolves, RelationDeliveredTo, RelationExposedTo} {
		if !KnownRelationType(value) {
			t.Fatalf("KnownRelationType(%q) = false", value)
		}
	}
	if KnownRelationType("missing") {
		t.Fatal("unknown relation type accepted")
	}

	for _, value := range []string{"human", "reviewer", "agent", "tool"} {
		if !KnownActorKind(value) {
			t.Fatalf("KnownActorKind(%q) = false", value)
		}
	}
	if KnownActorKind("robot") {
		t.Fatal("unknown actor kind accepted")
	}

	for _, value := range []string{"active", "expired", "discarded", "superseded", "open", "accepted", "rejected"} {
		if !KnownStatus(value) {
			t.Fatalf("KnownStatus(%q) = false", value)
		}
	}
	if KnownStatus("missing") {
		t.Fatal("unknown status accepted")
	}
}

func TestDecodeStrictRejectsMultipleJSONValues(t *testing.T) {
	if _, err := DecodeEvent([]byte(`{} {}`)); err == nil {
		t.Fatal("multiple JSON values accepted")
	}
}

func FuzzEventEnvelopeValidation(f *testing.F) {
	f.Add([]byte(`{"schema_version":"fabric/1.0"}`))
	f.Add([]byte(`not json`))
	f.Fuzz(func(t *testing.T, raw []byte) {
		var event EventEnvelope
		if json.Unmarshal(raw, &event) == nil {
			_ = event.Validate()
		}
	})
}
