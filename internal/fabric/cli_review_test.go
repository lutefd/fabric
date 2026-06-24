package fabric

import "testing"

func TestReviewNoteValidationAndStructuredText(t *testing.T) {
	chdirTemp(t)

	if err := Run([]string{"review", "note", "--pr", "123", "before init"}); err == nil {
		t.Fatal("review note before init returned nil error")
	}
	mustRun(t, "init")
	if err := Run([]string{"review"}); err == nil {
		t.Fatal("review without note returned nil error")
	}
	if err := Run([]string{"review", "note", "--wat"}); err == nil {
		t.Fatal("review note accepted unknown flag")
	}
	if err := Run([]string{"review", "note", "missing pr"}); err == nil {
		t.Fatal("review note without pr returned nil error")
	}
	if err := Run([]string{"review", "note", "--pr", "123"}); err == nil {
		t.Fatal("empty review note returned nil error")
	}

	mustRun(t, "review", "note", "--pr", "123", "--issue", "VS-123", "--area", "file-opening", "--rejects", "picker-level Office special-casing", "--prefer", "shared file-open resolver", "--reason", "consistent behavior")
	events, err := loadEvents()
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("events count = %d, want 1", len(events))
	}
	event := events[0]
	if event.Kind != "review_direction" || event.Scope.PR != "123" || event.Source.Type != "review" {
		t.Fatalf("review event = %#v", event)
	}
	assertContains(t, event.Text, "Reviewer rejected picker-level Office special-casing.")
	assertContains(t, event.Text, "Preferred path: shared file-open resolver.")
	assertContains(t, event.Text, "Reason: consistent behavior.")
	if event.Confidence != "reviewer_confirmed" || event.TTL != "until_pr_closed" {
		t.Fatalf("review event lifecycle fields = %#v", event)
	}
}

func TestExplainPRShowsReviewDirection(t *testing.T) {
	chdirTemp(t)

	mustRun(t, "init")
	noMatches := captureStdout(t, func() {
		mustRun(t, "explain", "--pr", "123")
	})
	assertContains(t, noMatches, "No active direction found.")

	mustRun(t, "thread", "start", "--id", "thread-review-fix", "--issue", "VS-123", "--area", "file-opening")
	mustRun(t, "review", "note", "--pr", "123", "--issue", "VS-123", "--area", "file-opening", "Reviewer rejected picker-level Office special-casing; move unsupported file handling into the shared file-open resolver.")

	output := captureStdout(t, func() {
		mustRun(t, "explain", "--pr", "123")
	})
	assertContains(t, output, "Active direction for PR 123")
	assertContains(t, output, "Kind:\nreview_direction")
	assertContains(t, output, "pr: 123")
	assertContains(t, output, "issue: VS-123")
	assertContains(t, output, "area: file-opening")
	assertContains(t, output, "Source:\nreview")
	assertContains(t, output, "Stale:\n- thread-review-fix")
}
