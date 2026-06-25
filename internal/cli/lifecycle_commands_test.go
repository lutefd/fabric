package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lutefd/fabric/protocol"
)

func TestListFiltersAndScopeRendering(t *testing.T) {
	chdirTemp(t)
	mustRun(t, "init")
	mustRun(t, "thread", "start", "--id", "thread-a", "--issue", "FAB-1", "--pr", "8", "--area", "cli", "--path", "internal/cli/**")
	mustRun(t, "note", "--candidate", "--thread", "thread-a", "Scoped candidate")
	mustRun(t, "note", "--live", "--global", "Global live")

	for _, args := range [][]string{
		{"list", "extra"},
		{"list", "--durability", "forever"},
		{"list", "--status", "pending"},
	} {
		if err := Run(args); err == nil {
			t.Fatalf("Run(%v) succeeded", args)
		}
	}

	scoped := captureStdout(t, func() {
		mustRun(t, "list", "--status", "any", "--pr", "8", "--area", "cli", "--path", "internal/cli/app.go")
	})
	assertContains(t, scoped, "Scoped candidate")
	assertContains(t, scoped, "scope: issue=FAB-1 pr=8 areas=cli paths=internal/cli/**")

	none := captureStdout(t, func() {
		mustRun(t, "list", "--durability", "durable")
	})
	assertContains(t, none, "No matching direction.")
}

func TestLifecycleCommandValidationAndReports(t *testing.T) {
	chdirTemp(t)

	for _, args := range [][]string{
		{"expire"},
		{"discard"},
		{"keep"},
		{"consolidate"},
		{"consolidate", "--pr", "8", "--issue", "FAB-1"},
	} {
		if err := Run(args); err == nil {
			t.Fatalf("Run(%v) succeeded before init", args)
		}
	}

	mustRun(t, "init")
	for _, args := range [][]string{
		{"discard", "rec_missing"},
		{"discard", "rec_missing", "--reason", "duplicate"},
		{"keep", "rec_missing"},
		{"keep", "rec_missing", "--candidate"},
		{"expire", "rec_missing"},
	} {
		if err := Run(args); err == nil {
			t.Fatalf("Run(%v) succeeded", args)
		}
	}

	mustRun(t, "note", "--durable", "--issue", "FAB-1", "Durable direction")
	durableID := recordIDAt(t, 0)
	if err := Run([]string{"expire", durableID}); err == nil {
		t.Fatal("durable event expired without --force")
	}
	expired := captureStdout(t, func() {
		mustRun(t, "expire", durableID, "--force", "--reason", "replaced")
	})
	assertContains(t, expired, "Reason: replaced")

	mustRun(t, "note", "--candidate", "--issue", "FAB-1", "Candidate direction")
	candidateID := recordIDByText(t, "Candidate direction")
	kept := captureStdout(t, func() {
		mustRun(t, "keep", candidateID, "--candidate", "--reason", "needs more evidence")
	})
	assertContains(t, kept, "Kept")

	mustRun(t, "note", "--candidate", "--issue", "FAB-2", "Discard me")
	discardID := recordIDByText(t, "Discard me")
	discarded := captureStdout(t, func() {
		mustRun(t, "discard", discardID, "--reason", "too narrow")
	})
	assertContains(t, discarded, "Reason: too narrow")

	mustRun(t, "review", "note", "--pr", "8", "--issue", "FAB-1", "--rejects", "old path", "--prefer", "new path", "--reason", "reviewer said so", "Review direction")
	mustRun(t, "note", "--live", "--kind", "review_requirement", "--pr", "8", "--issue", "FAB-1", "--reason", "CI asks for it", "Add a CLI test")
	mustRun(t, "consolidate", "--issue", "FAB-1")
	report := mustRead(t, consolidationPath)
	assertContains(t, report, "Durable directions already kept")
	assertContains(t, report, "Candidate directions to review")
	assertContains(t, report, "Live directions likely to expire")
	assertContains(t, report, "Why likely temporary")
}

func TestConsolidationReportClassifiesAllLifecycleShapes(t *testing.T) {
	events := []DirectionEvent{
		{ID: "durable-active", Durability: DurabilityDurable, Status: StatusActive, Text: "Durable"},
		{ID: "durable-expired", Durability: DurabilityDurable, Status: StatusExpired, Text: "Old durable"},
		{ID: "candidate-review", Durability: DurabilityCandidate, Kind: "review_direction", ReviewType: "preference", PreferredPaths: []string{"new path"}, Status: StatusActive, Text: "Prefer new path"},
		{ID: "candidate-requirement", Durability: DurabilityCandidate, Kind: "review_requirement", Status: StatusActive, Text: "Add tests"},
		{ID: "candidate-task", Durability: DurabilityCandidate, Status: StatusActive, Text: "Checklist item"},
		{ID: "candidate-note", Durability: DurabilityCandidate, Status: StatusActive, Text: "Reusable direction"},
		{ID: "candidate-expired", Durability: DurabilityCandidate, Status: StatusExpired, Text: "Old candidate"},
		{ID: "live-review", Durability: DurabilityLive, Kind: "review_direction", Status: StatusActive, Text: "Review says no"},
		{ID: "live-requirement", Durability: DurabilityLive, Kind: "review_requirement", Status: StatusActive, Text: "Run checklist"},
		{ID: "live-task", Durability: DurabilityLive, Status: StatusActive, Text: "Add tests for parser"},
		{ID: "live-note", Durability: DurabilityLive, Status: StatusActive, Text: "Temporary note"},
		{ID: "challenge", Durability: DurabilityLive, Kind: "challenge", Status: "open", Text: "Challenge"},
		{ID: "live-expired", Durability: DurabilityLive, Status: StatusExpired, Text: "Old live"},
	}
	scope := consolidationScope{IsPR: true, PR: "8", Issue: "FAB-1"}
	if !matchesConsolidationScope(DirectionEvent{Scope: EventScope{Issue: "FAB-1"}}, scope) {
		t.Fatal("PR scope did not match inferred issue")
	}
	report := buildConsolidationReport(scope, events)
	if len(report.DurableActive) != 1 || len(report.CandidateActive) != 4 || len(report.LiveActive) != 4 || len(report.OpenChallenges) != 1 || len(report.Inactive) != 3 {
		t.Fatalf("unexpected report: %#v", report)
	}
	md := consolidationMarkdown(report)
	for _, want := range []string{
		"Why it may be durable",
		"It captures rejected or preferred paths",
		"It is a PR-local checklist or test item.",
		"It is a task-local checklist item.",
		"resolve challenge challenge before consolidating as durable",
	} {
		assertContains(t, md, want)
	}
	empty := consolidationMarkdown(consolidationReport{Scope: consolidationScope{IsIssue: true, Issue: "FAB-2"}})
	assertContains(t, empty, "No cleanup actions suggested.")
}

func TestContinuationAndHandoffValidation(t *testing.T) {
	chdirTemp(t)
	for _, args := range [][]string{
		{"continue"},
		{"continue", "--issue", "FAB-1", "--budget", "0"},
		{"handoff"},
	} {
		if err := Run(args); err == nil {
			t.Fatalf("Run(%v) succeeded before init", args)
		}
	}

	mustRun(t, "init")
	mustRun(t, "thread", "start", "--id", "thread-pr", "--pr", "8", "--issue", "FAB-1", "--area", "cli")
	mustRun(t, "review", "note", "--pr", "8", "--issue", "FAB-1", "--rejects", "legacy-only tests", "--prefer", "behavior tests", "--reason", "reliability", "Prefer behavior tests")
	mustRun(t, "note", "--live", "--kind", "review_requirement", "--pr", "8", "--issue", "FAB-1", "--reason", "coverage gate", "Exercise CLI edge cases")
	directionID := recordIDAt(t, 0)
	mustRun(t, "challenge", "--direction", directionID, "--pr", "8", "--issue", "FAB-1", "--proposal", "Keep the legacy shape", "--reason", "compatibility")

	out := captureStdout(t, func() {
		mustRun(t, "continue", "--pr", "8", "--thread", "thread-next", "--budget", "40")
	})
	assertContains(t, out, "# Continuation Context")
	assertContains(t, out, "Some relevant direction was omitted")

	if err := Run([]string{"handoff"}); err == nil {
		t.Fatal("handoff without --pr succeeded")
	}
	handoff := captureStdout(t, func() {
		mustRun(t, "handoff", "--pr", "8")
	})
	assertContains(t, handoff, "Wrote .fabric/generated/HANDOFF.md")
	md := mustRead(t, handoffPath)
	assertContains(t, md, "Do not reopen")
	assertContains(t, md, "Keep the legacy shape")
}

func TestRuntimeStorePersistenceAndProtocolErrors(t *testing.T) {
	chdirTemp(t)
	mustRun(t, "init")

	if got := runtimeKindForEvent("unknown.event"); got != "" {
		t.Fatalf("unknown runtime event kind = %q", got)
	}
	if err := appendRuntimeEnvelope(runtimeThreads, protocol.EventEnvelope{}); err == nil {
		t.Fatal("invalid runtime envelope was accepted")
	}

	thread := ThreadRecord{ThreadID: "thread-a", Issue: "FAB-1", Areas: []string{"cli"}}
	if err := saveRuntimeThread(thread, protocol.EventThreadStarted); err != nil {
		t.Fatal(err)
	}
	threads, err := loadRuntimeThreads()
	if err != nil {
		t.Fatal(err)
	}
	if threads["thread-a"].Issue != "FAB-1" {
		t.Fatalf("thread did not round-trip: %#v", threads)
	}

	eventID, err := protocol.NewEventID()
	if err != nil {
		t.Fatal(err)
	}
	recordID, err := protocol.NewRecordID()
	if err != nil {
		t.Fatal(err)
	}
	event := DirectionEvent{ID: recordID, HeadEventID: eventID, Status: StatusActive, Scope: EventScope{Issue: "FAB-1", Areas: []string{"cli"}}, Text: "Direction"}
	projection, err := createProjection("sync", "thread-a", protocol.Scope{Issue: "FAB-1", Areas: []string{"cli"}}, []DirectionEvent{event}, true)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := recordProjectionReceipt(projection, protocol.ReceiptExposed, "codex")
	if err != nil {
		t.Fatal(err)
	}
	if receipt.State != protocol.ReceiptExposed {
		t.Fatalf("receipt state = %q", receipt.State)
	}
	if _, _, err := deliveredForThread("thread-a"); err != nil {
		t.Fatal(err)
	}

	if _, err := loadProjection("prj_missing"); err == nil {
		t.Fatal("missing projection loaded")
	}

	runtimeRoot, err := sharedRuntimeRoot()
	if err != nil {
		t.Fatal(err)
	}
	badThreadDir := filepath.Join(runtimeRoot, runtimeThreads)
	if err := os.WriteFile(filepath.Join(badThreadDir, "bad.json"), []byte(`{"payload":{`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadRuntimeThreads(); err == nil {
		t.Fatal("malformed runtime thread event loaded")
	}
}

func TestMarkdownIngestAndMatchingBranches(t *testing.T) {
	events := []DirectionEvent{
		{ID: "human", Source: EventSource{Type: "human", ThreadID: "thread-a"}},
		{ID: "review", Source: EventSource{Type: "review", ThreadID: "thread-b"}},
		{ID: "tool", Source: EventSource{Type: "tool", ThreadID: "thread-c"}},
		{ID: "agent", Source: EventSource{Type: "agent", ThreadID: "thread-d"}},
		{ID: "unknown", Source: EventSource{}},
	}
	for _, event := range events {
		if projectedSource(event) == "" {
			t.Fatalf("empty projected source for %#v", event)
		}
	}

	md := continuationMarkdown("FAB-1", "8", []DirectionEvent{
		{ID: "review", Kind: "review_direction", Text: "Use shared parser", Reason: "consistency", RejectedPaths: []string{"ad hoc"}, PreferredPaths: []string{"shared helper"}},
		{ID: "requirement", Kind: "review_requirement", Text: "Add tests", Reason: "review"},
		{ID: "direction", Kind: "note", Text: "Keep behavior"},
		{ID: "resolution", Kind: "challenge_resolution", Text: "Rejected challenge", Challenges: "challenge", Status: "rejected"},
	}, false)
	for _, want := range []string{"Rejected paths", "Preferred paths", "Resolved challenge", "Active issue direction"} {
		assertContains(t, md, want)
	}

	graphOut := captureStdout(t, func() {
		_ = printExplain("", []string{"cli"}, []DirectionEvent{{ID: "rec-1", Text: "Direction", Status: "open", Scope: EventScope{Areas: []string{"cli"}}}}, map[string]ThreadRecord{})
	})
	assertContains(t, graphOut, "Challenge state:")

	if _, err := parseIngestFile("# Fabric PR Review Ingest\n\n## Review directions\n\n### Direction 1\nReview says:\n"); err == nil {
		t.Fatal("empty ingest item was accepted")
	}
	ingest, err := parseIngestFile(strings.Join([]string{
		"# Fabric PR Review Ingest",
		"PR: 8",
		"Issue: FAB-1",
		"Areas:",
		"- cli",
		"notes leave areas",
		"## Review directions",
		"### Direction 1",
		"Type: test_request",
		"Durability: unknown",
		"Review says: Add tests",
		"Evidence:",
		"- reviewer comment: please test this",
		"- url: https://example.invalid/review",
		"- plain note",
	}, "\n"))
	if err != nil {
		t.Fatal(err)
	}
	if inferIngestDurability(ingest.Items[0]) != DurabilityLive || inferIngestKind(ingest.Items[0]) != "review_requirement" {
		t.Fatalf("unexpected ingest inference: %#v", ingest.Items[0])
	}
	if len(ingest.Items[0].Evidence) != 3 {
		t.Fatalf("evidence = %#v", ingest.Items[0].Evidence)
	}

	ordered := prioritizedEvents([]DirectionEvent{
		{ID: "ordinary", Kind: "note"},
		{ID: "requirement", Kind: "review_requirement"},
	}, "", "", nil)
	if ordered[0].ID != "requirement" {
		t.Fatalf("review requirement priority changed: %#v", ordered)
	}
}

func TestStorageAndRepositoryErrorBranches(t *testing.T) {
	chdirTemp(t)
	if _, err := os.Stat(configPath); err == nil {
		t.Fatal("temp repo unexpectedly initialized")
	}
	if _, err := loadCurrentThreadID(); err == nil {
		t.Fatal("loadCurrentThreadID succeeded before init")
	}
	if _, err := resolveThreadID(""); err == nil {
		t.Fatal("resolveThreadID without current thread succeeded")
	}

	mustRun(t, "init")
	if err := os.WriteFile(currentThreadPath, []byte("ghost\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, err := loadCurrentThreadID(); err != nil || got != "ghost" {
		t.Fatalf("current thread = %q err=%v", got, err)
	}

	if _, err := promoteEvent("rec_missing", "reason"); err == nil {
		t.Fatal("missing event promoted")
	}
	mustRun(t, "note", "--durable", "--global", "Durable direction")
	id := recordIDAt(t, 0)
	if _, err := promoteEvent(id, "reason"); err == nil {
		t.Fatal("durable event promoted again")
	}

	report, err := ledgerHealth()
	if err != nil {
		t.Fatal(err)
	}
	if report.Counts[DurabilityDurable] == 0 {
		t.Fatalf("ledger report did not count durable event: %#v", report)
	}
}

func TestMalformedRuntimeFilesReturnErrors(t *testing.T) {
	chdirTemp(t)
	mustRun(t, "init")
	root, err := sharedRuntimeRoot()
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		kind string
		run  func() error
	}{
		{runtimeReceipts, func() error { _, err := loadReceipts(); return err }},
		{runtimeProjections, func() error { _, err := loadProjection("anything"); return err }},
	} {
		dir := filepath.Join(root, tc.kind)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "bad.json"), []byte(`{"payload":{`), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := tc.run(); err == nil {
			t.Fatalf("malformed %s file did not error", tc.kind)
		}
		if err := os.Remove(filepath.Join(dir, "bad.json")); err != nil {
			t.Fatal(err)
		}
	}
}

func recordIDByText(t *testing.T, text string) string {
	t.Helper()
	events, err := loadEvents()
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range events {
		if event.Text == text {
			return event.ID
		}
	}
	t.Fatalf("record with text %q not found in %#v", text, events)
	return ""
}
