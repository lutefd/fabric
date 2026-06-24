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
