package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/lutefd/fabric/protocol"
)

func TestRelationCommandValidationBranches(t *testing.T) {
	chdirTemp(t)
	mustRun(t, "init")

	for _, args := range [][]string{
		{"relation"},
		{"relation", "remove"},
		{"relation", "add", "--type", "unknown", "--from", "record:a", "--to", "record:b"},
		{"relation", "add", "--type", "informed_by", "--from", "record", "--to", "record:b"},
		{"relation", "add", "--type", "informed_by", "--from", "record:a", "--to", "record:"},
		{"relation", "add", "--type", "informed_by", "--from", "record:a", "--to", "record:b", "--candidate", "--durable", "--reason", "x"},
		{"relation", "add", "--type", "informed_by", "--from", "record:a", "--to", "record:b", "--durable"},
		{"relation", "add", "--type", "supersedes", "--from", "action:a", "--to", "record:b"},
	} {
		if err := Run(args); err == nil {
			t.Fatalf("Run(%v) succeeded", args)
		}
	}
}

func TestRelationCommandRecordsLiveCandidateAndDurableRelations(t *testing.T) {
	chdirTemp(t)
	mustRun(t, "init")
	mustRun(t, "thread", "start", "--id", "thread-a", "--issue", "FAB-1")

	mustRun(t, "relation", "add", "--type", "informed_by", "--from", "action:codex:msg-1", "--to", "thread:thread-a", "--from-url", "https://example.invalid/from", "--to-url", "https://example.invalid/to")
	mustRun(t, "relation", "add", "--type", "implements", "--from", "commit:abc", "--to", "record:rec-1", "--candidate")
	mustRun(t, "relation", "add", "--type", "derived_from", "--from", "record:rec-1", "--to", "message:m1", "--durable", "--reason", "stable provenance")

	relations, err := loadRelations()
	if err != nil {
		t.Fatal(err)
	}
	if len(relations) != 3 {
		t.Fatalf("relations = %#v", relations)
	}
	if relations[0].RelationID > relations[1].RelationID {
		t.Fatalf("relations not sorted: %#v", relations)
	}
}

func TestSupersedesTrustValidationBranches(t *testing.T) {
	chdirTemp(t)
	mustRun(t, "init")
	mustRun(t, "note", "--candidate", "--global", "Human direction")
	humanID := recordIDAt(t, 0)
	if err := validateSupersedesTrust(protocol.NodeRef{Kind: "record", ID: "missing"}, protocol.NodeRef{Kind: "record", ID: humanID}); err == nil {
		t.Fatal("missing record accepted for supersedes")
	}
	if trustRank("repository_reviewed") <= trustRank("human_confirmed") || trustRank("unknown") != 0 {
		t.Fatal("trust rank ordering changed")
	}
}

func TestExplainGraphValidationAndEmptyOutput(t *testing.T) {
	chdirTemp(t)
	mustRun(t, "init")

	if err := Run([]string{"explain", "--node", "record:rec-1", "--direction", "sideways"}); err == nil {
		t.Fatal("invalid graph direction accepted")
	}
	if _, err := explainGraph(protocol.NodeRef{Kind: "record", ID: "rec-1"}, "", []string{protocol.RelationInformedBy}, 0); err != nil {
		t.Fatal(err)
	}
	output := captureStdout(t, func() {
		printGraph(protocol.Graph{Root: protocol.NodeRef{Kind: "record", ID: "rec-1"}})
	})
	assertContains(t, output, "No provenance relations found.")
}

func TestExplainGraphIncludesReceiptAndNodeDetails(t *testing.T) {
	chdirTemp(t)
	mustRun(t, "init")
	mustRun(t, "thread", "start", "--id", "source-thread", "--issue", "FAB-1")
	mustRun(t, "note", "--candidate", "--thread", "source-thread", "Shared direction")
	recordID := recordIDAt(t, 0)
	mustRun(t, "thread", "start", "--id", "target-thread", "--issue", "FAB-1")
	mustRun(t, "sync", "--thread", "target-thread")

	projections, err := loadRuntimeEvents(runtimeProjections)
	if err != nil {
		t.Fatal(err)
	}
	if len(projections) == 0 {
		t.Fatal("sync did not create a projection")
	}
	projectionID := ""
	for _, event := range projections {
		var payload protocol.ProjectionCreated
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			t.Fatal(err)
		}
		if payload.Projection.ThreadID == "target-thread" && len(payload.Projection.RecordIDs) != 0 {
			projectionID = payload.Projection.ProjectionID
		}
	}
	if projectionID == "" {
		t.Fatal("target projection not found")
	}
	mustRun(t, "context", "acknowledge", "--projection", projectionID, "--state", "exposed", "--provider", "codex")

	graph, err := explainGraph(protocol.NodeRef{Kind: "record", ID: recordID}, "outgoing", nil, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(graph.Relations) == 0 || len(graph.RelationDetails) == 0 || len(graph.NodeDetails) == 0 {
		t.Fatalf("graph was not enriched: %#v", graph)
	}
	output := captureStdout(t, func() { printGraph(graph) })
	assertContains(t, output, "availability, not proof of influence")
	assertContains(t, output, "Shared direction")
}

func TestPrintGraphShowsRelationReasonAndRecordConflict(t *testing.T) {
	graph := protocol.Graph{
		Root: protocol.NodeRef{Kind: "record", ID: "rec-1"},
		Relations: []protocol.Relation{{
			RelationID: "rel-1", Type: protocol.RelationInformedBy,
			From: protocol.NodeRef{Kind: "record", ID: "rec-1"}, To: protocol.NodeRef{Kind: "message", ID: "m1"},
			Reason: "because",
		}},
		RelationDetails: []protocol.RelationDetail{{RelationID: "rel-1", Actor: protocol.ActorRef{Kind: "human"}, Trust: protocol.TrustClaim{Level: "human_confirmed"}}},
		NodeDetails: []protocol.NodeDetail{{Ref: protocol.NodeRef{Kind: "record", ID: "rec-1"}, Record: &protocol.RecordNodeDetail{
			Record: protocol.Record{RecordID: "rec-1", Text: "direction", Status: "active", Reason: "rationale"},
			Trust:  protocol.TrustClaim{Level: "human_confirmed"}, HeadEventID: "evt-1", HeadActor: protocol.ActorRef{Kind: "human"}, HeadTrust: protocol.TrustClaim{Level: "human_confirmed"},
			Conflict: &protocol.MaterializationConflict{Message: "competing children"},
		}}},
	}
	output := captureStdout(t, func() { printGraph(graph) })
	for _, want := range []string{"reason: because", "asserted by: human", "rationale", "latest revision", "conflict: competing children"} {
		if !strings.Contains(output, want) {
			t.Fatalf("printGraph output missing %q:\n%s", want, output)
		}
	}
}
