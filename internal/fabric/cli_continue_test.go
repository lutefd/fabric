package fabric

import "testing"

func TestContinueValidationAndNoActiveDirection(t *testing.T) {
	chdirTemp(t)

	mustRun(t, "init")
	if err := Run([]string{"continue", "--wat"}); err == nil {
		t.Fatal("continue accepted unknown flag")
	}
	if err := Run([]string{"continue"}); err == nil {
		t.Fatal("continue without scope returned nil error")
	}
	if err := Run([]string{"continue", "--pr", "123", "--budget", "0"}); err == nil {
		t.Fatal("continue accepted non-positive budget")
	}

	mustRun(t, "continue", "--pr", "123")
	context := mustRead(t, continuePath)
	assertContains(t, context, "# Continuation Context")
	assertContains(t, context, "PR:\n123")
	assertContains(t, context, "No active review direction found.")

	mustRun(t, "continue", "--issue", "VS-123", "--thread", "thread-empty")
	threads, err := loadThreads()
	if err != nil {
		t.Fatal(err)
	}
	if got := threads["thread-empty"].LastSeenEventID; got != "" {
		t.Fatalf("empty continuation last seen = %q, want empty", got)
	}
}

func TestContinueBudgetPrioritizesReviewDirection(t *testing.T) {
	chdirTemp(t)

	mustRun(t, "init")
	mustRun(t, "note", "--issue", "VS-123", "--area", "file-opening", "This older issue direction is long enough to exceed the tiny continuation budget.")
	mustRun(t, "review", "note", "--pr", "123", "--issue", "VS-123", "--area", "file-opening", "Fix")
	mustRun(t, "continue", "--pr", "123", "--budget", "1")

	context := mustRead(t, continuePath)
	assertContains(t, context, "1. Fix")
	assertContains(t, context, budgetOmittedMessage)
	assertNotContains(t, context, "This older issue direction")
}

func TestInferScopeFromPRKeepsExplicitScopeAndSkipsEmptyOrDuplicateAreas(t *testing.T) {
	events := []DirectionEvent{
		{Scope: EventScope{PR: "999", Issue: "OTHER", Areas: []string{"other"}}},
		{Scope: EventScope{PR: "123", Issue: "VS-123", Areas: []string{"", "file-opening", "explicit-area"}}},
	}

	issue, areas := inferScopeFromPR(events, "123", "EXPLICIT-1", []string{"explicit-area"})
	if issue != "EXPLICIT-1" {
		t.Fatalf("issue = %q, want explicit issue", issue)
	}
	if len(areas) != 2 || areas[0] != "explicit-area" || areas[1] != "file-opening" {
		t.Fatalf("areas = %#v, want explicit-area then file-opening", areas)
	}
}
