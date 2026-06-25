package cli

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lutefd/fabric/internal/core"
	filestore "github.com/lutefd/fabric/internal/store"
	"github.com/lutefd/fabric/protocol"
)

func TestStorageRuntimeBehavior(t *testing.T) {
	t.Run("lock paths and current-thread read errors", func(t *testing.T) {
		chdirTemp(t)
		mustRun(t, "init")

		if err := os.WriteFile("file-parent", []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := withFileLock("file-parent/lock", func() error { return nil }); err == nil {
			t.Fatal("withFileLock with file parent returned nil error")
		}

		if err := os.Mkdir("lockdir", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := withFileLock("lockdir", func() error { return nil }); err == nil {
			t.Fatal("withFileLock with directory lock path returned nil error")
		}

		lockPath := filepath.Join(t.TempDir(), "lock")
		locked := false
		if err := withFileLock(lockPath, func() error { locked = true; return nil }); err != nil {
			t.Fatalf("withFileLock success path: %v", err)
		}
		if !locked {
			t.Fatal("withFileLock did not execute fn")
		}

		if err := os.Mkdir(currentThreadPath, 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := loadCurrentThreadID(); err == nil {
			t.Fatal("loadCurrentThreadID with directory path returned nil error")
		}

		localLock, err := ledgerLockPath()
		if err != nil {
			t.Fatalf("ledgerLockPath outside git: %v", err)
		}
		if localLock != filepath.Join(filepath.Dir(currentThreadPath), "lock") {
			t.Fatalf("ledgerLockPath = %q, want local lock path", localLock)
		}

		if err := os.Mkdir("gitdir", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(filepath.Join("gitdir", "commondir"), 0o755); err != nil {
			t.Fatal(err)
		}
		gitDir, err := filepath.Abs("gitdir")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(".git", []byte("gitdir:"+gitDir), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := gitCommonDir(); err == nil {
			t.Fatal("gitCommonDir with commondir as directory returned nil error")
		}
		if err := withLedgerLock(func() error { return nil }); err == nil {
			t.Fatal("withLedgerLock with commondir as directory returned nil error")
		}
	})

	t.Run("runtime kind and envelope validation", func(t *testing.T) {
		chdirTemp(t)
		mustRun(t, "init")

		if got := runtimeKindForEvent("unknown"); got != "" {
			t.Fatalf("runtimeKindForEvent(unknown) = %q, want empty", got)
		}

		if err := appendRuntimeEnvelope(runtimeThreads, protocol.EventEnvelope{}); err == nil {
			t.Fatal("appendRuntimeEnvelope accepted invalid envelope")
		}

		envelope, err := protocol.NewEnvelope(protocol.EventThreadStarted,
			protocol.ActorRef{Kind: "agent", ID: "thread-a"},
			protocol.TrustClaim{Level: "agent_asserted"},
			protocol.ThreadEvent{Thread: protocol.Thread{
				ThreadID:  "thread-a",
				CreatedAt: time.Now().Format(time.RFC3339Nano),
				UpdatedAt: time.Now().Format(time.RFC3339Nano),
				Scope:     protocol.Scope{Global: true},
			}})
		if err != nil {
			t.Fatal(err)
		}
		if err := appendRuntimeEnvelope(runtimeProjections, envelope); err == nil {
			t.Fatal("appendRuntimeEnvelope accepted kind mismatch")
		}

		if err := saveRuntimeThread(ThreadRecord{ThreadID: "thread-a"}, protocol.EventProjectionCreated); err == nil {
			t.Fatal("saveRuntimeThread with wrong event type returned nil error")
		}
	})

	t.Run("revision delivery branches", func(t *testing.T) {
		chdirTemp(t)
		mustRun(t, "init")

		if allRevisionsDelivered(DirectionEvent{}, nil) {
			t.Fatal("allRevisionsDelivered with empty IDs should be false")
		}

		evt1, err := protocol.NewEventID()
		if err != nil {
			t.Fatal(err)
		}
		evt2, err := protocol.NewEventID()
		if err != nil {
			t.Fatal(err)
		}
		rec1, err := protocol.NewRecordID()
		if err != nil {
			t.Fatal(err)
		}

		if !allRevisionsDelivered(DirectionEvent{HeadEventID: evt1}, map[string]bool{evt1: true}) {
			t.Fatal("allRevisionsDelivered should be true when head delivered")
		}
		if allRevisionsDelivered(DirectionEvent{HeadEventID: evt1}, map[string]bool{evt2: true}) {
			t.Fatal("allRevisionsDelivered should be false when head missing")
		}

		if err := writeTestReceipt(t, "thread-a", []string{evt1}, []string{rec1}); err != nil {
			t.Fatal(err)
		}

		delivered, records, err := deliveredForThread("thread-a")
		if err != nil {
			t.Fatal(err)
		}
		if !delivered[evt1] {
			t.Fatal("deliveredForThread did not include event id")
		}
		if !records[rec1] {
			t.Fatal("deliveredForThread did not include record id")
		}

		thread := ThreadRecord{ThreadID: "thread-a", Issue: "FAB-1"}
		events := []DirectionEvent{
			{ID: rec1, HeadEventID: evt1, Status: core.StatusActive, Scope: core.EventScope{Issue: "FAB-1"}},
			{ID: "rec-2", HeadEventID: evt2, Status: core.StatusActive, Scope: core.EventScope{Issue: "FAB-1"}},
			{ID: "rec-3", HeadEventID: evt2, Status: core.StatusActive, Scope: core.EventScope{Issue: "FAB-2"}},
		}
		matches, err := relevantUndelivered(events, thread)
		if err != nil {
			t.Fatal(err)
		}
		if len(matches) != 1 || matches[0].ID != "rec-2" {
			t.Fatalf("relevantUndelivered = %#v, want rec-2 only", matches)
		}

		if err := os.WriteFile(runtimeReceiptPath(t), []byte("{bad"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, _, err := deliveredForThread("thread-a"); err == nil {
			t.Fatal("deliveredForThread accepted malformed receipts")
		}
	})

	t.Run("stale and seen classification", func(t *testing.T) {
		chdirTemp(t)
		mustRun(t, "init")

		evt1, err := protocol.NewEventID()
		if err != nil {
			t.Fatal(err)
		}
		evt2, err := protocol.NewEventID()
		if err != nil {
			t.Fatal(err)
		}
		rec1, err := protocol.NewRecordID()
		if err != nil {
			t.Fatal(err)
		}

		if err := writeTestReceipt(t, "thread-a", []string{evt1}, nil); err != nil {
			t.Fatal(err)
		}

		threads := map[string]ThreadRecord{
			"thread-a": {ThreadID: "thread-a", Issue: "FAB-1"},
			"thread-b": {ThreadID: "thread-b", Issue: "FAB-2"},
		}

		inactive := DirectionEvent{Status: core.StatusExpired}
		seen, stale, err := seenAndStaleFromReceipts(inactive, threads)
		if err != nil {
			t.Fatal(err)
		}
		if len(seen) != 0 || len(stale) != 0 {
			t.Fatalf("inactive event produced seen/stale: %v %v", seen, stale)
		}

		activeSeen := DirectionEvent{ID: rec1, HeadEventID: evt1, Status: core.StatusActive, Scope: core.EventScope{Issue: "FAB-1"}}
		seen, stale, err = seenAndStaleFromReceipts(activeSeen, threads)
		if err != nil {
			t.Fatal(err)
		}
		if len(seen) != 1 || seen[0] != "thread-a" || len(stale) != 0 {
			t.Fatalf("seen = %v, stale = %v, want [thread-a] []", seen, stale)
		}

		activeStale := DirectionEvent{ID: rec1, HeadEventID: evt2, Status: core.StatusActive, Scope: core.EventScope{Issue: "FAB-1"}}
		seen, stale, err = seenAndStaleFromReceipts(activeStale, threads)
		if err != nil {
			t.Fatal(err)
		}
		if len(seen) != 0 || len(stale) != 1 || stale[0] != "thread-a" {
			t.Fatalf("seen = %v, stale = %v, want [] [thread-a]", seen, stale)
		}
	})

	t.Run("projection and receipt store errors", func(t *testing.T) {
		chdirTemp(t)
		mustRun(t, "init")

		root, err := sharedRuntimeRoot()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(root, 0o755); err != nil {
			t.Fatal(err)
		}

		if _, err := loadProjection("prj-missing"); err == nil {
			t.Fatal("loadProjection missing returned nil error")
		}

		if err := os.WriteFile(filepath.Join(root, runtimeProjections), []byte(""), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := createProjection("sync", "thread-a", protocol.Scope{Global: true}, nil, false); err == nil {
			t.Fatal("createProjection with file projection dir returned nil error")
		}

		if err := os.WriteFile(filepath.Join(root, runtimeReceipts), []byte(""), 0o644); err != nil {
			t.Fatal(err)
		}
		projectionID, err := protocol.NewProjectionID()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := recordProjectionReceipt(protocol.Projection{ProjectionID: projectionID, ThreadID: "thread-a"}, protocol.ReceiptDelivered, "codex"); err == nil {
			t.Fatal("recordProjectionReceipt with file receipts dir returned nil error")
		}

		if err := os.WriteFile(filepath.Join(root, runtimeThreads), []byte(""), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := loadRuntimeEvents(runtimeThreads); err == nil {
			t.Fatal("loadRuntimeEvents with file threads dir returned nil error")
		}
		if _, err := loadRuntimeThreads(); err == nil {
			t.Fatal("loadRuntimeThreads with file threads dir returned nil error")
		}
	})
}

func writeTestReceipt(t *testing.T, threadID string, eventIDs, recordIDs []string) error {
	t.Helper()
	receiptID, err := protocol.NewReceiptID()
	if err != nil {
		return err
	}
	projectionID, err := protocol.NewProjectionID()
	if err != nil {
		return err
	}
	receipt := protocol.Receipt{
		ReceiptID:    receiptID,
		ProjectionID: projectionID,
		ThreadID:     threadID,
		State:        protocol.ReceiptDelivered,
		OccurredAt:   time.Now().Format(time.RFC3339Nano),
		EventIDs:     eventIDs,
		RecordIDs:    recordIDs,
	}
	payload := protocol.ReceiptRecorded{Receipt: receipt}
	envelope, err := protocol.NewEnvelope(protocol.EventReceiptRecorded,
		protocol.ActorRef{Kind: "agent", ID: threadID},
		protocol.TrustClaim{Level: "agent_asserted", Basis: "test"}, payload)
	if err != nil {
		return err
	}
	return appendRuntimeEnvelope(runtimeReceipts, envelope)
}

func runtimeReceiptPath(t *testing.T) string {
	t.Helper()
	root, err := sharedRuntimeRoot()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(root, runtimeReceipts, "corrupt.json")
}

func TestRelationStorageBranches(t *testing.T) {
	t.Run("appendRelation and parseNodeSpec branches", func(t *testing.T) {
		chdirTemp(t)
		mustRun(t, "init")

		if err := os.WriteFile(".git", []byte("x"), 0); err != nil {
			t.Fatal(err)
		}
		if err := appendRelation(protocol.Relation{}, DurabilityLive, protocol.ActorRef{Kind: "agent"}, protocol.TrustClaim{Level: "agent_asserted"}); err == nil {
			t.Fatal("appendRelation with unreadable .git returned nil error")
		}

		if _, err := parseNodeSpec("kind:provider:"); err == nil {
			t.Fatal("parseNodeSpec accepted empty third part")
		}
		if got := trustRank("unknown"); got != 0 {
			t.Fatalf("trustRank(unknown) = %d, want 0", got)
		}
	})

	t.Run("loadRelations and enrich error branches", func(t *testing.T) {
		chdirTemp(t)
		mustRun(t, "init")
		mustRun(t, "thread", "start", "--id", "thread-a", "--issue", "FAB-1")
		mustRun(t, "note", "--candidate", "--thread", "thread-a", "direction")

		if err := os.WriteFile(ledgerEventsPath+"/corrupt.json", []byte("{bad"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := loadRelations(); err == nil {
			t.Fatal("loadRelations accepted corrupt ledger")
		}
		if err := os.Remove(ledgerEventsPath + "/corrupt.json"); err != nil {
			t.Fatal(err)
		}

		if _, err := loadRelations(); err != nil {
			t.Fatalf("loadRelations after cleanup: %v", err)
		}

		root, err := sharedRuntimeRoot()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, runtimeReceipts, "bad.json"), []byte("{bad"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := loadRelations(); err == nil {
			t.Fatal("loadRelations accepted corrupt receipts")
		}
		if err := os.Remove(filepath.Join(root, runtimeReceipts, "bad.json")); err != nil {
			t.Fatal(err)
		}

		recordID := recordIDAt(t, 0)
		if err := validateSupersedesTrust(protocol.NodeRef{Kind: "record", ID: recordID}, protocol.NodeRef{Kind: "record", ID: recordID}); err != nil {
			t.Fatalf("validateSupersedesTrust same record: %v", err)
		}

		if err := os.WriteFile(ledgerEventsPath+"/corrupt.json", []byte("{bad"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := validateSupersedesTrust(protocol.NodeRef{Kind: "record", ID: recordID}, protocol.NodeRef{Kind: "record", ID: recordID}); err == nil {
			t.Fatal("validateSupersedesTrust accepted corrupt ledger")
		}

		if err := os.Remove(ledgerEventsPath + "/corrupt.json"); err != nil {
			t.Fatal(err)
		}
		if _, err := explainGraph(protocol.NodeRef{Kind: "record", ID: recordID}, "outgoing", []string{protocol.RelationInformedBy}, 1); err != nil {
			t.Fatalf("explainGraph: %v", err)
		}

		if err := os.WriteFile(filepath.Join(root, runtimeProjections, "bad.json"), []byte("{bad"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := explainGraph(protocol.NodeRef{Kind: "record", ID: recordID}, "outgoing", []string{protocol.RelationInformedBy}, 1); err == nil {
			t.Fatal("explainGraph accepted corrupt projections")
		}
	})
}

func TestIDGeneratorErrorsInRuntimeStore(t *testing.T) {
	chdirTemp(t)
	mustRun(t, "init")

	idErr := errors.New("id generation failed")

	previousProjectionID := newProjectionID
	newProjectionID = func() (string, error) { return "", idErr }
	defer func() { newProjectionID = previousProjectionID }()

	if _, err := createProjection("sync", "thread-a", protocol.Scope{Global: true}, nil, false); !errors.Is(err, idErr) {
		t.Fatalf("createProjection id err = %v", err)
	}
	newProjectionID = previousProjectionID

	projectionID, err := protocol.NewProjectionID()
	if err != nil {
		t.Fatal(err)
	}
	previousReceiptID := newReceiptID
	newReceiptID = func() (string, error) { return "", idErr }
	defer func() { newReceiptID = previousReceiptID }()

	if _, err := recordProjectionReceipt(protocol.Projection{ProjectionID: projectionID, ThreadID: "thread-a"}, protocol.ReceiptDelivered, "codex"); !errors.Is(err, idErr) {
		t.Fatalf("recordProjectionReceipt id err = %v", err)
	}
}

func TestIsActiveEventAndPromoteBranches(t *testing.T) {
	chdirTemp(t)
	mustRun(t, "init")
	mustRun(t, "note", "--durable", "--issue", "FAB-1", "Durable direction")
	mustRun(t, "note", "--live", "--issue", "FAB-1", "Live direction")

	events, err := loadEvents()
	if err != nil {
		t.Fatal(err)
	}
	var durableID, liveID string
	for _, event := range events {
		if event.Durability == core.DurabilityDurable {
			durableID = event.ID
		} else if event.Durability == core.DurabilityLive {
			liveID = event.ID
		}
	}
	if durableID == "" || liveID == "" {
		t.Fatal("failed to find durable and live records")
	}

	for _, status := range []string{"open", "accepted", "rejected"} {
		event := DirectionEvent{Status: status}
		if !isActiveEvent(event) {
			t.Fatalf("isActiveEvent(%q) = false", status)
		}
	}

	if _, err := promoteEvent(durableID, "reason"); err == nil {
		t.Fatal("promote durable returned nil error")
	}
	if _, err := promoteEvent(liveID, ""); err == nil {
		t.Fatal("promote without reason returned nil error")
	}
	if _, err := promoteEvent("rec-missing", "reason"); err == nil {
		t.Fatal("promote missing returned nil error")
	}
}

func TestLoadRuntimeThreadsOrderingAndReceiptUnmarshal(t *testing.T) {
	chdirTemp(t)
	mustRun(t, "init")

	if err := saveRuntimeThread(ThreadRecord{ThreadID: "thread-a", Issue: "FAB-1"}, protocol.EventThreadStarted); err != nil {
		t.Fatal(err)
	}
	threads, err := loadRuntimeThreads()
	if err != nil {
		t.Fatal(err)
	}
	if threads["thread-a"].Issue != "FAB-1" {
		t.Fatalf("loadRuntimeThreads = %#v", threads)
	}

	root, err := sharedRuntimeRoot()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, runtimeThreads), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, runtimeThreads, "bad.json"), []byte("{bad"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadRuntimeThreads(); err == nil {
		t.Fatal("loadRuntimeThreads accepted corrupt json")
	}

	if err := os.MkdirAll(filepath.Join(root, runtimeReceipts), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, runtimeReceipts, "bad.json"), []byte("{bad"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadReceipts(); err == nil {
		t.Fatal("loadReceipts accepted corrupt json")
	}
}

func TestInstallRootAgentsFileBranches(t *testing.T) {
	chdirTemp(t)
	block := "<!-- fabric:start -->\nblock\n<!-- fabric:end -->\n"

	if err := installRootAgentsFile("missing.txt", block); err != nil {
		t.Fatalf("installRootAgentsFile missing: %v", err)
	}
	if got := mustRead(t, "missing.txt"); got != block {
		t.Fatalf("missing file content = %q", got)
	}

	if err := os.WriteFile("existing.txt", []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := installRootAgentsFile("existing.txt", block); err != nil {
		t.Fatalf("installRootAgentsFile existing: %v", err)
	}
	got := mustRead(t, "existing.txt")
	if !strings.Contains(got, "hello") || !strings.Contains(got, "block") {
		t.Fatalf("existing file content = %q", got)
	}

	if err := os.WriteFile("block.txt", []byte("before\n"+block+"after\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := installRootAgentsFile("block.txt", "<!-- fabric:start -->\nnew\n<!-- fabric:end -->\n"); err != nil {
		t.Fatalf("installRootAgentsFile replace: %v", err)
	}
	got = mustRead(t, "block.txt")
	if !strings.Contains(got, "new") || strings.Contains(got, "block") {
		t.Fatalf("replace content = %q", got)
	}
}

func TestIsActiveEventEmptyStatus(t *testing.T) {
	if !isActiveEvent(DirectionEvent{}) {
		t.Fatal("empty status should be active")
	}
}

func TestPromoteTrustRankAndLedgerLoadError(t *testing.T) {
	chdirTemp(t)
	mustRun(t, "init")

	lowTrust := DirectionEvent{
		Kind:       "note",
		CreatedAt:  nowString(),
		Durability: core.DurabilityCandidate,
		Scope:      core.EventScope{Repo: repoName(), Issue: "FAB-1", Global: true},
		Source:     core.EventSource{Type: "agent"},
		Text:       "low trust",
		Confidence: "agent_asserted",
		TTL:        "until_issue_closed",
	}
	if err := appendEvent(&lowTrust); err != nil {
		t.Fatal(err)
	}
	if _, err := promoteEvent(lowTrust.ID, "reason"); err == nil {
		t.Fatal("promote low-trust event returned nil error")
	}

	if err := os.WriteFile(ledgerEventsPath+"/corrupt.json", []byte("{bad"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := promoteEvent(lowTrust.ID, "reason"); err == nil {
		t.Fatal("promote with corrupt ledger returned nil error")
	}
}

func TestInstallRootAgentsFileEmptyAndNoNewline(t *testing.T) {
	chdirTemp(t)
	block := "<!-- fabric:start -->\nblock\n<!-- fabric:end -->\n"

	if err := os.WriteFile("empty.txt", []byte("   \n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := installRootAgentsFile("empty.txt", block); err != nil {
		t.Fatalf("installRootAgentsFile empty: %v", err)
	}
	got := mustRead(t, "empty.txt")
	if got != block {
		t.Fatalf("empty file content = %q, want %q", got, block)
	}

	if err := os.WriteFile("nonewline.txt", []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := installRootAgentsFile("nonewline.txt", block); err != nil {
		t.Fatalf("installRootAgentsFile no newline: %v", err)
	}
	got = mustRead(t, "nonewline.txt")
	if !strings.Contains(got, "hello\n\n") || !strings.Contains(got, "block") {
		t.Fatalf("no newline content = %q", got)
	}
}

func TestGitCommonDirFileBranch(t *testing.T) {
	chdirTemp(t)
	if err := os.WriteFile(".git", []byte("not a gitdir file"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, err := gitCommonDir(); err != nil || got != "" {
		t.Fatalf("gitCommonDir file without gitdir = %q, %v", got, err)
	}
}

func TestLedgerHealthBranches(t *testing.T) {
	chdirTemp(t)
	mustRun(t, "init")

	report, err := ledgerHealth()
	if err != nil {
		t.Fatalf("ledgerHealth: %v", err)
	}
	if !report.DurableLedgerOK {
		t.Fatalf("ledgerHealth durable not ok: %s", report.DurableLedgerError)
	}
	if report.SharedMirrorOK {
		t.Fatal("ledgerHealth shared mirror should not be ok outside git")
	}

	if err := os.WriteFile(ledgerEventsPath+"/corrupt.json", []byte("{bad"), 0o644); err != nil {
		t.Fatal(err)
	}
	report, err = ledgerHealth()
	if err != nil {
		t.Fatal(err)
	}
	if report.DurableLedgerOK {
		t.Fatal("ledgerHealth accepted corrupt ledger")
	}
	if err := os.Remove(ledgerEventsPath + "/corrupt.json"); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(".git", []byte("gitdir:"+filepath.Join(mustGetwd(), "gitdir")), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir("gitdir", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join("gitdir", "commondir"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := ledgerHealth(); err == nil {
		t.Fatal("ledgerHealth with broken git returned nil error")
	}
}

func TestSeenStaleReceiptLoadError(t *testing.T) {
	chdirTemp(t)
	mustRun(t, "init")

	evt1, err := protocol.NewEventID()
	if err != nil {
		t.Fatal(err)
	}
	if err := writeTestReceipt(t, "thread-a", []string{evt1}, nil); err != nil {
		t.Fatal(err)
	}
	root, err := sharedRuntimeRoot()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, runtimeReceipts, "bad.json"), []byte("{bad"), 0o644); err != nil {
		t.Fatal(err)
	}

	threads := map[string]ThreadRecord{"thread-a": {ThreadID: "thread-a", Issue: "FAB-1"}}
	event := DirectionEvent{HeadEventID: evt1, Status: core.StatusActive, Scope: core.EventScope{Issue: "FAB-1"}}
	if _, _, err := seenAndStaleFromReceipts(event, threads); err == nil {
		t.Fatal("seenAndStaleFromReceipts accepted corrupt receipts")
	}
}

func TestLoadRuntimeThreadsUpdateOrdering(t *testing.T) {
	chdirTemp(t)
	mustRun(t, "init")

	old := ThreadRecord{ThreadID: "thread-a", Issue: "FAB-1", CreatedAt: "2020-01-01T00:00:00Z", UpdatedAt: "2020-01-01T00:00:00Z"}
	new := ThreadRecord{ThreadID: "thread-a", Issue: "FAB-2", CreatedAt: time.Now().Format(time.RFC3339Nano), UpdatedAt: time.Now().Format(time.RFC3339Nano)}
	if err := saveRuntimeThread(old, protocol.EventThreadStarted); err != nil {
		t.Fatal(err)
	}
	if err := saveRuntimeThread(new, protocol.EventThreadScopeChanged); err != nil {
		t.Fatal(err)
	}
	threads, err := loadRuntimeThreads()
	if err != nil {
		t.Fatal(err)
	}
	if threads["thread-a"].Issue != "FAB-2" {
		t.Fatalf("loadRuntimeThreads did not apply newer update: %#v", threads["thread-a"])
	}

	older := ThreadRecord{ThreadID: "thread-a", Issue: "FAB-3", CreatedAt: "2019-01-01T00:00:00Z", UpdatedAt: "2019-01-01T00:00:00Z"}
	if err := saveRuntimeThread(older, protocol.EventThreadScopeChanged); err != nil {
		t.Fatal(err)
	}
	threads, err = loadRuntimeThreads()
	if err != nil {
		t.Fatal(err)
	}
	if threads["thread-a"].Issue != "FAB-2" {
		t.Fatalf("loadRuntimeThreads applied older update: %#v", threads["thread-a"])
	}
}

func TestRelationCommandRunErrorBranches(t *testing.T) {
	chdirTemp(t)
	mustRun(t, "init")

	if err := Run([]string{"relation", "add", "--unknown"}); err == nil {
		t.Fatal("relation add accepted unknown flag")
	}

	if err := os.WriteFile(".git", []byte("gitdir:"+filepath.Join(mustGetwd(), "gitdir")), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir("gitdir", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join("gitdir", "commondir"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"relation", "add", "--type", "informed_by", "--from", "record:rec-1", "--to", "record:rec-2"}); err == nil {
		t.Fatal("relation add with broken git returned nil error")
	}
}

func TestBrokenGitPropagatesToRuntimeAndRepository(t *testing.T) {
	chdirTemp(t)
	mustRun(t, "init")

	if err := os.WriteFile(".git", []byte("gitdir:"+filepath.Join(mustGetwd(), "gitdir")), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir("gitdir", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join("gitdir", "commondir"), 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := sharedRuntimeRoot(); err == nil {
		t.Fatal("sharedRuntimeRoot with broken git returned nil error")
	}
	if _, err := sharedRuntimePath(runtimeThreads); err == nil {
		t.Fatal("sharedRuntimePath with broken git returned nil error")
	}
	if _, err := runtimeFileStore(); err == nil {
		t.Fatal("runtimeFileStore with broken git returned nil error")
	}
	if _, err := loadRuntimeEvents(runtimeThreads); err == nil {
		t.Fatal("loadRuntimeEvents with broken git returned nil error")
	}
	if err := appendRuntimeEnvelope(runtimeThreads, protocol.EventEnvelope{}); err == nil {
		t.Fatal("appendRuntimeEnvelope with broken git returned nil error")
	}
	if err := saveRuntimeThread(ThreadRecord{ThreadID: "thread-a"}, protocol.EventThreadStarted); err == nil {
		t.Fatal("saveRuntimeThread with broken git returned nil error")
	}
	if _, err := createProjection("sync", "thread-a", protocol.Scope{Global: true}, nil, false); err == nil {
		t.Fatal("createProjection with broken git returned nil error")
	}
	projectionID, err := protocol.NewProjectionID()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := recordProjectionReceipt(protocol.Projection{ProjectionID: projectionID, ThreadID: "thread-a"}, protocol.ReceiptDelivered, "codex"); err == nil {
		t.Fatal("recordProjectionReceipt with broken git returned nil error")
	}

	if _, err := directionRepository(); err == nil {
		t.Fatal("directionRepository with broken git returned nil error")
	}
	if err := appendDirection(&DirectionEvent{}); err == nil {
		t.Fatal("appendDirection with broken git returned nil error")
	}
	if _, err := appendDirectionState(DirectionEvent{}, DirectionEvent{}, "reason"); err == nil {
		t.Fatal("appendDirectionState with broken git returned nil error")
	}
	if _, _, err := loadDirectionsUnlocked(); err == nil {
		t.Fatal("loadDirectionsUnlocked with broken git returned nil error")
	}
	if _, _, err := loadProtocolEventsUnlocked(); err == nil {
		t.Fatal("loadProtocolEventsUnlocked with broken git returned nil error")
	}
}

func TestRuntimeKindAndTrustRankBranches(t *testing.T) {
	if got := runtimeKindForEvent(protocol.EventRelationCreated); got != filestore.RuntimeRelations {
		t.Fatalf("runtimeKindForEvent(relation.created) = %q, want %q", got, filestore.RuntimeRelations)
	}
	if got := trustRank("tool_verified"); got != 2 {
		t.Fatalf("trustRank(tool_verified) = %d, want 2", got)
	}
}

func TestLedgerHealthSharedDirLoadError(t *testing.T) {
	chdirTemp(t)
	mustRun(t, "init")

	if err := os.Mkdir(".git", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(".git", "fabric"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(".git", "fabric", "events"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	report, err := ledgerHealth()
	if err != nil {
		t.Fatalf("ledgerHealth: %v", err)
	}
	if report.SharedMirrorOK {
		t.Fatal("ledgerHealth shared mirror should not be ok when events is a file")
	}
}

func TestRelationGraphWithCorruptRuntimeData(t *testing.T) {
	chdirTemp(t)
	mustRun(t, "init")
	mustRun(t, "thread", "start", "--id", "thread-a", "--issue", "FAB-1")
	mustRun(t, "note", "--candidate", "--thread", "thread-a", "direction")
	recordID := recordIDAt(t, 0)

	root, err := sharedRuntimeRoot()
	if err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join(root, runtimeReceipts), 0o755); err != nil {
		t.Fatal(err)
	}
	receiptEnv, err := protocol.NewEnvelope(protocol.EventReceiptRecorded,
		protocol.ActorRef{Kind: "agent", ID: "thread-a"},
		protocol.TrustClaim{Level: "agent_asserted", Basis: "test"},
		protocol.ReceiptRecorded{Receipt: protocol.Receipt{
			ReceiptID:    "invalid-id",
			ProjectionID: "prj-invalid",
			ThreadID:     "thread-a",
			State:        protocol.ReceiptDelivered,
			OccurredAt:   time.Now().Format(time.RFC3339Nano),
		}})
	if err != nil {
		t.Fatal(err)
	}
	receiptData, err := json.Marshal(receiptEnv)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, runtimeReceipts, "bad.json"), receiptData, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadRelations(); err == nil {
		t.Fatal("loadRelations accepted invalid receipt ID")
	}
	if _, err := explainGraph(protocol.NodeRef{Kind: "record", ID: recordID}, "outgoing", []string{protocol.RelationInformedBy}, 1); err == nil {
		t.Fatal("explainGraph accepted invalid receipt ID")
	}

	if err := os.Remove(filepath.Join(root, runtimeReceipts, "bad.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, runtimeReceipts, "bad.json"), []byte("{bad"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := explainGraph(protocol.NodeRef{Kind: "record", ID: recordID}, "outgoing", []string{protocol.RelationInformedBy}, 1); err == nil {
		t.Fatal("explainGraph accepted corrupt receipts")
	}

	if err := os.Remove(filepath.Join(root, runtimeReceipts, "bad.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, runtimeProjections, "bad.json"), []byte("{bad"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := explainGraph(protocol.NodeRef{Kind: "record", ID: recordID}, "outgoing", []string{protocol.RelationInformedBy}, 1); err == nil {
		t.Fatal("explainGraph accepted corrupt projections")
	}

	if err := os.Remove(filepath.Join(root, runtimeProjections, "bad.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, runtimeThreads, "bad.json"), []byte("{bad"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := explainGraph(protocol.NodeRef{Kind: "record", ID: recordID}, "outgoing", []string{protocol.RelationInformedBy}, 1); err == nil {
		t.Fatal("explainGraph accepted corrupt threads")
	}
}

func TestInstallRootAgentsFileRemainingBranches(t *testing.T) {
	chdirTemp(t)
	block := "<!-- fabric:start -->\nblock\n<!-- fabric:end -->\n"

	if err := os.WriteFile("empty.txt", []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := installRootAgentsFile("empty.txt", block); err != nil {
		t.Fatalf("installRootAgentsFile empty: %v", err)
	}
	if got := mustRead(t, "empty.txt"); got != block {
		t.Fatalf("empty file content = %q, want %q", got, block)
	}

	if err := os.WriteFile("nonewline.txt", []byte("before\n"+block+"after"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := installRootAgentsFile("nonewline.txt", "<!-- fabric:start -->\nnew\n<!-- fabric:end -->\n"); err != nil {
		t.Fatalf("installRootAgentsFile replace no newline: %v", err)
	}
	got := mustRead(t, "nonewline.txt")
	if !strings.HasSuffix(got, "\n") || strings.Contains(got, "block") {
		t.Fatalf("replace no newline content = %q", got)
	}

	if err := os.Mkdir("dirpath.txt", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := installRootAgentsFile("dirpath.txt", block); err == nil {
		t.Fatal("installRootAgentsFile with directory path returned nil error")
	}
}

func TestLedgerHealthMismatchDetection(t *testing.T) {
	chdirTemp(t)
	if err := os.Mkdir(".git", 0o755); err != nil {
		t.Fatal(err)
	}
	mustRun(t, "init")
	mustRun(t, "note", "--durable", "--issue", "FAB-1", "Durable direction")

	shared, err := sharedEventDir()
	if err != nil {
		t.Fatal(err)
	}
	if shared == "" {
		t.Fatal("shared event dir is empty despite .git")
	}

	events, _, err := filestore.Load(ledgerEventsPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) == 0 {
		t.Fatal("no durable events found")
	}
	localEventID := events[0].EventID

	entries, err := os.ReadDir(shared)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if err := os.Remove(filepath.Join(shared, entry.Name())); err != nil {
			t.Fatal(err)
		}
	}

	report, err := ledgerHealth()
	if err != nil {
		t.Fatalf("ledgerHealth: %v", err)
	}
	if len(report.DurableSharedMismatches) != 1 || !strings.Contains(report.DurableSharedMismatches[0], localEventID) {
		t.Fatalf("ledgerHealth mismatches = %v, want event %s", report.DurableSharedMismatches, localEventID)
	}
}

func TestRelevantUndeliveredReceiptError(t *testing.T) {
	chdirTemp(t)
	mustRun(t, "init")

	evt1, err := protocol.NewEventID()
	if err != nil {
		t.Fatal(err)
	}
	if err := writeTestReceipt(t, "thread-a", []string{evt1}, nil); err != nil {
		t.Fatal(err)
	}

	root, err := sharedRuntimeRoot()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, runtimeReceipts, "bad.json"), []byte("{bad"), 0o644); err != nil {
		t.Fatal(err)
	}

	thread := ThreadRecord{ThreadID: "thread-a", Issue: "FAB-1"}
	event := DirectionEvent{HeadEventID: evt1, Status: core.StatusActive, Scope: core.EventScope{Issue: "FAB-1"}}
	if _, err := relevantUndelivered([]DirectionEvent{event}, thread); err == nil {
		t.Fatal("relevantUndelivered accepted corrupt receipts")
	}
}

func TestExplainAndEnrichGraphLedgerErrorBranches(t *testing.T) {
	chdirTemp(t)
	mustRun(t, "init")
	mustRun(t, "thread", "start", "--id", "thread-a", "--issue", "FAB-1")
	mustRun(t, "note", "--candidate", "--thread", "thread-a", "direction")
	recordID := recordIDAt(t, 0)

	if err := os.WriteFile(ledgerEventsPath+"/corrupt.json", []byte("{bad"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := explainGraph(protocol.NodeRef{Kind: "record", ID: recordID}, "outgoing", []string{protocol.RelationInformedBy}, 1); err == nil {
		t.Fatal("explainGraph accepted corrupt ledger")
	}

	if err := os.Remove(ledgerEventsPath + "/corrupt.json"); err != nil {
		t.Fatal(err)
	}

	root, err := sharedRuntimeRoot()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, runtimeThreads), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, runtimeThreads, "bad.json"), []byte("{bad"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := explainGraph(protocol.NodeRef{Kind: "record", ID: recordID}, "outgoing", []string{protocol.RelationInformedBy}, 1); err == nil {
		t.Fatal("explainGraph accepted corrupt threads for enrichGraph")
	}
}
