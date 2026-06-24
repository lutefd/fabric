package cli

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
)

func TestRunDispatchAndUsage(t *testing.T) {
	output := captureStdout(t, func() {
		mustRun(t)
		mustRun(t, "help")
		mustRun(t, "-h")
		mustRun(t, "--help")
	})
	assertContains(t, output, "Fabric")
	assertContains(t, output, "fabric init")
	assertContains(t, output, "fabric review note")
	assertContains(t, output, "fabric continue")

	if err := Run([]string{"missing"}); err == nil {
		t.Fatal("unknown command returned nil error")
	}
}

func TestInitIsIdempotentAndWritesTemplates(t *testing.T) {
	chdirTemp(t)

	mustRun(t, "init")
	mustRun(t, "init")

	assertContains(t, mustRead(t, configPath), `repo: `)
	assertContains(t, mustRead(t, ".agents/skills/fabric-session/SKILL.md"), "name: fabric-session")
	assertContains(t, mustRead(t, ".agents/skills/fabric-provenance/SKILL.md"), "name: fabric-provenance")
	assertContains(t, mustRead(t, ".agents/skills/fabric-record-direction/SKILL.md"), "name: fabric-record-direction")
	assertContains(t, mustRead(t, ".agents/skills/fabric-pr-direction/SKILL.md"), "name: fabric-pr-direction")
	assertContains(t, mustRead(t, ".agents/skills/fabric-consolidate/SKILL.md"), "name: fabric-consolidate")
	assertContains(t, mustRead(t, ".agents/skills/fabric-publish/SKILL.md"), "name: fabric-publish")
	assertContains(t, mustRead(t, ".agents/skills/fabric-publish/agents/openai.yaml"), "allow_implicit_invocation: false")
	assertContains(t, mustRead(t, agentsPath), "fabric sync")
	assertContains(t, mustRead(t, agentsPath), "$fabric-pr-direction")
	if _, err := os.Stat(".fabric/skills"); !os.IsNotExist(err) {
		t.Fatalf("legacy .fabric/skills should not be generated, stat error = %v", err)
	}

	if err := runInit([]string{"--bad"}); err == nil {
		t.Fatal("runInit accepted an unknown flag")
	}
}

func TestThreadStartValidationAndGeneratedID(t *testing.T) {
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
	mustRun(t, "note", "--durable", "--global", "Global direction")
	mustRun(t, "thread", "start", "--issue", "VS-123")
	mustRun(t, "thread", "start", "--id", "thread-pr", "--pr", "123")

	threads, err := loadThreads()
	if err != nil {
		t.Fatal(err)
	}
	if len(threads) != 2 {
		t.Fatalf("threads count = %d, want 2", len(threads))
	}
	for id, thread := range threads {
		if id == "thread-pr" {
			if thread.PR != "123" {
				t.Fatalf("thread-pr PR = %q, want 123", thread.PR)
			}
			continue
		}
		if !strings.HasPrefix(id, "thr_") {
			t.Fatalf("generated id = %q, want thr_ prefix", id)
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
	mustRun(t, "note", "--durable", "--thread", "thread-a", "Inferred from thread")
	mustRun(t, "note", "--durable", "--kind", "constraint", "--global", "Repo-wide constraint")

	if err := os.MkdirAll(".git", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(".git/HEAD", []byte("ref: refs/heads/feature/VS-999-filtering\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(currentThreadPath); err != nil {
		t.Fatal(err)
	}
	mustRun(t, "note", "--durable", "Inferred from branch")

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

func TestAgentProtocolUsesCurrentThreadAndRootAgentsFile(t *testing.T) {
	chdirTemp(t)

	mustRun(t, "init")
	mustRun(t, "install-agents")

	agents := mustRead(t, "AGENTS.md")
	assertContains(t, agents, fabricBlockStart)
	assertContains(t, agents, "Before substantive multi-step implementation")
	assertContains(t, agents, "Skip session setup for read-only inspection")
	assertContains(t, agents, "fabric sync")
	assertContains(t, agents, "$fabric-session")
	assertContains(t, agents, "$fabric-provenance")
	assertContains(t, agents, fabricBlockEnd)

	if err := os.WriteFile("AGENTS.md", []byte("User notes before\n\n"+agents+"\nUser notes after\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustRun(t, "install-agents")
	updatedAgents := mustRead(t, "AGENTS.md")
	assertContains(t, updatedAgents, "User notes before")
	assertContains(t, updatedAgents, "User notes after")
	assertContains(t, updatedAgents, "$fabric-pr-direction")
	if got := strings.Count(updatedAgents, fabricBlockStart); got != 1 {
		t.Fatalf("fabric start markers = %d, want 1", got)
	}
	if got := strings.Count(updatedAgents, fabricBlockEnd); got != 1 {
		t.Fatalf("fabric end markers = %d, want 1", got)
	}

	mustRun(t, "thread", "start", "--id", "thread-a", "--issue", "FAB-1", "--area", "agent-protocol")
	if got := mustRead(t, currentThreadPath); got != "thread-a\n" {
		t.Fatalf("current thread = %q, want thread-a", got)
	}

	mustRun(t, "thread", "start", "--id", "thread-b", "--issue", "FAB-1", "--area", "agent-protocol")
	mustRun(t, "note", "--durable", "--thread", "thread-a", "Root AGENTS.md is required")
	statusBeforeSync := captureStdout(t, func() {
		mustRun(t, "status")
	})
	assertContains(t, statusBeforeSync, "Current thread:\nthread-b")
	assertContains(t, statusBeforeSync, "1 new relevant direction available.")
	assertContains(t, statusBeforeSync, "Run: fabric sync")
	assertContains(t, statusBeforeSync, "- .fabric/generated/SYNC_DELTA.md")

	mustRun(t, "sync")
	assertContains(t, mustRead(t, syncPath), "Root AGENTS.md is required")

	statusAfterSync := captureStdout(t, func() {
		mustRun(t, "status")
	})
	assertContains(t, statusAfterSync, "No new relevant directions.")

	mustRun(t, "note", "--durable", "Current thread scope should be inferred")
	events, err := loadEvents()
	if err != nil {
		t.Fatal(err)
	}
	got := events[len(events)-1]
	if got.Source.ThreadID != "thread-b" || got.Scope.Issue != "FAB-1" || len(got.Scope.Areas) != 1 || got.Scope.Areas[0] != "agent-protocol" {
		t.Fatalf("current-thread inferred event = %#v", got)
	}
}

func TestSharedEventsPropagateAcrossGitWorktrees(t *testing.T) {
	root := t.TempDir()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previous); err != nil {
			t.Fatal(err)
		}
	})

	commonGit := root + "/common.git"
	workA := root + "/work-a"
	workB := root + "/work-b"
	for _, dir := range []string{commonGit + "/worktrees/a", commonGit + "/worktrees/b", workA, workB} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(commonGit+"/worktrees/a/commondir", []byte("../..\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(commonGit+"/worktrees/b/commondir", []byte("../..\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(workA+"/.git", []byte("gitdir: "+commonGit+"/worktrees/a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(workB+"/.git", []byte("gitdir: "+commonGit+"/worktrees/b\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.Chdir(workA); err != nil {
		t.Fatal(err)
	}
	mustRun(t, "init")
	mustRun(t, "thread", "start", "--id", "thread-a", "--issue", "FAB-1", "--area", "agent-protocol")
	if err := os.Chdir(workB); err != nil {
		t.Fatal(err)
	}
	mustRun(t, "init")
	mustRun(t, "thread", "start", "--id", "thread-b", "--issue", "FAB-1", "--area", "agent-protocol")

	if err := os.Chdir(workA); err != nil {
		t.Fatal(err)
	}
	mustRun(t, "note", "--durable", "Direction from worktree A")
	assertContains(t, readJSONDir(t, ledgerEventsPath), "Direction from worktree A")
	sharedPath, err := sharedEventDir()
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, readJSONDir(t, sharedPath), "Direction from worktree A")

	if err := os.Chdir(workB); err != nil {
		t.Fatal(err)
	}
	status := captureStdout(t, func() {
		mustRun(t, "status")
	})
	assertContains(t, status, "1 new relevant direction available.")
	mustRun(t, "sync")
	assertContains(t, mustRead(t, syncPath), "Direction from worktree A")
	assertNotContains(t, readJSONDir(t, ledgerEventsPath), "Direction from worktree A")
}

func TestInitCreatesImmutableLedger(t *testing.T) {
	chdirTemp(t)
	if err := os.Mkdir(".git", 0o755); err != nil {
		t.Fatal(err)
	}

	mustRun(t, "init")
	if info, err := os.Stat(ledgerEventsPath); err != nil || !info.IsDir() {
		t.Fatalf("immutable ledger directory missing: info=%v err=%v", info, err)
	}
}

func TestStatusReportsNoCurrentAndUnknownCurrentThread(t *testing.T) {
	chdirTemp(t)

	mustRun(t, "init")
	statusWithoutCurrent := captureStdout(t, func() {
		mustRun(t, "status")
	})
	assertContains(t, statusWithoutCurrent, "Current thread:\nnone")
	assertContains(t, statusWithoutCurrent, "issue: none")
	assertContains(t, statusWithoutCurrent, "Run: fabric thread start --issue ... --area ...")

	if err := saveCurrentThreadID("thread-missing"); err != nil {
		t.Fatal(err)
	}
	statusWithUnknownCurrent := captureStdout(t, func() {
		mustRun(t, "status")
	})
	assertContains(t, statusWithUnknownCurrent, `unknown current thread "thread-missing"`)
	assertContains(t, statusWithUnknownCurrent, "Generated files:")
}

func TestSyncExplicitThreadOverridesCurrentThread(t *testing.T) {
	chdirTemp(t)

	mustRun(t, "init")
	mustRun(t, "thread", "start", "--id", "thread-a", "--issue", "FAB-1", "--area", "agent-protocol")
	mustRun(t, "thread", "start", "--id", "thread-b", "--issue", "OTHER-1", "--area", "other-area")
	mustRun(t, "note", "--durable", "--thread", "thread-b", "--issue", "FAB-1", "--area", "agent-protocol", "Direction for the non-current thread")

	mustRun(t, "sync", "--thread", "thread-a")

	assertContains(t, mustRead(t, syncPath), "Direction for the non-current thread")
	if got := mustRead(t, currentThreadPath); got != "thread-b\n" {
		t.Fatalf("current thread = %q, want thread-b", got)
	}
	assertDelivered(t, "thread-a", recordIDAt(t, 0))
}

func TestSyncEnforcesBudgetAndUsesThreadScopeForApplicability(t *testing.T) {
	chdirTemp(t)

	mustRun(t, "init")
	mustRun(t, "thread", "start", "--id", "thread-a", "--issue", "VS-123", "--area", "producer-only")
	mustRun(t, "thread", "start", "--id", "thread-b", "--issue", "VS-123")
	mustRun(t, "note", "--durable", "--thread", "thread-a", "--issue", "VS-123", "--area", "producer-only", "Short")
	mustRun(t, "note", "--durable", "--thread", "thread-a", "--issue", "VS-123", "--area", "producer-only", "This note is long enough to exceed the tiny budget")

	mustRun(t, "sync", "--thread", "thread-b", "--budget", "20")

	syncDelta := mustRead(t, syncPath)
	assertContains(t, syncDelta, "1. Short")
	assertContains(t, syncDelta, budgetOmittedMessage)
	assertContains(t, syncDelta, "- Same issue: VS-123")
	assertNotContains(t, syncDelta, "- Same area: producer-only")

	assertDelivered(t, "thread-b", recordIDAt(t, 0))
	assertNotDelivered(t, "thread-b", recordIDAt(t, 1))
}

func TestConcurrentSharedLedgerWritesDoNotLoseEvents(t *testing.T) {
	chdirTemp(t)
	if err := os.Mkdir(".git", 0o755); err != nil {
		t.Fatal(err)
	}

	mustRun(t, "init")
	mustRun(t, "thread", "start", "--id", "thread-a", "--issue", "FAB-2", "--area", "ledger-safety")

	const workers = 10
	var wg sync.WaitGroup
	errors := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if err := Run([]string{"note", "--durable", fmt.Sprintf("concurrent note %d", i)}); err != nil {
				errors <- err
			}
		}(i)
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		t.Fatalf("concurrent note failed: %v", err)
	}

	events, err := loadEvents()
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != workers {
		t.Fatalf("events count = %d, want %d", len(events), workers)
	}
	seen := map[string]bool{}
	for _, event := range events {
		if event.ID == "" {
			t.Fatal("event has empty ID")
		}
		if seen[event.ID] {
			t.Fatalf("duplicate event ID %s", event.ID)
		}
		seen[event.ID] = true
	}

	sharedPath, err := sharedEventDir()
	if err != nil {
		t.Fatal(err)
	}
	sharedData := readJSONDir(t, sharedPath)
	for i := 0; i < workers; i++ {
		if !strings.Contains(sharedData, fmt.Sprintf("concurrent note %d", i)) {
			t.Fatalf("shared ledger missing note %d", i)
		}
	}
}

func TestDoctorDetectsLedgerProblems(t *testing.T) {
	chdirTemp(t)
	if err := os.Mkdir(".git", 0o755); err != nil {
		t.Fatal(err)
	}

	mustRun(t, "init")
	mustRun(t, "thread", "start", "--id", "thread-a", "--issue", "FAB-2", "--area", "ledger-safety")
	mustRun(t, "note", "--live", "Live direction")
	mustRun(t, "note", "--candidate", "Candidate direction")
	mustRun(t, "note", "--durable", "Durable direction")

	output := captureStdout(t, func() {
		mustRun(t, "doctor")
	})
	assertContains(t, output, "Shared mirror:\nok")
	assertContains(t, output, "Durable ledger:\nok")
	assertContains(t, output, "- live: 1")
	assertContains(t, output, "- candidate: 1")
	assertContains(t, output, "- durable: 1")
	assertContains(t, output, "- invalid immutable events: none")
	assertContains(t, output, "- durable/shared mismatch: none")

	if err := os.WriteFile(ledgerEventsPath+"/evt_bad.json", []byte("not json\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	output = captureStdout(t, func() {
		mustRun(t, "doctor")
	})
	assertContains(t, output, "Durable ledger:\nerror")
	assertContains(t, output, "- invalid immutable events:")
}

func TestSyncBudgetCanOmitAllRelevantDirection(t *testing.T) {
	chdirTemp(t)

	mustRun(t, "init")
	mustRun(t, "thread", "start", "--id", "thread-a", "--issue", "VS-123")
	mustRun(t, "thread", "start", "--id", "thread-b", "--issue", "VS-123")
	mustRun(t, "note", "--durable", "--thread", "thread-a", "--issue", "VS-123", "Long enough to exceed one token")
	mustRun(t, "sync", "--thread", "thread-b", "--budget", "1")

	syncDelta := mustRead(t, syncPath)
	assertContains(t, syncDelta, "No direction included within the current budget.")
	assertContains(t, syncDelta, budgetOmittedMessage)
	assertContains(t, syncDelta, "Source:\n(none included)")
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

	mustRun(t, "note", "--durable", "--global", "Global direction")
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

func TestPreflightEnforcesBudget(t *testing.T) {
	chdirTemp(t)

	mustRun(t, "init")
	mustRun(t, "note", "--durable", "--global", "Short")
	mustRun(t, "note", "--durable", "--global", "This note is long enough to exceed the tiny budget")
	mustRun(t, "preflight", "task", "--issue", "VS-123", "--budget", "15")

	taskDirection := mustRead(t, taskPath)
	assertContains(t, taskDirection, "1. Short")
	assertContains(t, taskDirection, budgetOmittedMessage)
	assertNotContains(t, taskDirection, "This note is long enough")
}

func TestPreflightBudgetCanOmitAllRelevantDirection(t *testing.T) {
	chdirTemp(t)

	mustRun(t, "init")
	mustRun(t, "note", "--durable", "--global", "Long enough to exceed one token")
	mustRun(t, "preflight", "task", "--issue", "VS-123", "--budget", "1")

	taskDirection := mustRead(t, taskPath)
	assertContains(t, taskDirection, "No direction included within the current budget.")
	assertContains(t, taskDirection, budgetOmittedMessage)
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
	mustRun(t, "note", "--durable", "--thread", "thread-a", "--issue", "VS-123", "--area", "virtual-store/listing", "Use the existing endpoint")

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

func TestNoteDurabilityFlagsAndMutualExclusion(t *testing.T) {
	chdirTemp(t)

	if err := Run([]string{"note", "--live", "--durable", "text"}); err == nil {
		t.Fatal("note accepted both --live and --durable")
	}
	if err := Run([]string{"note", "--live", "--candidate", "text"}); err == nil {
		t.Fatal("note accepted both --live and --candidate")
	}
	if err := Run([]string{"note", "--candidate", "--durable", "text"}); err == nil {
		t.Fatal("note accepted both --candidate and --durable")
	}

	mustRun(t, "init")
	mustRun(t, "thread", "start", "--id", "thread-a", "--issue", "VS-123")

	mustRun(t, "note", "--live", "Live direction")
	events, err := loadEvents()
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Durability != DurabilityLive {
		t.Fatalf("live event = %#v", events[0])
	}

	mustRun(t, "note", "--candidate", "Candidate direction")
	events, err = loadEvents()
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[1].Durability != DurabilityCandidate {
		t.Fatalf("candidate event = %#v", events[1])
	}

	mustRun(t, "note", "--durable", "Durable direction")
	events, err = loadEvents()
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 || events[2].Durability != DurabilityDurable {
		t.Fatalf("durable event = %#v", events[2])
	}
}

func TestNoteLiveNotPersistedToLocalLedger(t *testing.T) {
	chdirTemp(t)
	if err := os.Mkdir(".git", 0o755); err != nil {
		t.Fatal(err)
	}

	mustRun(t, "init")
	mustRun(t, "thread", "start", "--id", "thread-a", "--issue", "VS-123")
	mustRun(t, "note", "--live", "Live only direction")

	if data := strings.TrimSpace(readJSONDir(t, ledgerEventsPath)); data != "" {
		t.Fatalf("local ledger should be empty for live event, got %q", data)
	}

	sharedPath, err := sharedEventDir()
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, readJSONDir(t, sharedPath), "Live only direction")

	events, err := loadEvents()
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("loadEvents count = %d, want 1", len(events))
	}
}

func TestPromoteCandidateToDurable(t *testing.T) {
	chdirTemp(t)
	if err := os.Mkdir(".git", 0o755); err != nil {
		t.Fatal(err)
	}

	mustRun(t, "init")
	mustRun(t, "thread", "start", "--id", "thread-a", "--issue", "VS-123")
	mustRun(t, "note", "--candidate", "Promote me")

	events, err := loadEvents()
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Durability != DurabilityCandidate {
		t.Fatalf("candidate event = %#v", events[0])
	}

	if err := Run([]string{"promote", "missing"}); err == nil {
		t.Fatal("promote accepted unknown event id")
	}

	mustRun(t, "promote", "--reason", "Confirmed for durable reuse", events[0].ID)

	events, err = loadEvents()
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Durability != DurabilityDurable {
		t.Fatalf("promoted event = %#v", events[0])
	}

	sharedPath, err := sharedEventDir()
	if err != nil {
		t.Fatal(err)
	}
	sharedData := readJSONDir(t, sharedPath)
	if !strings.Contains(sharedData, `"durability": "durable"`) {
		t.Fatalf("shared ledger did not reflect promotion: %s", sharedData)
	}
}
