package fabric

import (
	"os"
	"strings"
	"testing"
)

func TestIngestPRCreatesCandidateReviewDirection(t *testing.T) {
	chdirTemp(t)

	mustRun(t, "init")
	mustRun(t, "thread", "start", "--id", "thread-a", "--issue", "FAB-3", "--area", "review-ingest")

	review := `# Fabric PR Review Ingest

PR: 123
Issue: FAB-3
Areas:
- review-ingest

## Review directions

### Direction 1

Type: rejection
Durability: candidate

Review says:
Do not turn every PR comment into durable direction.

Rejected paths:
- Automatically promoting all review comments to durable project memory
- Replaying the full PR thread into future agents

Preferred paths:
- Create candidate review directions
- Let humans promote after consolidation

Reason:
Most review comments are task-specific or noisy.

Evidence:
- reviewer comment: "This should be candidate direction, not permanent memory."
`
	if err := os.WriteFile("review.md", []byte(review), 0o644); err != nil {
		t.Fatal(err)
	}

	mustRun(t, "ingest-pr", "--pr", "123", "--issue", "FAB-3", "--area", "review-ingest", "--from-file", "review.md")

	events, err := loadEvents()
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("events count = %d, want 1", len(events))
	}
	event := events[0]
	if event.Kind != "review_direction" {
		t.Fatalf("kind = %q, want review_direction", event.Kind)
	}
	if event.Durability != DurabilityCandidate {
		t.Fatalf("durability = %q, want candidate", event.Durability)
	}
	if event.Scope.PR != "123" || event.Scope.Issue != "FAB-3" || len(event.Scope.Areas) != 1 || event.Scope.Areas[0] != "review-ingest" {
		t.Fatalf("scope = %#v", event.Scope)
	}
	if event.ReviewType != "rejection" {
		t.Fatalf("review_type = %q, want rejection", event.ReviewType)
	}
	if len(event.RejectedPaths) != 2 || len(event.PreferredPaths) != 2 {
		t.Fatalf("rejected=%d preferred=%d, want 2/2", len(event.RejectedPaths), len(event.PreferredPaths))
	}
	if len(event.Evidence) != 1 {
		t.Fatalf("evidence count = %d, want 1", len(event.Evidence))
	}
}

func TestIngestPRRequirementDefaultsToLive(t *testing.T) {
	chdirTemp(t)

	mustRun(t, "init")
	review := `# Fabric PR Review Ingest

PR: 123
Issue: FAB-3

## Review directions

### Direction 1

Type: requirement

Review says:
Add tests for ingest parser.

Reason:
Parser should reject malformed files.
`
	if err := os.WriteFile("review.md", []byte(review), 0o644); err != nil {
		t.Fatal(err)
	}

	mustRun(t, "ingest-pr", "--pr", "123", "--issue", "FAB-3", "--from-file", "review.md")

	events, err := loadEvents()
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("events count = %d, want 1", len(events))
	}
	if events[0].Kind != "review_requirement" || events[0].Durability != DurabilityLive {
		t.Fatalf("event = %#v", events[0])
	}
}

func TestIngestPRFeedsContinueContext(t *testing.T) {
	chdirTemp(t)

	mustRun(t, "init")
	review := `# Fabric PR Review Ingest

PR: 123
Issue: FAB-3

## Review directions

### Direction 1

Type: rejection

Review says:
Do not special-case Office files in the picker.

Rejected paths:
- picker-level Office special casing

Preferred paths:
- shared file-open resolver

Reason:
Consistency.
`
	if err := os.WriteFile("review.md", []byte(review), 0o644); err != nil {
		t.Fatal(err)
	}

	mustRun(t, "ingest-pr", "--pr", "123", "--issue", "FAB-3", "--from-file", "review.md")
	mustRun(t, "thread", "start", "--id", "thread-continue", "--pr", "123")
	mustRun(t, "continue", "--pr", "123")

	context := mustRead(t, continuePath)
	assertContains(t, context, "Current review direction:")
	assertContains(t, context, "Do not special-case Office files in the picker.")
	assertContains(t, context, "Rejected paths:")
	assertContains(t, context, "- picker-level Office special casing")
	assertContains(t, context, "Preferred paths:")
	assertContains(t, context, "- shared file-open resolver")
	assertContains(t, context, "Reason:")
}

func TestHandoffIncludesReviewDirectionsAndLiveRequirements(t *testing.T) {
	chdirTemp(t)

	mustRun(t, "init")
	review := `# Fabric PR Review Ingest

PR: 123
Issue: FAB-3

## Review directions

### Direction 1

Type: rejection

Review says:
Do not special-case Office files.

Rejected paths:
- picker-level special casing

Preferred paths:
- shared resolver

### Direction 2

Type: requirement

Review says:
Add parser tests.
`
	if err := os.WriteFile("review.md", []byte(review), 0o644); err != nil {
		t.Fatal(err)
	}

	mustRun(t, "ingest-pr", "--pr", "123", "--issue", "FAB-3", "--from-file", "review.md")
	mustRun(t, "handoff", "--pr", "123")

	handoff := mustRead(t, handoffPath)
	assertContains(t, handoff, "Current review direction:")
	assertContains(t, handoff, "Do not special-case Office files.")
	assertContains(t, handoff, "Active live requirements:")
	assertContains(t, handoff, "Add parser tests.")
	assertContains(t, handoff, "Do not reopen:")
	assertContains(t, handoff, "- picker-level special casing")
}

func TestIngestPRDryRunDoesNotWriteEvents(t *testing.T) {
	chdirTemp(t)

	mustRun(t, "init")
	review := `# Fabric PR Review Ingest

PR: 123
Issue: FAB-3

## Review directions

### Direction 1

Type: rejection

Review says:
Do not write this event.
`
	if err := os.WriteFile("review.md", []byte(review), 0o644); err != nil {
		t.Fatal(err)
	}

	output := captureStdout(t, func() {
		mustRun(t, "ingest-pr", "--pr", "123", "--issue", "FAB-3", "--from-file", "review.md", "--dry-run")
	})
	assertContains(t, output, "Would create events:")

	events, err := loadEvents()
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("dry run wrote %d events", len(events))
	}
}

func TestIngestPRReportsMalformedItems(t *testing.T) {
	chdirTemp(t)

	mustRun(t, "init")
	review := `# Fabric PR Review Ingest

PR: 123
Issue: FAB-3

## Review directions

### Direction 1

Type: rejection

### Direction 2

Type: requirement

Review says:
Valid requirement.
`
	if err := os.WriteFile("review.md", []byte(review), 0o644); err != nil {
		t.Fatal(err)
	}

	output := captureStdout(t, func() {
		mustRun(t, "ingest-pr", "--pr", "123", "--issue", "FAB-3", "--from-file", "review.md")
	})
	assertContains(t, output, "Warning: skipped direction item with no 'Review says'")

	events, err := loadEvents()
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("events count = %d, want 1", len(events))
	}
}

func TestIngestPRMarksRelatedThreadsStale(t *testing.T) {
	chdirTemp(t)

	mustRun(t, "init")
	mustRun(t, "thread", "start", "--id", "thread-a", "--pr", "123")
	review := `# Fabric PR Review Ingest

PR: 123

## Review directions

### Direction 1

Type: rejection

Review says:
New review direction.
`
	if err := os.WriteFile("review.md", []byte(review), 0o644); err != nil {
		t.Fatal(err)
	}

	mustRun(t, "ingest-pr", "--pr", "123", "--issue", "FAB-3", "--from-file", "review.md")

	status := captureStdout(t, func() {
		mustRun(t, "status")
	})
	assertContains(t, status, "1 new relevant direction available.")
}

func TestIngestPRRequiresPRAndIssueOrArea(t *testing.T) {
	chdirTemp(t)

	mustRun(t, "init")
	if err := Run([]string{"ingest-pr"}); err == nil {
		t.Fatal("ingest-pr without args returned nil error")
	}
	if err := Run([]string{"ingest-pr", "--pr", "123"}); err == nil {
		t.Fatal("ingest-pr without issue/area returned nil error")
	}
	if err := Run([]string{"ingest-pr", "--pr", "123", "--issue", "FAB-3"}); err == nil {
		t.Fatal("ingest-pr without input source returned nil error")
	}
}

func TestIngestPRTemplateWritesFile(t *testing.T) {
	chdirTemp(t)

	mustRun(t, "init")
	mustRun(t, "ingest-pr", "template", "--pr", "123", "--issue", "FAB-3", "--area", "review-ingest")

	content := mustRead(t, ingestTemplatePath)
	assertContains(t, content, "PR: 123")
	assertContains(t, content, "Issue: FAB-3")
	assertContains(t, content, "- review-ingest")
	assertContains(t, content, "## Review directions")
}

func TestIngestPRRejectsDurableWithoutAllowDurable(t *testing.T) {
	chdirTemp(t)

	mustRun(t, "init")
	review := `# Fabric PR Review Ingest

PR: 123
Issue: FAB-3

## Review directions

### Direction 1

Type: rejection
Durability: durable

Review says:
Should not be allowed by default.
`
	if err := os.WriteFile("review.md", []byte(review), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Run([]string{"ingest-pr", "--pr", "123", "--issue", "FAB-3", "--from-file", "review.md"}); err == nil {
		t.Fatal("ingest-pr accepted durable without --allow-durable")
	}

	mustRun(t, "ingest-pr", "--pr", "123", "--issue", "FAB-3", "--from-file", "review.md", "--allow-durable")
	events, err := loadEvents()
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Durability != DurabilityDurable {
		t.Fatalf("event = %#v", events[0])
	}
}

func TestIngestPRFromStdin(t *testing.T) {
	chdirTemp(t)

	mustRun(t, "init")
	review := `# Fabric PR Review Ingest

PR: 123
Issue: FAB-3

## Review directions

### Direction 1

Type: requirement

Review says:
From stdin.
`

	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin = r
	go func() {
		_, _ = w.WriteString(review)
		_ = w.Close()
	}()
	defer func() { os.Stdin = oldStdin }()

	mustRun(t, "ingest-pr", "--pr", "123", "--issue", "FAB-3", "--stdin")

	events, err := loadEvents()
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || !strings.Contains(events[0].Text, "From stdin.") {
		t.Fatalf("event = %#v", events[0])
	}
}
