package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/lutefd/fabric/protocol"
)

func TestMatchingHelperCoverage(t *testing.T) {
	events := []DirectionEvent{
		{ID: "global", Scope: EventScope{Global: true}, Status: StatusActive},
		{ID: "inactive", Scope: EventScope{Global: true}, Status: StatusExpired},
		{ID: "issue", Scope: EventScope{Issue: "FAB-1"}, Status: StatusActive},
	}
	matches := relevantEvents(events, "FAB-1", nil)
	if len(matches) != 2 {
		t.Fatalf("relevantEvents matches = %#v", matches)
	}

	if scopePathMatches("", "main.go") {
		t.Fatal("empty path matched")
	}
	if !scopePathMatches("internal/**", "internal/cli/app.go") {
		t.Fatal("recursive scope path did not match")
	}

	challenge := DirectionEvent{ID: "challenge", Kind: "challenge", Status: "open", Scope: EventScope{Global: true}}
	resolution := DirectionEvent{ID: "resolution", Kind: "challenge_resolution", Challenges: "challenge", Scope: EventScope{Global: true}}
	conflict := DirectionEvent{ID: "conflict", Conflict: &protocol.MaterializationConflict{ParentEventID: "evt_parent"}, Scope: EventScope{Global: true}}
	reviewDirection := DirectionEvent{ID: "review", Kind: "review_direction", Scope: EventScope{Global: true}}
	ordered := prioritizedEvents([]DirectionEvent{reviewDirection, challenge, resolution, conflict}, "", "", nil)
	if ordered[0].ID != "conflict" {
		t.Fatalf("conflict should sort first: %#v", ordered)
	}
	if isOpenChallenge(challenge, map[string]DirectionEvent{"challenge": resolution}) {
		t.Fatal("resolved challenge reported open")
	}
}

func TestStorageHelperCoverage(t *testing.T) {
	events := []DirectionEvent{
		{ID: "active", Status: StatusActive, Durability: ""},
		{ID: "open", Status: "open", Durability: DurabilityLive},
		{ID: "accepted", Status: "accepted", Durability: DurabilityCandidate},
		{ID: "rejected", Status: "rejected", Durability: DurabilityDurable},
		{ID: "expired", Status: StatusExpired},
		{ID: "discarded", Status: StatusDiscarded},
		{ID: "superseded", Status: StatusSuperseded},
		{ID: "unknown", Status: "mystery"},
	}
	if normalizeStatus("") != StatusActive || normalizeDurability("") != DurabilityDurable {
		t.Fatal("normalization defaults changed")
	}
	active := filterActiveEvents(events)
	if len(active) != 4 {
		t.Fatalf("active events = %d, want 4", len(active))
	}
	counts := eventDurabilityCounts(active)
	if counts[DurabilityDurable] != 2 || counts[DurabilityCandidate] != 1 || counts[DurabilityLive] != 1 {
		t.Fatalf("counts = %#v", counts)
	}
}

func TestGitCommonDirVariants(t *testing.T) {
	chdirTemp(t)
	if got, err := gitCommonDir(); err != nil || got != "" {
		t.Fatalf("no git common dir = %q err=%v", got, err)
	}

	if err := os.WriteFile(".git", []byte("not a gitdir file"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, err := gitCommonDir(); err != nil || got != "" {
		t.Fatalf("plain .git file common dir = %q err=%v", got, err)
	}

	if err := os.Remove(".git"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(".gitdir", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(".git", []byte("gitdir: .gitdir\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := gitCommonDir()
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(mustGetwd(), ".gitdir") {
		t.Fatalf("gitdir common dir = %q", got)
	}

	common := filepath.Join(mustGetwd(), "common")
	if err := os.WriteFile(filepath.Join(".gitdir", "commondir"), []byte("../common\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err = gitCommonDir()
	if err != nil {
		t.Fatal(err)
	}
	if got != common {
		t.Fatalf("commondir = %q, want %q", got, common)
	}
}

func TestMarshalGraph(t *testing.T) {
	raw := marshalGraph(protocol.Graph{Root: protocol.NodeRef{Kind: "record", ID: "rec-1"}})
	if !json.Valid(raw) {
		t.Fatalf("invalid graph JSON: %s", raw)
	}
}
