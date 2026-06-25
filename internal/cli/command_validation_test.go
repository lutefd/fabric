package cli

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/lutefd/fabric/protocol"
)

func TestChallengeCleanConformanceAndContextValidation(t *testing.T) {
	chdirTemp(t)

	for _, args := range [][]string{
		{"challenge"},
		{"challenge", "--direction", "rec_missing"},
		{"challenge", "--direction", "rec_missing", "--proposal", "try this"},
		{"challenge", "resolve"},
		{"challenge", "resolve", "rec_missing", "--accepted", "--rejected"},
		{"clean"},
		{"clean", "unknown"},
		{"context"},
		{"context", "ack"},
		{"context", "acknowledge"},
		{"context", "acknowledge", "--projection", "prj_missing", "--state", "seen"},
		{"conformance"},
	} {
		if err := Run(args); err == nil {
			t.Fatalf("Run(%v) succeeded before init", args)
		}
	}

	mustRun(t, "init")
	mustRun(t, "thread", "start", "--id", "thread-a", "--issue", "FAB-1")
	mustRun(t, "note", "--candidate", "Direction")
	directionID := recordIDAt(t, 0)

	for _, args := range [][]string{
		{"challenge", "--direction", directionID},
		{"challenge", "--direction", directionID, "--proposal", "try this"},
		{"challenge", "--direction", "rec_missing", "--issue", "FAB-1", "--proposal", "try this"},
		{"challenge", "resolve", "rec_missing", "--accepted"},
		{"clean", "live", "extra"},
		{"clean", "live", "--record", directionID},
		{"clean", "runtime"},
		{"clean", "runtime", "extra", "--thread", "thread-a"},
		{"context", "acknowledge", "--projection", "prj_missing"},
	} {
		if err := Run(args); err == nil {
			t.Fatalf("Run(%v) succeeded", args)
		}
	}

	mustRun(t, "challenge", "--direction", directionID, "--issue", "FAB-1", "--proposal", "Alternative")
	challengeID := recordIDAt(t, 1)
	mustRun(t, "challenge", "resolve", challengeID, "--accepted")
	if err := Run([]string{"challenge", "resolve", challengeID, "--accepted"}); err == nil {
		t.Fatal("resolved challenge was resolved twice")
	}
}

func TestCommandParseAndArityValidation(t *testing.T) {
	chdirTemp(t)
	for _, args := range [][]string{
		{"init", "--bad"},
		{"install-agents", "--bad"},
		{"thread"},
		{"thread", "unknown"},
		{"promote", "--bad"},
		{"sync", "--bad"},
		{"status", "--bad"},
		{"doctor", "--bad"},
		{"preflight"},
		{"preflight", "task"},
		{"preflight", "task", "--issue"},
		{"preflight", "task", "--area"},
		{"preflight", "task", "--path"},
		{"preflight", "task", "--budget"},
		{"preflight", "task", "--budget", "nope"},
		{"preflight", "task", "--budget=nope"},
		{"explain", "--bad"},
		{"explain"},
	} {
		if err := Run(args); err == nil {
			t.Fatalf("Run(%v) succeeded before init", args)
		}
	}

	mustRun(t, "init")
	for _, args := range [][]string{
		{"install-agents", "--bad"},
		{"thread", "list", "extra"},
		{"thread", "use"},
		{"thread", "use", "missing"},
		{"thread", "clear", "extra"},
		{"thread", "start", "--bad"},
		{"thread", "start", "--id", "empty-scope"},
		{"note", "--live", "--candidate", "--global", "too many durability flags"},
		{"promote"},
		{"promote", "rec_missing"},
		{"sync", "--thread", "missing"},
		{"status", "--bad"},
		{"doctor", "--bad"},
		{"explain", "--direction", "sideways", "--node", "record:rec_missing"},
		{"explain", "--node", "bad"},
	} {
		if err := Run(args); err == nil {
			t.Fatalf("Run(%v) succeeded", args)
		}
	}
}

func TestInitializationAndFilesystemFailures(t *testing.T) {
	t.Run("init fails when fabric root is a file", func(t *testing.T) {
		chdirTemp(t)
		if err := os.WriteFile(".fabric", []byte("not a directory"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := Run([]string{"init"}); err == nil {
			t.Fatal("init succeeded with .fabric as a file")
		}
	})

	t.Run("init fails when agents root is a file", func(t *testing.T) {
		chdirTemp(t)
		if err := os.WriteFile(".agents", []byte("not a directory"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := Run([]string{"init"}); err == nil {
			t.Fatal("init succeeded with .agents as a file")
		}
	})

	t.Run("install agents propagates write failures", func(t *testing.T) {
		chdirTemp(t)
		mustRun(t, "init")
		if err := os.RemoveAll(repoAgentSkillsRoot); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(repoAgentSkillsRoot, []byte("not a directory"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := Run([]string{"install-agents"}); err == nil {
			t.Fatal("install-agents succeeded with repo skills root as a file")
		}
	})
}

func TestIDGenerationFailuresPropagate(t *testing.T) {
	chdirTemp(t)
	mustRun(t, "init")
	idErr := errors.New("id generation failed")

	previousThreadID := newThreadID
	newThreadID = func() (string, error) { return "", idErr }
	if err := Run([]string{"thread", "start", "--issue", "FAB-1"}); !errors.Is(err, idErr) {
		t.Fatalf("thread start err = %v", err)
	}
	newThreadID = previousThreadID

	previousRelationID := newRelationID
	newRelationID = func() (string, error) { return "", idErr }
	if err := Run([]string{"relation", "add", "--type", "informed_by", "--from", "record:rec_1", "--to", "thread:thread-a"}); !errors.Is(err, idErr) {
		t.Fatalf("relation err = %v", err)
	}
	newRelationID = previousRelationID

	previousProjectionID := newProjectionID
	newProjectionID = func() (string, error) { return "", idErr }
	if _, err := createProjection("sync", "thread-a", protocol.Scope{Global: true}, nil, false); !errors.Is(err, idErr) {
		t.Fatalf("projection err = %v", err)
	}
	newProjectionID = previousProjectionID

	projectionID, err := protocol.NewProjectionID()
	if err != nil {
		t.Fatal(err)
	}
	previousReceiptID := newReceiptID
	newReceiptID = func() (string, error) { return "", idErr }
	if _, err := recordProjectionReceipt(protocol.Projection{ProjectionID: projectionID, ThreadID: "thread-a", Scope: protocol.Scope{Global: true}}, protocol.ReceiptDelivered, "codex"); !errors.Is(err, idErr) {
		t.Fatalf("receipt err = %v", err)
	}
	newReceiptID = previousReceiptID
}

func TestCleanSharedFilesAndContainsSelectedBranches(t *testing.T) {
	chdirTemp(t)
	mustRun(t, "init")

	output := captureStdout(t, func() {
		if err := cleanSharedFiles("record", nil, false, true); err != nil {
			t.Fatal(err)
		}
	})
	assertContains(t, output, "Nothing eligible")

	if containsSelected(float64(1), map[string]bool{"1": true}) {
		t.Fatal("numeric JSON value matched selected IDs")
	}
	if !containsSelected(map[string]any{"nested": []any{"rec-1"}}, map[string]bool{"rec-1": true}) {
		t.Fatal("nested selected value not found")
	}

}

func TestConformanceReportsInvalidFiles(t *testing.T) {
	chdirTemp(t)
	mustRun(t, "init")

	valid := filepath.Join(t.TempDir(), "valid.json")
	event := protocolEnvelopeForCLI(t)
	if err := os.WriteFile(valid, mustMarshalJSON(t, event), 0o644); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(t.TempDir(), "missing.json")
	invalid := filepath.Join(t.TempDir(), "invalid.json")
	if err := os.WriteFile(invalid, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Run([]string{"conformance", "--file", valid, "--file", missing, invalid}); err == nil {
		t.Fatal("conformance accepted invalid files")
	}
}

func protocolEnvelopeForCLI(t *testing.T) protocol.EventEnvelope {
	t.Helper()
	recordID, _ := protocol.NewRecordID()
	event, err := protocol.NewEnvelope(protocol.EventRecordCreated,
		protocol.ActorRef{Kind: "human"},
		protocol.TrustClaim{Level: "human_confirmed"},
		protocol.RecordCreated{Record: protocol.Record{
			RecordID: recordID, Kind: "direction", CreatedAt: "2026-06-25T00:00:00Z",
			Scope: protocol.Scope{Global: true}, Source: protocol.SourceRef{Type: "human"},
			Text: "Direction", Confidence: "human_confirmed", TTL: "until_superseded",
			Status: "active", Durability: "durable",
		}})
	if err != nil {
		t.Fatal(err)
	}
	return event
}

func mustMarshalJSON(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
