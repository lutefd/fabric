package cli

import (
	"strings"
	"testing"
)

func TestConsolidateGroupsCandidatesLiveAndDurableDirections(t *testing.T) {
	chdirTemp(t)

	mustRun(t, "init")
	mustRun(t, "note", "--durable", "--issue", "VS-123", "Durable project direction")
	mustRun(t, "review", "note", "--pr", "123", "--issue", "VS-123", "Candidate review direction")
	mustRun(t, "note", "--live", "--kind", "review_requirement", "--pr", "123", "--issue", "VS-123", "Add tests for the parser")

	mustRun(t, "consolidate", "--pr", "123")

	consolidation := mustRead(t, consolidationPath)
	assertContains(t, consolidation, "## Durable directions already kept")
	assertContains(t, consolidation, "## Candidate directions to review")
	assertContains(t, consolidation, "## Live directions likely to expire")
	assertContains(t, consolidation, "Candidate review direction")
	assertContains(t, consolidation, "Add tests for the parser")
	assertContains(t, consolidation, "fabric promote")
	assertContains(t, consolidation, "fabric expire")
}

func TestExpireRemovesEventFromActiveContext(t *testing.T) {
	chdirTemp(t)

	mustRun(t, "init")
	mustRun(t, "thread", "start", "--id", "thread-a", "--pr", "123", "--issue", "VS-123")
	mustRun(t, "review", "note", "--pr", "123", "--issue", "VS-123", "Temporary requirement")

	events, err := loadEvents()
	if err != nil {
		t.Fatal(err)
	}
	liveEvent := events[len(events)-1]

	mustRun(t, "expire", liveEvent.ID, "--reason", "done")

	mustRun(t, "continue", "--pr", "123", "--thread", "thread-b")
	context := mustRead(t, continuePath)
	assertNotContains(t, context, "Temporary requirement")

	mustRun(t, "consolidate", "--pr", "123", "--include-inactive")
	consolidation := mustRead(t, consolidationPath)
	assertContains(t, consolidation, "Temporary requirement")
	assertContains(t, consolidation, "expired")
}

func TestDiscardRemovesCandidateFromPreflightAndContinue(t *testing.T) {
	chdirTemp(t)

	mustRun(t, "init")
	mustRun(t, "thread", "start", "--id", "thread-a", "--issue", "VS-123")
	mustRun(t, "note", "--candidate", "--issue", "VS-123", "Noisy candidate")

	events, err := loadEvents()
	if err != nil {
		t.Fatal(err)
	}
	candidate := events[len(events)-1]

	mustRun(t, "discard", candidate.ID, "--reason", "too specific")

	mustRun(t, "preflight", "task", "--issue", "VS-123")
	direction := mustRead(t, taskPath)
	assertNotContains(t, direction, "Noisy candidate")

	mustRun(t, "continue", "--issue", "VS-123", "--thread", "thread-b")
	context := mustRead(t, continuePath)
	assertNotContains(t, context, "Noisy candidate")
}

func TestPromoteRecordsReviewReason(t *testing.T) {
	chdirTemp(t)

	mustRun(t, "init")
	mustRun(t, "note", "--candidate", "--issue", "VS-123", "Promote me")

	events, err := loadEvents()
	if err != nil {
		t.Fatal(err)
	}
	candidate := events[len(events)-1]

	mustRun(t, "promote", candidate.ID, "--reason", "Reusable review-ingest product direction")

	events, err = loadEvents()
	if err != nil {
		t.Fatal(err)
	}
	promoted := events[len(events)-1]
	if promoted.Durability != DurabilityDurable {
		t.Fatalf("durability = %q, want durable", promoted.Durability)
	}
	if promoted.ReviewedAt == "" {
		t.Fatal("reviewed_at is empty")
	}
	if !strings.Contains(promoted.LifecycleReason, "Reusable review-ingest product direction") {
		t.Fatalf("lifecycle_reason = %q, want reason", promoted.LifecycleReason)
	}
}

func TestConsolidateHighlightsOpenChallenges(t *testing.T) {
	chdirTemp(t)

	mustRun(t, "init")
	mustRun(t, "note", "--durable", "--issue", "VS-123", "Existing direction")
	directionID := recordIDAt(t, 0)
	mustRun(t, "challenge", "--direction", directionID, "--pr", "123", "--issue", "VS-123", "--proposal", "New path", "--reason", "why")

	mustRun(t, "consolidate", "--pr", "123")

	consolidation := mustRead(t, consolidationPath)
	assertContains(t, consolidation, "## Open challenges")
	assertContains(t, consolidation, "New path")
	assertNotContains(t, consolidation, "fabric promote "+recordIDAt(t, 1))
}

func TestKeepCandidateMarksReviewedButStillCandidate(t *testing.T) {
	chdirTemp(t)

	mustRun(t, "init")
	mustRun(t, "note", "--candidate", "--issue", "VS-123", "Keep me")

	events, err := loadEvents()
	if err != nil {
		t.Fatal(err)
	}
	candidate := events[len(events)-1]

	mustRun(t, "keep", candidate.ID, "--candidate", "--reason", "needs more evidence")

	events, err = loadEvents()
	if err != nil {
		t.Fatal(err)
	}
	kept := events[len(events)-1]
	if kept.Durability != DurabilityCandidate {
		t.Fatalf("durability = %q, want candidate", kept.Durability)
	}
	if kept.Status != StatusActive {
		t.Fatalf("status = %q, want active", kept.Status)
	}
	if kept.ReviewedAt == "" {
		t.Fatal("reviewed_at is empty")
	}
}

func TestExpireDurableRequiresForce(t *testing.T) {
	chdirTemp(t)

	mustRun(t, "init")
	mustRun(t, "note", "--durable", "--issue", "VS-123", "Durable direction")

	events, err := loadEvents()
	if err != nil {
		t.Fatal(err)
	}
	durable := events[len(events)-1]

	if err := Run([]string{"expire", durable.ID, "--reason", "done"}); err == nil {
		t.Fatal("expire durable without force returned nil error")
	}

	mustRun(t, "expire", durable.ID, "--force", "--reason", "superseded")

	events, err = loadEvents()
	if err != nil {
		t.Fatal(err)
	}
	expired := events[len(events)-1]
	if expired.Status != StatusExpired {
		t.Fatalf("status = %q, want expired", expired.Status)
	}
}
