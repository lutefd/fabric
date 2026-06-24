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
	if err := Run([]string{"continue", "--pr", "123"}); err == nil {
		t.Fatal("continue without current or explicit thread returned nil error")
	}

	mustRun(t, "continue", "--pr", "123", "--thread", "thread-pr")
	context := mustRead(t, continuePath)
	assertContains(t, context, "# Continuation Context")
	assertContains(t, context, "PR:\n123")
	assertContains(t, context, "No active review direction found.")
	if got := mustRead(t, currentThreadPath); got != "thread-pr\n" {
		t.Fatalf("current thread = %q, want thread-pr", got)
	}

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
	mustRun(t, "thread", "start", "--id", "thread-pr", "--pr", "123")
	mustRun(t, "note", "--issue", "VS-123", "--area", "file-opening", "This older issue direction is long enough to exceed the tiny continuation budget.")
	mustRun(t, "review", "note", "--pr", "123", "--issue", "VS-123", "--area", "file-opening", "Fix")
	mustRun(t, "continue", "--pr", "123", "--budget", "1")

	context := mustRead(t, continuePath)
	assertContains(t, context, "1. Fix")
	assertContains(t, context, budgetOmittedMessage)
	assertNotContains(t, context, "This older issue direction")

	threads, err := loadThreads()
	if err != nil {
		t.Fatal(err)
	}
	thread := threads["thread-pr"]
	if thread.Issue != "VS-123" || thread.PR != "123" || len(thread.Areas) != 1 || thread.Areas[0] != "file-opening" {
		t.Fatalf("continued current thread scope = %#v", thread)
	}
	if got := thread.LastSeenEventID; got != "evt_000002" {
		t.Fatalf("continued current thread last seen = %q, want evt_000002", got)
	}
}

func TestContinueExplicitThreadOverridesCurrentThread(t *testing.T) {
	chdirTemp(t)

	mustRun(t, "init")
	mustRun(t, "thread", "start", "--id", "thread-current", "--issue", "CUR-1")
	mustRun(t, "review", "note", "--pr", "123", "--issue", "FAB-1", "--area", "agent-protocol", "Carry review direction forward")

	mustRun(t, "continue", "--pr", "123", "--thread", "thread-followup")

	context := mustRead(t, continuePath)
	assertContains(t, context, "Carry review direction forward")
	if got := mustRead(t, currentThreadPath); got != "thread-followup\n" {
		t.Fatalf("current thread = %q, want thread-followup", got)
	}
	threads, err := loadThreads()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := threads["thread-current"]; !ok {
		t.Fatal("existing current thread was not preserved")
	}
	thread := threads["thread-followup"]
	if thread.Issue != "FAB-1" || thread.PR != "123" || len(thread.Areas) != 1 || thread.Areas[0] != "agent-protocol" {
		t.Fatalf("followup thread scope = %#v", thread)
	}
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
