package core

import (
	"testing"

	"github.com/lutefd/fabric/protocol"
)

func TestPrepareDirectionLinksThreadPRAndEvidenceSources(t *testing.T) {
	event := DirectionEvent{
		Kind: "review_direction", CreatedAt: "2026-06-24T18:00:00Z",
		Scope: EventScope{PR: "42"}, Source: EventSource{Type: "review", ThreadID: "thread-a", PR: "42"},
		Text: "Use the shared resolver.", Confidence: "reviewer_confirmed", TTL: "until_pr_closed",
		Status: StatusActive, Durability: DurabilityCandidate,
		Evidence: []EvidenceRef{{Type: "reviewer_comment", URL: "https://example.invalid/review/1"}},
	}
	_, _, relations, err := PrepareDirection(event)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"thread:thread-a": false, "pr:42": false, "evidence:https://example.invalid/review/1": false}
	for _, relation := range relations {
		if _, ok := want[relation.To.Key()]; ok {
			want[relation.To.Key()] = true
		}
	}
	for key, found := range want {
		if !found {
			t.Fatalf("missing automatic source relation to %s: %#v", key, relations)
		}
	}
}

func TestDirectionProtocolRoundTripAndStateChange(t *testing.T) {
	event := DirectionEvent{
		Kind: "challenge", CreatedAt: "2026-06-24T18:00:00Z",
		Scope: EventScope{Issue: "FAB-1"}, Source: EventSource{Type: "human", ThreadID: "thread-a", URL: "https://example.invalid/msg"},
		Text: "Challenge the assumption.", Confidence: "human_confirmed", TTL: "until_resolved",
		Challenges: "rec_01978f71-79c7-7d53-a52a-cac034f15868",
		Evidence:   []EvidenceRef{{Type: "note", URL: "https://example.invalid/evidence", Author: "reviewer", Text: "because"}},
	}
	prepared, envelope, relations, err := PrepareDirection(event)
	if err != nil {
		t.Fatal(err)
	}
	if prepared.Status != StatusActive || prepared.Durability != DurabilityDurable {
		t.Fatalf("defaults not normalized: %#v", prepared)
	}
	if len(relations) != 4 {
		t.Fatalf("relations = %d, want 4: %#v", len(relations), relations)
	}

	record := DirectionToRecord(prepared)
	roundTrip := RecordToDirection(record, prepared.Actor, prepared.Trust)
	if roundTrip.Challenges != prepared.Challenges || len(roundTrip.Evidence) != 1 {
		t.Fatalf("round trip lost fields: %#v", roundTrip)
	}

	after := prepared
	after.Status = StatusDiscarded
	changed, stateEnvelope, err := StateChangeEnvelope(prepared, after, "bad direction", protocol.ActorRef{Kind: "agent", ID: "agent-1"}, protocol.TrustClaim{Level: "agent_asserted"})
	if err != nil {
		t.Fatal(err)
	}
	if changed.HeadEventID == "" || changed.HeadEventID == envelope.EventID || stateEnvelope.ParentEventID != envelope.EventID {
		t.Fatalf("state change envelope not linked: changed=%#v envelope=%#v", changed, stateEnvelope)
	}

	directions, conflicts := MaterializeDirections([]protocol.EventEnvelope{envelope, stateEnvelope})
	if len(conflicts) != 0 || len(directions) != 1 || directions[0].Status != StatusDiscarded {
		t.Fatalf("directions=%#v conflicts=%v", directions, conflicts)
	}
}

func TestActorAndTrustSources(t *testing.T) {
	explicitActor := protocol.ActorRef{Kind: "tool", ID: "tool-1"}
	explicitTrust := protocol.TrustClaim{Level: "tool_verified"}
	actor, trust := ActorAndTrust(DirectionEvent{Actor: explicitActor, Trust: explicitTrust})
	if actor != explicitActor || trust != explicitTrust {
		t.Fatalf("explicit actor/trust changed: %#v %#v", actor, trust)
	}

	cases := []struct {
		sourceType string
		kind       string
	}{
		{sourceType: "human", kind: "human"},
		{sourceType: "review", kind: "reviewer"},
		{sourceType: "pr_ingest", kind: "reviewer"},
		{sourceType: "tool", kind: "tool"},
		{sourceType: "agent", kind: "agent"},
	}
	for _, tc := range cases {
		actor, trust := ActorAndTrust(DirectionEvent{Source: EventSource{Type: tc.sourceType, ThreadID: "thread-1"}})
		if actor.Kind != tc.kind || actor.ID != "thread-1" || trust.Level != "agent_asserted" || trust.Basis != tc.sourceType {
			t.Fatalf("ActorAndTrust(%q) = %#v %#v", tc.sourceType, actor, trust)
		}
	}
}
