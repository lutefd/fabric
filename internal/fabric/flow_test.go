package fabric

import "testing"

func TestDirectionPacketSyncsAcrossThreads(t *testing.T) {
	chdirTemp(t)

	mustRun(t, "init")
	mustRun(t, "thread", "start", "--id", "thread-a", "--issue", "VS-123", "--area", "virtual-store/listing")
	mustRun(t, "thread", "start", "--id", "thread-b", "--issue", "VS-123", "--area", "virtual-store/listing")
	mustRun(t, "note", "--thread", "thread-a", "--issue", "VS-123", "--area", "virtual-store/listing", "Don't create a second listing endpoint; extend the existing one or escalate API direction")

	events, err := loadEvents()
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].ID != "evt_000001" {
		t.Fatalf("events = %#v, want one evt_000001", events)
	}

	threads, err := loadThreads()
	if err != nil {
		t.Fatal(err)
	}
	if got := threads["thread-a"].LastSeenEventID; got != "evt_000001" {
		t.Fatalf("thread-a last seen = %q, want evt_000001", got)
	}
	if got := threads["thread-b"].LastSeenEventID; got != "" {
		t.Fatalf("thread-b last seen before sync = %q, want empty", got)
	}

	mustRun(t, "sync", "--thread", "thread-b", "--budget", "300")
	threads, err = loadThreads()
	if err != nil {
		t.Fatal(err)
	}
	if got := threads["thread-b"].LastSeenEventID; got != "evt_000001" {
		t.Fatalf("thread-b last seen after sync = %q, want evt_000001", got)
	}

	syncDelta := mustRead(t, syncPath)
	assertContains(t, syncDelta, "Don't create a second listing endpoint")
	assertContains(t, syncDelta, "Human note from related thread thread-a.")
	assertContains(t, syncDelta, "- Same issue: VS-123")
	assertContains(t, syncDelta, "- Same area: virtual-store/listing")
}

func TestPreflightAcceptsTaskBeforeFlags(t *testing.T) {
	chdirTemp(t)

	mustRun(t, "init")
	mustRun(t, "note", "--global", "Prefer the existing extension points before adding new surfaces")
	mustRun(t, "preflight", "add filtering to virtual-store listing", "--issue", "VS-123", "--area", "virtual-store/listing", "--budget", "800")

	taskDirection := mustRead(t, taskPath)
	assertContains(t, taskDirection, "Task:\nadd filtering to virtual-store listing")
	assertContains(t, taskDirection, "Prefer the existing extension points")
}

func TestPRReviewContinuationLifecycle(t *testing.T) {
	chdirTemp(t)

	mustRun(t, "init")
	mustRun(t, "thread", "start", "--id", "thread-review-fix", "--issue", "VS-123", "--area", "file-opening")
	mustRun(t, "note", "--issue", "VS-123", "--area", "file-opening", "Do not implement full Office preview unless the task is explicitly rescoped.")

	output := captureStdout(t, func() {
		mustRun(t, "review", "note", "--pr", "123", "--issue", "VS-123", "--area", "file-opening", "Reviewer rejected picker-level Office special-casing; move unsupported file handling into the shared file-open resolver.")
	})
	assertContains(t, output, "Recorded review direction evt_000002 for PR 123.")
	assertContains(t, output, "Marked 1 related threads stale:")
	assertContains(t, output, "- thread-review-fix")

	mustRun(t, "continue", "--pr", "123", "--thread", "thread-c", "--budget", "700")
	context := mustRead(t, continuePath)
	assertContains(t, context, "# Continuation Context")
	assertContains(t, context, "PR:\n123")
	assertContains(t, context, "Current review direction:")
	assertContains(t, context, "1. Reviewer rejected picker-level Office special-casing")
	assertContains(t, context, "Active issue direction:")
	assertContains(t, context, "1. Do not implement full Office preview unless the task is explicitly rescoped.")
	assertContains(t, context, "- Address the review direction first.")
	assertContains(t, context, "- Do not reopen rejected implementation paths.")

	threads, err := loadThreads()
	if err != nil {
		t.Fatal(err)
	}
	if got := threads["thread-c"].LastSeenEventID; got != "evt_000002" {
		t.Fatalf("thread-c last seen = %q, want evt_000002", got)
	}

	explain := captureStdout(t, func() {
		mustRun(t, "explain", "--pr", "123")
	})
	assertContains(t, explain, "Seen by:\n- thread-c")
	assertContains(t, explain, "Stale:\n- thread-review-fix")
}
