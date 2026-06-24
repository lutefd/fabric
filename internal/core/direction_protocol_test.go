package core

import "testing"

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
