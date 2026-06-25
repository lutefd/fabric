package cli

import (
	"os"
	"testing"
)

func TestListShowsActionableLiveDirection(t *testing.T) {
	chdirTemp(t)
	mustRun(t, "init")
	mustRun(t, "thread", "start", "--id", "thread-a", "--issue", "FAB-1")
	mustRun(t, "note", "--live", "Remember the local TODO")
	mustRun(t, "note", "--candidate", "A later candidate")

	output := captureStdout(t, func() { mustRun(t, "list", "--durability", "live") })
	assertContains(t, output, "Remember the local TODO")
	assertNotContains(t, output, "A later candidate")
}

func TestCleanLivePreviewsThenRemovesOnlySelectedLiveRecord(t *testing.T) {
	chdirTemp(t)
	mustRun(t, "init")
	mustRun(t, "thread", "start", "--id", "thread-a", "--issue", "TEST-1")
	mustRun(t, "note", "--live", "Temporary test note")
	liveID := recordIDAt(t, 0)
	mustRun(t, "note", "--candidate", "Keep candidate")

	preview := captureStdout(t, func() { mustRun(t, "clean", "live", "--record", liveID) })
	assertContains(t, preview, "Preview only")
	assertContains(t, captureStdout(t, func() { mustRun(t, "list", "--status", "any") }), "Temporary test note")

	mustRun(t, "clean", "live", "--record", liveID, "--apply")
	all := captureStdout(t, func() { mustRun(t, "list", "--status", "any") })
	assertNotContains(t, all, "Temporary test note")
	assertContains(t, all, "Keep candidate")
	if _, err := os.Stat(ledgerEventsPath); err != nil {
		t.Fatal(err)
	}
}

func TestThreadCanBeListedSelectedAndCleared(t *testing.T) {
	chdirTemp(t)
	mustRun(t, "init")
	mustRun(t, "thread", "start", "--id", "thread-a", "--issue", "FAB-1")
	mustRun(t, "thread", "start", "--id", "thread-b", "--area", "runtime")

	listed := captureStdout(t, func() { mustRun(t, "thread", "list") })
	assertContains(t, listed, "thread-a")
	assertContains(t, listed, "thread-b")
	mustRun(t, "thread", "use", "thread-a")
	if current, err := loadCurrentThreadID(); err != nil || current != "thread-a" {
		t.Fatalf("current thread = %q, %v", current, err)
	}
	mustRun(t, "thread", "clear")
	if current, err := loadCurrentThreadID(); err != nil || current != "" {
		t.Fatalf("current thread after clear = %q, %v", current, err)
	}
}

func TestCleanRuntimeDoesNotDeleteDirectionFromThread(t *testing.T) {
	chdirTemp(t)
	mustRun(t, "init")
	mustRun(t, "thread", "start", "--id", "obsolete", "--issue", "TEST-1")
	mustRun(t, "note", "--live", "Preserve this direction")

	mustRun(t, "clean", "runtime", "--thread", "obsolete", "--apply")
	listed := captureStdout(t, func() { mustRun(t, "list", "--durability", "live") })
	assertContains(t, listed, "Preserve this direction")
}

func TestListTreatsResolvedChallengeAsHistory(t *testing.T) {
	chdirTemp(t)
	mustRun(t, "init")
	mustRun(t, "thread", "start", "--id", "thread-a", "--issue", "FAB-1")
	mustRun(t, "note", "--candidate", "Original direction")
	directionID := recordIDAt(t, 0)
	mustRun(t, "challenge", "--direction", directionID, "--issue", "FAB-1", "Alternative")
	challengeID := recordIDAt(t, 1)
	mustRun(t, "challenge", "resolve", challengeID, "--rejected", "--reason", "Keep original")

	active := captureStdout(t, func() { mustRun(t, "list", "--status", "active") })
	assertNotContains(t, active, "Alternative")
	assertNotContains(t, active, "Challenge "+challengeID)

	history := captureStdout(t, func() { mustRun(t, "list", "--status", "inactive") })
	assertContains(t, history, "Alternative")
	assertContains(t, history, "Challenge "+challengeID)
}
