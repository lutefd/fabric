package fabric

import (
	"errors"
	"os"
	"testing"
)

func TestChallengeFlowCreatesAndResolvesExplicitDirectionDispute(t *testing.T) {
	chdirTemp(t)

	mustRun(t, "init")
	mustRun(t, "thread", "start", "--id", "thread-c", "--pr", "123", "--issue", "VS-123", "--area", "file-opening")
	mustRun(t, "note", "--issue", "VS-123", "--area", "file-opening", "Do not implement full Office preview; this is an entry-point consistency issue.")
	mustRun(t, "review", "note", "--pr", "123", "--issue", "VS-123", "--area", "file-opening", "Reviewer rejected picker-level Office special-casing; move unsupported file handling into the shared file-open resolver.")

	output := captureStdout(t, func() {
		mustRun(t, "challenge", "--direction", "evt_000001", "--pr", "123", "--issue", "VS-123", "--area", "file-opening", "--proposal", "Implement internal Office preview for supported Office files", "--reason", "Product explicitly rescoped this from entry-point consistency to preview support.")
	})
	assertContains(t, output, "Recorded challenge evt_000003 against evt_000001.")
	assertContains(t, output, "Wrote .fabric/generated/CHALLENGE.md")
	assertContains(t, output, "Marked 1 related threads stale:")
	assertContains(t, output, "- thread-c")

	events, err := loadEvents()
	if err != nil {
		t.Fatal(err)
	}
	if events[2].Kind != "challenge" || events[2].Challenges != "evt_000001" || events[2].Status != "open" {
		t.Fatalf("challenge event = %#v", events[2])
	}

	challenge := mustRead(t, challengePath)
	assertContains(t, challenge, "Challenged direction:\nevt_000001")
	assertContains(t, challenge, "Existing direction:\nDo not implement full Office preview")
	assertContains(t, challenge, "Challenge:\nImplement internal Office preview for supported Office files")
	assertContains(t, challenge, "Reason:\nProduct explicitly rescoped")

	mustRun(t, "continue", "--pr", "123", "--thread", "thread-d", "--budget", "700")
	context := mustRead(t, continuePath)
	assertContains(t, context, "Open challenge:")
	assertContains(t, context, "1. Direction evt_000001 is being challenged.")
	assertContains(t, context, "Proposed exception: Implement internal Office preview for supported Office files")
	assertContains(t, context, "Do not assume the old direction is final for this PR.")
	assertOrder(t, context, "Open challenge:", "Current review direction:")
	assertOrder(t, context, "Current review direction:", "Active issue direction:")

	mustRun(t, "challenge", "resolve", "evt_000003", "--accepted")
	events, err = loadEvents()
	if err != nil {
		t.Fatal(err)
	}
	if events[3].Kind != "challenge_resolution" || events[3].Challenges != "evt_000003" || events[3].Status != "accepted" {
		t.Fatalf("resolution event = %#v", events[3])
	}

	mustRun(t, "continue", "--pr", "123", "--budget", "700")
	context = mustRead(t, continuePath)
	assertContains(t, context, "Open challenge:\n\nNo open challenge found.")
	assertContains(t, context, "Resolved challenge:")
	assertContains(t, context, "Challenge evt_000003 accepted as a scoped exception.")

	explain := captureStdout(t, func() {
		mustRun(t, "explain", "--pr", "123")
	})
	assertContains(t, explain, "Kind:\nchallenge")
	assertContains(t, explain, "status: open")
	assertContains(t, explain, "Kind:\nchallenge_resolution")
	assertContains(t, explain, "status: accepted")
}

func TestChallengeValidationAndTextForm(t *testing.T) {
	chdirTemp(t)

	mustRun(t, "init")
	mustRun(t, "note", "--issue", "VS-123", "Existing direction")

	if err := Run([]string{"challenge", "--wat"}); err == nil {
		t.Fatal("challenge accepted unknown flag")
	}
	if err := Run([]string{"challenge", "--issue", "VS-123", "missing direction"}); err == nil {
		t.Fatal("challenge without direction returned nil error")
	}
	if err := Run([]string{"challenge", "--direction", "evt_999999", "--issue", "VS-123", "unknown direction"}); err == nil {
		t.Fatal("challenge accepted unknown direction")
	}
	if err := Run([]string{"challenge", "--direction", "evt_000001", "--issue", "VS-123"}); err == nil {
		t.Fatal("challenge without text returned nil error")
	}
	if err := Run([]string{"challenge", "--direction", "evt_000001", "missing scope"}); err == nil {
		t.Fatal("challenge without scope returned nil error")
	}

	mustRun(t, "challenge", "--direction", "evt_000001", "--issue", "VS-123", "Challenge: implement Office preview because the task was explicitly rescoped.")
	events, err := loadEvents()
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, events[1].Text, "Challenge direction evt_000001: Challenge: implement Office preview")

	if err := Run([]string{"challenge", "resolve", "evt_000002"}); err == nil {
		t.Fatal("resolve without status returned nil error")
	}
	if err := Run([]string{"challenge", "resolve", "--accepted"}); err == nil {
		t.Fatal("resolve without challenge id returned nil error")
	}
	if err := Run([]string{"challenge", "resolve", "evt_000002", "--accepted", "--rejected"}); err == nil {
		t.Fatal("resolve accepted multiple statuses")
	}
	if err := Run([]string{"challenge", "resolve", "--wat"}); err == nil {
		t.Fatal("resolve accepted unknown flag")
	}
	if err := Run([]string{"challenge", "resolve", "evt_999999", "--accepted"}); err == nil {
		t.Fatal("resolve accepted unknown challenge")
	}
	mustRun(t, "challenge", "resolve", "evt_000002", "--superseded", "--reason", "New API direction approved.")
	if err := Run([]string{"challenge", "resolve", "evt_000002", "--accepted"}); err == nil {
		t.Fatal("resolve accepted an already resolved challenge")
	}
}

func TestChallengeCreateStorageFailures(t *testing.T) {
	chdirTemp(t)

	if err := Run([]string{"challenge", "--direction", "evt_000001", "--issue", "VS-123", "before init"}); err == nil {
		t.Fatal("challenge before init returned nil error")
	}

	mustRun(t, "init")
	mustRun(t, "note", "--issue", "VS-123", "Existing direction")

	oldAppend := appendLedger
	appendLedger = func(string, any) error {
		return errors.New("append failed")
	}
	if err := Run([]string{"challenge", "--direction", "evt_000001", "--issue", "VS-123", "append failure"}); err == nil {
		t.Fatal("challenge append failure returned nil error")
	}
	appendLedger = oldAppend

	chdirTemp(t)
	mustRun(t, "init")
	mustRun(t, "note", "--issue", "VS-123", "Existing direction")
	if err := os.Remove(challengePath); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if err := os.MkdirAll(challengePath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"challenge", "--direction", "evt_000001", "--issue", "VS-123", "write failure"}); err == nil {
		t.Fatal("challenge write failure returned nil error")
	}

	chdirTemp(t)
	mustRun(t, "init")
	mustRun(t, "note", "--issue", "VS-123", "Existing direction")
	if err := os.Remove(threadsPath); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(threadsPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"challenge", "--direction", "evt_000001", "--issue", "VS-123", "thread load failure"}); err == nil {
		t.Fatal("challenge thread load failure returned nil error")
	}
}

func TestChallengeResolveStorageFailuresAndStatuses(t *testing.T) {
	chdirTemp(t)

	if err := Run([]string{"challenge", "resolve", "evt_000001", "--accepted"}); err == nil {
		t.Fatal("resolve before init returned nil error")
	}

	mustRun(t, "init")
	mustRun(t, "note", "--issue", "VS-123", "Existing direction")
	mustRun(t, "challenge", "--direction", "evt_000001", "--issue", "VS-123", "Challenge for rejection")
	mustRun(t, "challenge", "--direction", "evt_000001", "--issue", "VS-123", "Challenge for append failure")

	mustRun(t, "challenge", "resolve", "--rejected", "evt_000002")
	events, err := loadEvents()
	if err != nil {
		t.Fatal(err)
	}
	if events[3].Status != "rejected" || events[3].Text != "Challenge evt_000002 rejected." {
		t.Fatalf("rejected resolution = %#v", events[3])
	}
	if got := resolutionPhrase("custom"); got != "custom" {
		t.Fatalf("default resolution phrase = %q, want custom", got)
	}

	oldAppend := appendLedger
	appendLedger = func(string, any) error {
		return errors.New("append failed")
	}
	if err := Run([]string{"challenge", "resolve", "evt_000003", "--accepted"}); err == nil {
		t.Fatal("resolve append failure returned nil error")
	}
	appendLedger = oldAppend
}
