package cli

import (
	"strings"
	"testing"
)

func TestMatchingHelpersCoverStaleSeenAndMisses(t *testing.T) {
	chdirTemp(t)
	mustRun(t, "init")
	mustRun(t, "thread", "start", "--id", "source", "--issue", "VS-123", "--area", "virtual-store/listing")
	mustRun(t, "thread", "start", "--id", "stale", "--issue", "VS-123")
	mustRun(t, "thread", "start", "--id", "unmatch", "--issue", "VS-999")
	mustRun(t, "note", "--durable", "--thread", "source", "Scoped direction")
	events, err := loadEvents()
	if err != nil {
		t.Fatal(err)
	}
	threads, err := loadThreads()
	if err != nil {
		t.Fatal(err)
	}
	stale, err := staleThreads(events[0], threads)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(stale, ",") != "stale" {
		t.Fatalf("stale = %v, want [stale]", stale)
	}
}

func TestPrioritizedEventsCoversChallengePriorityBranches(t *testing.T) {
	events := []DirectionEvent{
		{ID: "rec_d", Kind: "note", Scope: EventScope{Issue: "VS-123"}, Text: "issue"},
		{ID: "rec_c", Kind: "review_direction", Scope: EventScope{PR: "123"}, Text: "review"},
		{ID: "rec_b", Kind: "challenge", Scope: EventScope{Issue: "VS-123"}, Challenges: "rec_a", Status: "open", Text: "issue challenge"},
		{ID: "rec_a", Kind: "challenge", Scope: EventScope{Areas: []string{"file-opening"}}, Challenges: "rec_root", Status: "open", Text: "area challenge"},
		{ID: "rec_e", Kind: "challenge", Scope: EventScope{PR: "123"}, Challenges: "rec_c", Status: "accepted", Text: "accepted challenge"},
	}

	ordered := prioritizedEvents(events, "VS-123", "", []string{"file-opening"})
	got := make([]string, 0, len(ordered))
	for _, event := range ordered {
		got = append(got, event.ID)
	}
	if strings.Join(got, ",") != "rec_a,rec_b,rec_d,rec_c,rec_e" {
		t.Fatalf("ordered = %v", got)
	}

	if isOpenChallenge(events[4], nil) {
		t.Fatal("accepted challenge reported open")
	}
}

func TestAreaPathMappingsApplyToEitherSideOfMatch(t *testing.T) {
	chdirTemp(t)
	mustRun(t, "init")
	if err := writeFile(configPath, `repo: test
areas:
  api:
    paths:
      - internal/api/**
`); err != nil {
		t.Fatal(err)
	}
	pathRecord := DirectionEvent{Scope: EventScope{Paths: []string{"internal/api/handler.go"}}}
	if !reasonForScope(pathRecord, "", "", []string{"api"}).matched() {
		t.Fatal("thread area did not match record path through configured mapping")
	}
	areaRecord := DirectionEvent{Scope: EventScope{Areas: []string{"api"}}}
	if !reasonForScope(areaRecord, "", "", nil, []string{"internal/api/handler.go"}).matched() {
		t.Fatal("thread path did not match record area through configured mapping")
	}
}
