package direction

import (
	"errors"
	"testing"
	"time"

	"github.com/lutefd/fabric/internal/core"
	"github.com/lutefd/fabric/internal/store"
	"github.com/lutefd/fabric/protocol"
)

func TestRepositoryCreateRoutesDurableAndLiveDirections(t *testing.T) {
	durableDir := t.TempDir()
	activeDir := t.TempDir()
	sharedDir := t.TempDir()
	repo := Repository{Ledger: store.Ledger{DurableDir: durableDir, ActiveDir: activeDir, SharedDir: sharedDir}}

	durable := testDirectionEvent("durable")
	durable.Source.URL = "https://example.test/message"
	durable.Source.ThreadID = "thr_test"
	durable.Evidence = []core.EvidenceRef{{URL: "https://example.test/evidence"}}
	if err := repo.Create(&durable); err != nil {
		t.Fatal(err)
	}
	if durable.ID == "" || durable.HeadEventID == "" {
		t.Fatalf("created direction missing generated IDs: %#v", durable)
	}

	live := testDirectionEvent("live")
	if err := repo.Create(&live); err != nil {
		t.Fatal(err)
	}

	durableEvents, _, err := store.Load(durableDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(durableEvents) == 0 {
		t.Fatal("durable direction was not written to durable ledger")
	}
	activeEvents, _, err := store.Load(activeDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(activeEvents) != 0 {
		t.Fatalf("live direction should not be written to active dir when shared dir is present: %d", len(activeEvents))
	}
	sharedEvents, _, err := store.Load(sharedDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(sharedEvents) < 2 {
		t.Fatalf("shared ledger events = %d, want at least record events", len(sharedEvents))
	}
}

func TestRepositoryCreateWritesLiveWithoutSharedMirror(t *testing.T) {
	activeDir := t.TempDir()
	repo := Repository{Ledger: store.Ledger{ActiveDir: activeDir}}

	event := testDirectionEvent("live")
	if err := repo.Create(&event); err != nil {
		t.Fatal(err)
	}

	activeEvents, _, err := store.Load(activeDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(activeEvents) != 1 {
		t.Fatalf("active events = %d, want 1", len(activeEvents))
	}
}

func TestRepositoryChangeAndDirections(t *testing.T) {
	repo := Repository{Ledger: store.Ledger{DurableDir: t.TempDir(), ActiveDir: t.TempDir()}}
	before := testDirectionEvent("candidate")
	if err := repo.Create(&before); err != nil {
		t.Fatal(err)
	}

	after := before
	after.Status = core.StatusExpired
	changed, err := repo.Change(before, after, "completed", protocol.ActorRef{Kind: "agent", ID: "agent-1"}, protocol.TrustClaim{Level: "agent_asserted"})
	if err != nil {
		t.Fatal(err)
	}
	if changed.Status != core.StatusExpired || changed.LifecycleReason != "completed" || changed.HeadEventID == before.HeadEventID {
		t.Fatalf("unexpected changed direction: %#v", changed)
	}

	directions, conflicts, err := repo.Directions()
	if err != nil {
		t.Fatal(err)
	}
	if len(conflicts) != 0 || len(directions) != 1 {
		t.Fatalf("directions=%d conflicts=%v", len(directions), conflicts)
	}
	if directions[0].Status != core.StatusExpired || directions[0].LifecycleReason != "completed" {
		t.Fatalf("materialized direction did not include state change: %#v", directions[0])
	}
}

func TestRepositoryErrors(t *testing.T) {
	repo := Repository{}
	if err := repo.Create(&core.DirectionEvent{}); err == nil {
		t.Fatal("Create succeeded with invalid ledger")
	}
	if _, err := repo.Change(core.DirectionEvent{}, core.DirectionEvent{}, "", protocol.ActorRef{}, protocol.TrustClaim{}); err == nil {
		t.Fatal("Change succeeded with invalid ledger")
	}
	if err := repo.PutRelation(protocol.Relation{}, "durable", protocol.ActorRef{}, protocol.TrustClaim{}); err == nil {
		t.Fatal("PutRelation succeeded with invalid ledger")
	}

	validRepo := Repository{Ledger: store.Ledger{DurableDir: t.TempDir()}}
	badRelation := protocol.Relation{}
	err := validRepo.PutRelation(badRelation, "durable", protocol.ActorRef{Kind: "agent"}, protocol.TrustClaim{Level: "agent_asserted"})
	if err == nil {
		t.Fatal("PutRelation accepted invalid relation")
	}
	if !errors.Is(err, err) {
		t.Fatal("unreachable sanity check")
	}
}

func TestDurableLike(t *testing.T) {
	for _, durability := range []string{"", "durable", "candidate"} {
		if !durableLike(durability) {
			t.Fatalf("durableLike(%q) = false, want true", durability)
		}
	}
	if durableLike("live") {
		t.Fatal("durableLike(live) = true, want false")
	}
}

func testDirectionEvent(durability string) core.DirectionEvent {
	return core.DirectionEvent{
		Kind:       "direction",
		CreatedAt:  time.Now().Format(time.RFC3339Nano),
		Scope:      core.EventScope{Global: true},
		Source:     core.EventSource{Type: "human"},
		Text:       "Preserve the thing.",
		Confidence: "human_confirmed",
		TTL:        "until_superseded",
		Durability: durability,
	}
}
