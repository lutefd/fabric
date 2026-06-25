package protocol

import (
	cryptorand "crypto/rand"
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
	if _, err := DecodeEvent([]byte(`{} [`)); err == nil {
		t.Fatal("invalid trailing JSON value accepted")
	}
}

func TestDecodeEventAcceptsValidEnvelope(t *testing.T) {
	event := validRecordEnvelope(t)
	raw := mustJSON(t, event)
	decoded, err := DecodeEvent(raw)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.EventID != event.EventID {
		t.Fatalf("decoded event id = %q, want %q", decoded.EventID, event.EventID)
	}
}

func TestNewEnvelopeRejectsUnmarshalablePayload(t *testing.T) {
	if _, err := NewEnvelope(EventRecordCreated, ActorRef{Kind: "human"}, TrustClaim{Level: "human_confirmed"}, make(chan int)); err == nil {
		t.Fatal("NewEnvelope accepted unmarshalable payload")
	}
}

func TestNewEnvelopeReportsIDGenerationErrors(t *testing.T) {
	previous := cryptorand.Reader
	cryptorand.Reader = errReader{}
	defer func() {
		cryptorand.Reader = previous
	}()
	if _, err := NewEnvelope(EventRecordCreated, ActorRef{Kind: "human"}, TrustClaim{Level: "human_confirmed"}, RecordCreated{}); err == nil {
		t.Fatal("NewEnvelope succeeded when ID generation failed")
	}
}

func TestEnvelopeValidationRejectsEnvelopeFields(t *testing.T) {
	valid := validRecordEnvelope(t)
	cases := []struct {
		name   string
		mutate func(*EventEnvelope)
	}{
		{"schema", func(e *EventEnvelope) { e.SchemaVersion = "fabric/0.9" }},
		{"event id", func(e *EventEnvelope) { e.EventID = "evt_invalid" }},
		{"event type", func(e *EventEnvelope) { e.EventType = "unknown" }},
		{"occurred at", func(e *EventEnvelope) { e.OccurredAt = "not-time" }},
		{"actor kind", func(e *EventEnvelope) { e.Actor.Kind = "robot" }},
		{"trust", func(e *EventEnvelope) { e.Trust.Level = "" }},
		{"payload empty", func(e *EventEnvelope) { e.Payload = nil }},
		{"payload invalid", func(e *EventEnvelope) { e.Payload = json.RawMessage(`{`) }},
		{"parent event", func(e *EventEnvelope) { e.ParentEventID = "evt_invalid" }},
	}
	for _, tc := range cases {
		event := valid
		tc.mutate(&event)
		if err := event.Validate(); err == nil {
			t.Fatalf("%s: invalid envelope accepted", tc.name)
		}
	}
}

func TestPayloadValidationAcceptsAllEventTypes(t *testing.T) {
	for _, event := range []EventEnvelope{
		validRecordEnvelope(t),
		validStateChangedEnvelope(t),
		validRelationEnvelope(t),
		validThreadEnvelope(t, EventThreadStarted),
		validThreadEnvelope(t, EventThreadScopeChanged),
		validProjectionEnvelope(t),
		validReceiptEnvelope(t),
	} {
		if err := event.Validate(); err != nil {
			t.Fatalf("%s rejected: %v", event.EventType, err)
		}
	}
}

func TestPayloadValidationRejectsUnknownPayloadFields(t *testing.T) {
	for _, event := range []EventEnvelope{
		validRecordEnvelope(t),
		validStateChangedEnvelope(t),
		validRelationEnvelope(t),
		validThreadEnvelope(t, EventThreadStarted),
		validProjectionEnvelope(t),
		validReceiptEnvelope(t),
	} {
		event.Payload = json.RawMessage(`{"unexpected":true}`)
		if err := event.Validate(); err == nil {
			t.Fatalf("%s accepted unknown payload field", event.EventType)
		}
	}
}

func TestRecordCreatedValidationRejectsBadFields(t *testing.T) {
	base := validRecordEnvelope(t)
	cases := []struct {
		name   string
		mutate func(*Record)
	}{
		{"missing required", func(r *Record) { r.Text = "" }},
		{"created at", func(r *Record) { r.CreatedAt = "not-time" }},
		{"empty scope", func(r *Record) { r.Scope = Scope{} }},
		{"status", func(r *Record) { r.Status = "missing" }},
		{"durability", func(r *Record) { r.Durability = "forever" }},
		{"duplicate areas", func(r *Record) { r.Scope = Scope{Areas: []string{"a", "a"}} }},
		{"empty path", func(r *Record) { r.Scope = Scope{Paths: []string{""}} }},
	}
	for _, tc := range cases {
		var payload RecordCreated
		if err := json.Unmarshal(base.Payload, &payload); err != nil {
			t.Fatal(err)
		}
		tc.mutate(&payload.Record)
		event := base
		event.Payload = mustJSON(t, payload)
		if err := event.Validate(); err == nil {
			t.Fatalf("%s: invalid record accepted", tc.name)
		}
	}
}

func TestStateChangedValidationRejectsBadFields(t *testing.T) {
	base := validStateChangedEnvelope(t)
	cases := []struct {
		name   string
		mutate func(*EventEnvelope, *RecordStateChanged)
	}{
		{"record id", func(_ *EventEnvelope, p *RecordStateChanged) { p.RecordID = "rec_invalid" }},
		{"parent", func(e *EventEnvelope, _ *RecordStateChanged) { e.ParentEventID = "" }},
		{"no changes", func(_ *EventEnvelope, p *RecordStateChanged) {
			p.Status, p.Durability, p.LifecycleReason, p.ReviewedAt = "", "", "", ""
		}},
		{"status", func(_ *EventEnvelope, p *RecordStateChanged) { p.Status = "missing" }},
		{"durability", func(_ *EventEnvelope, p *RecordStateChanged) { p.Durability = "forever" }},
		{"reviewed at", func(_ *EventEnvelope, p *RecordStateChanged) { p.ReviewedAt = "not-time" }},
	}
	for _, tc := range cases {
		var payload RecordStateChanged
		if err := json.Unmarshal(base.Payload, &payload); err != nil {
			t.Fatal(err)
		}
		event := base
		tc.mutate(&event, &payload)
		event.Payload = mustJSON(t, payload)
		if err := event.Validate(); err == nil {
			t.Fatalf("%s: invalid state change accepted", tc.name)
		}
	}
}

func TestRelationValidationRejectsBadFields(t *testing.T) {
	base := validRelationEnvelope(t)
	cases := []struct {
		name   string
		mutate func(*Relation)
	}{
		{"relation id", func(r *Relation) { r.RelationID = "rel_invalid" }},
		{"type", func(r *Relation) { r.Type = "missing" }},
		{"from", func(r *Relation) { r.From.ID = "" }},
		{"to", func(r *Relation) { r.To.Kind = "" }},
		{"created at", func(r *Relation) { r.CreatedAt = "not-time" }},
	}
	for _, tc := range cases {
		var payload RelationCreated
		if err := json.Unmarshal(base.Payload, &payload); err != nil {
			t.Fatal(err)
		}
		tc.mutate(&payload.Relation)
		event := base
		event.Payload = mustJSON(t, payload)
		if err := event.Validate(); err == nil {
			t.Fatalf("%s: invalid relation accepted", tc.name)
		}
	}
}

func TestThreadValidationRejectsBadFields(t *testing.T) {
	base := validThreadEnvelope(t, EventThreadStarted)
	cases := []struct {
		name   string
		mutate func(*Thread)
	}{
		{"thread id", func(th *Thread) { th.ThreadID = "" }},
		{"scope", func(th *Thread) { th.Scope = Scope{} }},
		{"created", func(th *Thread) { th.CreatedAt = "not-time" }},
		{"updated", func(th *Thread) { th.UpdatedAt = "not-time" }},
	}
	for _, tc := range cases {
		var payload ThreadEvent
		if err := json.Unmarshal(base.Payload, &payload); err != nil {
			t.Fatal(err)
		}
		tc.mutate(&payload.Thread)
		event := base
		event.Payload = mustJSON(t, payload)
		if err := event.Validate(); err == nil {
			t.Fatalf("%s: invalid thread accepted", tc.name)
		}
	}
}

func TestProjectionValidationRejectsBadFields(t *testing.T) {
	base := validProjectionEnvelope(t)
	cases := []struct {
		name   string
		mutate func(*Projection)
	}{
		{"projection id", func(p *Projection) { p.ProjectionID = "prj_invalid" }},
		{"purpose", func(p *Projection) { p.Purpose = "" }},
		{"scope", func(p *Projection) { p.Scope = Scope{} }},
		{"created", func(p *Projection) { p.CreatedAt = "not-time" }},
		{"duplicate events", func(p *Projection) { p.EventIDs = append(p.EventIDs, p.EventIDs[0]) }},
		{"bad event id", func(p *Projection) { p.EventIDs = []string{"evt_invalid"} }},
		{"duplicate records", func(p *Projection) { p.RecordIDs = append(p.RecordIDs, p.RecordIDs[0]) }},
		{"bad record id", func(p *Projection) { p.RecordIDs = []string{"rec_invalid"} }},
		{"bad conflict", func(p *Projection) { p.Conflicts[0].Message = "" }},
		{"bad competing id", func(p *Projection) { p.Conflicts[0].CompetingEventIDs[0] = "evt_invalid" }},
	}
	for _, tc := range cases {
		var payload ProjectionCreated
		if err := json.Unmarshal(base.Payload, &payload); err != nil {
			t.Fatal(err)
		}
		tc.mutate(&payload.Projection)
		event := base
		event.Payload = mustJSON(t, payload)
		if err := event.Validate(); err == nil {
			t.Fatalf("%s: invalid projection accepted", tc.name)
		}
	}
}

func TestReceiptValidationRejectsBadFields(t *testing.T) {
	base := validReceiptEnvelope(t)
	cases := []struct {
		name   string
		mutate func(*Receipt)
	}{
		{"receipt id", func(r *Receipt) { r.ReceiptID = "rcp_invalid" }},
		{"projection id", func(r *Receipt) { r.ProjectionID = "prj_invalid" }},
		{"thread id", func(r *Receipt) { r.ThreadID = "" }},
		{"state", func(r *Receipt) { r.State = "seen" }},
		{"occurred", func(r *Receipt) { r.OccurredAt = "not-time" }},
		{"duplicate events", func(r *Receipt) { r.EventIDs = append(r.EventIDs, r.EventIDs[0]) }},
		{"bad event id", func(r *Receipt) { r.EventIDs = []string{"evt_invalid"} }},
		{"duplicate records", func(r *Receipt) { r.RecordIDs = append(r.RecordIDs, r.RecordIDs[0]) }},
		{"bad record id", func(r *Receipt) { r.RecordIDs = []string{"rec_invalid"} }},
	}
	for _, tc := range cases {
		var payload ReceiptRecorded
		if err := json.Unmarshal(base.Payload, &payload); err != nil {
			t.Fatal(err)
		}
		tc.mutate(&payload.Receipt)
		event := base
		event.Payload = mustJSON(t, payload)
		if err := event.Validate(); err == nil {
			t.Fatalf("%s: invalid receipt accepted", tc.name)
		}
	}
}

func validStateChangedEnvelope(t *testing.T) EventEnvelope {
	t.Helper()
	recordID, _ := NewRecordID()
	parentID, _ := NewEventID()
	event, err := NewEnvelope(EventRecordStateChanged, ActorRef{Kind: "human"}, TrustClaim{Level: "human_confirmed"}, RecordStateChanged{
		RecordID: recordID, Status: "expired", Durability: "candidate", LifecycleReason: "done", ReviewedAt: time.Now().Format(time.RFC3339Nano),
	})
	if err != nil {
		t.Fatal(err)
	}
	event.ParentEventID = parentID
	return event
}

func validRelationEnvelope(t *testing.T) EventEnvelope {
	t.Helper()
	relationID, _ := NewRelationID()
	event, err := NewEnvelope(EventRelationCreated, ActorRef{Kind: "agent"}, TrustClaim{Level: "agent_asserted"}, RelationCreated{Relation: Relation{
		RelationID: relationID, Type: RelationInformedBy,
		From: NodeRef{Kind: "action", ID: "message-1"}, To: NodeRef{Kind: "record", ID: "rec-1"},
		CreatedAt: time.Now().Format(time.RFC3339Nano),
	}})
	if err != nil {
		t.Fatal(err)
	}
	return event
}

func validThreadEnvelope(t *testing.T, eventType string) EventEnvelope {
	t.Helper()
	event, err := NewEnvelope(eventType, ActorRef{Kind: "agent"}, TrustClaim{Level: "agent_asserted"}, ThreadEvent{Thread: Thread{
		ThreadID: "thread-1", CreatedAt: time.Now().Format(time.RFC3339Nano), UpdatedAt: time.Now().Format(time.RFC3339Nano),
		Scope: Scope{Issue: "FAB-1"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	return event
}

func validProjectionEnvelope(t *testing.T) EventEnvelope {
	t.Helper()
	projectionID, _ := NewProjectionID()
	eventID, _ := NewEventID()
	recordID, _ := NewRecordID()
	parentID, _ := NewEventID()
	competingID, _ := NewEventID()
	event, err := NewEnvelope(EventProjectionCreated, ActorRef{Kind: "agent"}, TrustClaim{Level: "agent_asserted"}, ProjectionCreated{Projection: Projection{
		ProjectionID: projectionID, Purpose: "sync", CreatedAt: time.Now().Format(time.RFC3339Nano), Scope: Scope{Global: true},
		EventIDs: []string{eventID}, RecordIDs: []string{recordID},
		Conflicts: []MaterializationConflict{{
			RecordID: recordID, ParentEventID: parentID, CompetingEventIDs: []string{eventID, competingID}, Message: "conflict",
		}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	return event
}

func validReceiptEnvelope(t *testing.T) EventEnvelope {
	t.Helper()
	receiptID, _ := NewReceiptID()
	projectionID, _ := NewProjectionID()
	eventID, _ := NewEventID()
	recordID, _ := NewRecordID()
	event, err := NewEnvelope(EventReceiptRecorded, ActorRef{Kind: "agent"}, TrustClaim{Level: "agent_asserted"}, ReceiptRecorded{Receipt: Receipt{
		ReceiptID: receiptID, ProjectionID: projectionID, ThreadID: "thread-1", State: ReceiptExposed,
		OccurredAt: time.Now().Format(time.RFC3339Nano), EventIDs: []string{eventID}, RecordIDs: []string{recordID},
	}})
	if err != nil {
		t.Fatal(err)
	}
	return event
}

func mustJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
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
