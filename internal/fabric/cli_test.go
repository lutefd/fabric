package fabric

import (
	"os"
	"strings"
	"testing"
)

func TestRunDispatchAndUsage(t *testing.T) {
	output := captureStdout(t, func() {
		mustRun(t)
		mustRun(t, "help")
		mustRun(t, "-h")
		mustRun(t, "--help")
	})
	assertContains(t, output, "Fabric V0")
	assertContains(t, output, "fabric init")

	if err := Run([]string{"missing"}); err == nil {
		t.Fatal("unknown command returned nil error")
	}
}

func TestInitIsIdempotentAndWritesTemplates(t *testing.T) {
	chdirTemp(t)

	mustRun(t, "init")
	mustRun(t, "init")

	assertContains(t, mustRead(t, configPath), `repo: `)
	assertContains(t, mustRead(t, ".fabric/skills/preflight/SKILL.md"), "Fabric Preflight")
	assertContains(t, mustRead(t, ".fabric/skills/sync/SKILL.md"), "Fabric Sync")
	assertContains(t, mustRead(t, ".fabric/skills/note/SKILL.md"), "Fabric Note")
	assertContains(t, mustRead(t, agentsPath), "fabric sync")

	if err := runInit([]string{"--bad"}); err == nil {
		t.Fatal("runInit accepted an unknown flag")
	}
}

func TestThreadStartValidationGeneratedIDAndLastSeen(t *testing.T) {
	chdirTemp(t)

	mustRun(t, "init")
	if err := Run([]string{"thread", "start", "--wat"}); err == nil {
		t.Fatal("thread start accepted unknown flag")
	}
	if err := Run([]string{"thread"}); err == nil {
		t.Fatal("thread without start returned nil error")
	}
	if err := Run([]string{"thread", "start"}); err == nil {
		t.Fatal("thread start without scope returned nil error")
	}
	mustRun(t, "note", "--global", "Global direction")
	mustRun(t, "thread", "start", "--issue", "VS-123")

	threads, err := loadThreads()
	if err != nil {
		t.Fatal(err)
	}
	if len(threads) != 1 {
		t.Fatalf("threads count = %d, want 1", len(threads))
	}
	for id, thread := range threads {
		if !strings.HasPrefix(id, "thread_") {
			t.Fatalf("generated id = %q, want thread_ prefix", id)
		}
		if thread.LastSeenEventID != "evt_000001" {
			t.Fatalf("last seen = %q, want evt_000001", thread.LastSeenEventID)
		}
	}
}

func TestNoteValidationScopeInferenceAndBranchInference(t *testing.T) {
	chdirTemp(t)

	if err := Run([]string{"note", "--global", "before init"}); err == nil {
		t.Fatal("note before init returned nil error")
	}
	mustRun(t, "init")
	if err := Run([]string{"note", "--wat"}); err == nil {
		t.Fatal("note accepted unknown flag")
	}
	if err := Run([]string{"note", "--global"}); err == nil {
		t.Fatal("empty note returned nil error")
	}
	if err := Run([]string{"note", "scopeless"}); err == nil {
		t.Fatal("scopeless note returned nil error")
	}

	mustRun(t, "thread", "start", "--id", "thread-a", "--issue", "VS-123", "--area", "virtual-store/listing")
	mustRun(t, "note", "--thread", "thread-a", "Inferred from thread")
	mustRun(t, "note", "--kind", "constraint", "--global", "Repo-wide constraint")

	if err := os.MkdirAll(".git", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(".git/HEAD", []byte("ref: refs/heads/feature/VS-999-filtering\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustRun(t, "note", "Inferred from branch")

	events, err := loadEvents()
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 {
		t.Fatalf("events count = %d, want 3", len(events))
	}
	if events[0].Scope.Issue != "VS-123" || len(events[0].Scope.Areas) != 1 {
		t.Fatalf("thread-inferred scope = %#v", events[0].Scope)
	}
	if events[1].Kind != "constraint" || !events[1].Scope.Global {
		t.Fatalf("global constraint event = %#v", events[1])
	}
	if events[2].Scope.Issue != "VS-999" {
		t.Fatalf("branch-inferred issue = %q, want VS-999", events[2].Scope.Issue)
	}
}

func TestSyncValidationAndNoUpdates(t *testing.T) {
	chdirTemp(t)

	mustRun(t, "init")
	if err := Run([]string{"sync", "--wat"}); err == nil {
		t.Fatal("sync accepted unknown flag")
	}
	if err := Run([]string{"sync"}); err == nil {
		t.Fatal("sync without thread returned nil error")
	}
	if err := Run([]string{"sync", "--thread", "missing"}); err == nil {
		t.Fatal("sync unknown thread returned nil error")
	}

	mustRun(t, "thread", "start", "--id", "thread-a", "--issue", "VS-123")
	mustRun(t, "sync", "--thread", "thread-a")
	assertContains(t, mustRead(t, syncPath), "No new relevant direction")
}

func TestPreflightValidationAndParserBranches(t *testing.T) {
	chdirTemp(t)

	mustRun(t, "init")
	cases := [][]string{
		{"preflight", "--issue", "VS-123"},
		{"preflight", "task"},
		{"preflight", "task", "--issue"},
		{"preflight", "task", "--area"},
		{"preflight", "task", "--area", ""},
		{"preflight", "task", "--budget"},
		{"preflight", "task", "--budget", "wat"},
		{"preflight", "task", "--budget", "0"},
		{"preflight", "task", "--area="},
		{"preflight", "task", "--budget=wat"},
	}
	for _, args := range cases {
		if err := Run(args); err == nil {
			t.Fatalf("Run(%q) returned nil error", strings.Join(args, " "))
		}
	}

	mustRun(t, "note", "--global", "Global direction")
	mustRun(t, "preflight", "task", "--issue=VS-123", "--area=virtual-store/listing", "--budget=800")
	assertContains(t, mustRead(t, taskPath), "- Repo-wide direction")
}

func TestPreflightWithNoActiveDirection(t *testing.T) {
	chdirTemp(t)

	mustRun(t, "init")
	mustRun(t, "preflight", "task", "--issue", "VS-123")

	taskDirection := mustRead(t, taskPath)
	assertContains(t, taskDirection, "No active direction found.")
	assertContains(t, taskDirection, "Scope match:\n- None")
}

func TestExplainOutputBranches(t *testing.T) {
	chdirTemp(t)

	mustRun(t, "init")
	if err := Run([]string{"explain", "--wat"}); err == nil {
		t.Fatal("explain accepted unknown flag")
	}
	if err := Run([]string{"explain"}); err == nil {
		t.Fatal("explain without scope returned nil error")
	}

	noMatches := captureStdout(t, func() {
		mustRun(t, "explain", "--area", "unknown")
	})
	assertContains(t, noMatches, "No active direction found.")

	mustRun(t, "thread", "start", "--id", "thread-a", "--issue", "VS-123", "--area", "virtual-store/listing")
	mustRun(t, "thread", "start", "--id", "thread-b", "--issue", "VS-123", "--area", "virtual-store/listing")
	mustRun(t, "note", "--thread", "thread-a", "--issue", "VS-123", "--area", "virtual-store/listing", "Use the existing endpoint")

	output := captureStdout(t, func() {
		mustRun(t, "explain", "--issue", "VS-123")
		mustRun(t, "explain", "--area", "virtual-store/listing")
	})
	assertContains(t, output, "Active direction for issue VS-123")
	assertContains(t, output, "Active direction for area virtual-store/listing")
	assertContains(t, output, "Seen by:\n- thread-a")
	assertContains(t, output, "Stale:\n- thread-b")

	mustRun(t, "sync", "--thread", "thread-b")
	output = captureStdout(t, func() {
		mustRun(t, "explain", "--issue", "VS-123")
	})
	assertContains(t, output, "Seen by:\n- thread-a\n- thread-b")
	assertContains(t, output, "Stale:\n- (none)")
}
