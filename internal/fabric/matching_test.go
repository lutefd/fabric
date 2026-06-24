package fabric

import (
	"strings"
	"testing"
)

func TestMatchingHelpersCoverStaleSeenAndMisses(t *testing.T) {
	event := DirectionEvent{
		ID: "evt_000002",
		Scope: EventScope{
			Issue: "VS-123",
			Areas: []string{"virtual-store/listing"},
		},
		Source: EventSource{ThreadID: "source"},
	}
	threads := map[string]ThreadRecord{
		"source":  {ThreadID: "source", Issue: "VS-123", Areas: []string{"virtual-store/listing"}},
		"seen":    {ThreadID: "seen", Issue: "VS-123", LastSeenEventID: "evt_000002"},
		"stale":   {ThreadID: "stale", Areas: []string{"virtual-store/listing"}, LastSeenEventID: "evt_000001"},
		"future":  {ThreadID: "future", Issue: "VS-123", LastSeenEventID: "evt_000003"},
		"unmatch": {ThreadID: "unmatch", Issue: "VS-999"},
	}

	stale := staleThreads(event, threads)
	if strings.Join(stale, ",") != "stale" {
		t.Fatalf("stale = %v, want [stale]", stale)
	}
	seen, stale := seenAndStale(event, threads)
	if strings.Join(seen, ",") != "future,seen" {
		t.Fatalf("seen = %v, want [future seen]", seen)
	}
	if strings.Join(stale, ",") != "source,stale" {
		t.Fatalf("stale = %v, want [source stale]", stale)
	}

	if got := latestRelevantEventID(nil, "VS-123", nil); got != "" {
		t.Fatalf("latestRelevantEventID(nil) = %q, want empty", got)
	}
}

func TestPrioritizedEventsCoversChallengePriorityBranches(t *testing.T) {
	events := []DirectionEvent{
		{ID: "evt_000004", Kind: "note", Scope: EventScope{Issue: "VS-123"}, Text: "issue"},
		{ID: "evt_000003", Kind: "review_direction", Scope: EventScope{PR: "123"}, Text: "review"},
		{ID: "evt_000002", Kind: "challenge", Scope: EventScope{Issue: "VS-123"}, Challenges: "evt_000001", Status: "open", Text: "issue challenge"},
		{ID: "evt_000001", Kind: "challenge", Scope: EventScope{Areas: []string{"file-opening"}}, Challenges: "evt_000000", Status: "open", Text: "area challenge"},
		{ID: "evt_000005", Kind: "challenge", Scope: EventScope{PR: "123"}, Challenges: "evt_000003", Status: "accepted", Text: "accepted challenge"},
	}

	ordered := prioritizedEvents(events, "VS-123", "", []string{"file-opening"})
	got := make([]string, 0, len(ordered))
	for _, event := range ordered {
		got = append(got, event.ID)
	}
	if strings.Join(got, ",") != "evt_000002,evt_000001,evt_000003,evt_000004,evt_000005" {
		t.Fatalf("ordered = %v", got)
	}

	if isOpenChallenge(events[4], nil) {
		t.Fatal("accepted challenge reported open")
	}
}
