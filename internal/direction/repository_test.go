package direction

import (
	cryptorand "crypto/rand"
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

func TestRepositoryCreateErrorBranches(t *testing.T) {
	validateErr := errors.New("list failed")
	repo := Repository{Ledger: &fakeLedger{listErr: validateErr}}
	if err := repo.Create(&core.DirectionEvent{}); !errors.Is(err, validateErr) {
		t.Fatalf("Create validate err = %v, want %v", err, validateErr)
	}

	previous := cryptorand.Reader
	cryptorand.Reader = directionErrReader{}
	err := Repository{Ledger: &fakeLedger{}}.Create(&core.DirectionEvent{})
	cryptorand.Reader = previous
	if err == nil {
		t.Fatal("Create succeeded when direction preparation failed")
	}

	putErr := errors.New("put failed")
	event := testDirectionEvent("durable")
	if err := (Repository{Ledger: &fakeLedger{putErr: putErr}}).Create(&event); !errors.Is(err, putErr) {
		t.Fatalf("Create put err = %v, want %v", err, putErr)
	}

	relationErr := errors.New("relation failed")
	event = testDirectionEvent("durable")
	event.Source.URL = "https://example.invalid/source"
	ledger := &fakeLedger{putErrOnCall: map[int]error{2: relationErr}}
	if err := (Repository{Ledger: ledger}).Create(&event); !errors.Is(err, relationErr) {
		t.Fatalf("Create relation err = %v, want %v", err, relationErr)
	}
}

func TestRepositoryChangeErrorBranches(t *testing.T) {
	validateErr := errors.New("list failed")
	if _, err := (Repository{Ledger: &fakeLedger{listErr: validateErr}}).Change(core.DirectionEvent{}, core.DirectionEvent{}, "", protocol.ActorRef{}, protocol.TrustClaim{}); !errors.Is(err, validateErr) {
		t.Fatalf("Change validate err = %v, want %v", err, validateErr)
	}

	if _, err := (Repository{Ledger: &fakeLedger{}}).Change(core.DirectionEvent{}, core.DirectionEvent{}, "", protocol.ActorRef{Kind: "agent"}, protocol.TrustClaim{Level: "agent_asserted"}); err == nil {
		t.Fatal("Change succeeded with invalid state change envelope")
	}

	putErr := errors.New("put failed")
	before := testDirectionEvent("candidate")
	before.ID, _ = protocol.NewRecordID()
	before.HeadEventID, _ = protocol.NewEventID()
	after := before
	after.Status = core.StatusExpired
	if _, err := (Repository{Ledger: &fakeLedger{putErr: putErr}}).Change(before, after, "", protocol.ActorRef{Kind: "agent"}, protocol.TrustClaim{Level: "agent_asserted"}); !errors.Is(err, putErr) {
		t.Fatalf("Change put err = %v, want %v", err, putErr)
	}
}

func TestRepositoryPutRelationErrorBranches(t *testing.T) {
	validateErr := errors.New("list failed")
	if err := (Repository{Ledger: &fakeLedger{listErr: validateErr}}).PutRelation(protocol.Relation{}, "durable", protocol.ActorRef{}, protocol.TrustClaim{}); !errors.Is(err, validateErr) {
		t.Fatalf("PutRelation validate err = %v, want %v", err, validateErr)
	}

	previous := cryptorand.Reader
	cryptorand.Reader = directionErrReader{}
	err := (Repository{Ledger: &fakeLedger{}}).PutRelation(protocol.Relation{}, "durable", protocol.ActorRef{Kind: "agent"}, protocol.TrustClaim{Level: "agent_asserted"})
	cryptorand.Reader = previous
	if err == nil {
		t.Fatal("PutRelation succeeded when envelope creation failed")
	}

	putErr := errors.New("put failed")
	relation := protocol.Relation{
		RelationID: "rel_01978f71-79c7-7d53-a52a-cac034f15868",
		Type:       protocol.RelationInformedBy,
		From:       protocol.NodeRef{Kind: "record", ID: "a"},
		To:         protocol.NodeRef{Kind: "record", ID: "b"},
		CreatedAt:  time.Now().Format(time.RFC3339Nano),
	}
	if err := (Repository{Ledger: &fakeLedger{putErr: putErr}}).PutRelation(relation, "durable", protocol.ActorRef{Kind: "agent"}, protocol.TrustClaim{Level: "agent_asserted"}); !errors.Is(err, putErr) {
		t.Fatalf("PutRelation put err = %v, want %v", err, putErr)
	}
}

func TestRepositoryDirectionsReturnsListError(t *testing.T) {
	listErr := errors.New("list failed")
	if _, _, err := (Repository{Ledger: &fakeLedger{listErr: listErr}}).Directions(); !errors.Is(err, listErr) {
		t.Fatalf("Directions err = %v, want %v", err, listErr)
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

type fakeLedger struct {
	events       []protocol.EventEnvelope
	conflicts    []string
	listErr      error
	putErr       error
	putErrOnCall map[int]error
	putCalls     int
}

func (l fakeLedger) List() ([]protocol.EventEnvelope, []string, error) {
	return l.events, l.conflicts, l.listErr
}

func (l *fakeLedger) Put(event protocol.EventEnvelope, durable bool) error {
	l.putCalls++
	if err := l.putErrOnCall[l.putCalls]; err != nil {
		return err
	}
	if l.putErr != nil {
		return l.putErr
	}
	l.events = append(l.events, event)
	return nil
}

type directionErrReader struct{}

func (directionErrReader) Read([]byte) (int, error) {
	return 0, errors.New("random failed")
}
