package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/lutefd/fabric/protocol"
)

func TestCausalRelationsAndExposureRemainDistinct(t *testing.T) {
	chdirTemp(t)
	mustRun(t, "init")
	mustRun(t, "thread", "start", "--id", "thread-a", "--issue", "FAB-1")
	mustRun(t, "note", "--candidate", "--reason", "shared protocol boundary", "Use immutable protocol events")
	recordID := recordIDAt(t, 0)

	mustRun(t, "relation", "add", "--type", "informed_by",
		"--from", "action:codex:action-1", "--to", "record:"+recordID,
		"--actor-provider", "codex", "--actor-id", "action-1")
	causal := captureStdout(t, func() {
		mustRun(t, "explain", "--node", "action:codex:action-1")
	})
	assertContains(t, causal, "--informed_by-->")
	assertContains(t, causal, "[causal]")
	assertContains(t, causal, "- text: Use immutable protocol events")
	assertContains(t, causal, "- rationale: shared protocol boundary")
	assertContains(t, causal, "- creation trust: human_confirmed")
	assertContains(t, causal, "asserted by: agent (agent_asserted)")
	machine := captureStdout(t, func() {
		mustRun(t, "explain", "--node", "action:codex:action-1", "--json")
	})
	assertContains(t, machine, `"node_details"`)
	assertContains(t, machine, `"reason":"shared protocol boundary"`)
	assertContains(t, machine, `"trust":{"level":"human_confirmed"`)
	assertContains(t, machine, `"relation_details"`)
	assertContains(t, machine, `"actor":{"kind":"agent","id":"action-1","provider":"codex"}`)
	assertContains(t, machine, `"head_actor":{"kind":"human"`)

	graph, err := explainGraph(protocol.NodeRef{Kind: "action", Provider: "codex", ID: "action-1"}, "both", nil, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(graph.NodeDetails) == 0 || graph.NodeDetails[0].Record == nil && len(graph.NodeDetails) == 1 {
		t.Fatalf("graph lacks resolved record details: %#v", graph.NodeDetails)
	}
	if len(graph.RelationDetails) == 0 || graph.RelationDetails[0].Actor.Kind == "" {
		t.Fatalf("graph lacks relation assertion details: %#v", graph.RelationDetails)
	}

	projectionEvents, err := loadRuntimeEvents(runtimeProjections)
	if err != nil || len(projectionEvents) == 0 {
		t.Fatalf("projection events=%d err=%v", len(projectionEvents), err)
	}
	var payload protocol.ProjectionCreated
	if err := json.Unmarshal(projectionEvents[len(projectionEvents)-1].Payload, &payload); err != nil {
		t.Fatal(err)
	}
	mustRun(t, "context", "acknowledge", "--projection", payload.Projection.ProjectionID,
		"--state", "exposed", "--provider", "codex")

	availability := captureStdout(t, func() {
		mustRun(t, "explain", "--node", "record:"+recordID)
	})
	assertContains(t, availability, "--exposed_to-->")
	assertContains(t, availability, "[availability, not proof of influence]")

	graph, err = explainGraph(protocol.NodeRef{Kind: "record", ID: recordID}, "both", nil, 4)
	if err != nil {
		t.Fatal(err)
	}
	var hasProjection, hasThread bool
	for _, detail := range graph.NodeDetails {
		hasProjection = hasProjection || detail.Projection != nil
		hasThread = hasThread || detail.Thread != nil
	}
	if !hasProjection || !hasThread {
		t.Fatalf("availability path lacks projection/thread details: %#v", graph.NodeDetails)
	}
}

func TestLowerTrustCannotSupersedeHumanDirection(t *testing.T) {
	chdirTemp(t)
	mustRun(t, "init")
	mustRun(t, "note", "--candidate", "--global", "Human direction")
	humanID := recordIDAt(t, 0)
	agent := DirectionEvent{
		Kind: "finding", CreatedAt: nowString(), Durability: DurabilityCandidate,
		Scope: EventScope{Global: true}, Source: EventSource{Type: "agent"},
		Text: "Agent alternative", Confidence: "agent_asserted", TTL: "until_issue_closed",
	}
	if err := appendEvent(&agent); err != nil {
		t.Fatal(err)
	}
	err := Run([]string{"relation", "add", "--type", "supersedes",
		"--from", "record:" + agent.ID, "--to", "record:" + humanID})
	if err == nil || !strings.Contains(err.Error(), "lower-trust") {
		t.Fatalf("supersession error = %v", err)
	}
}

func TestMachineCapabilitiesAndLedgerConformance(t *testing.T) {
	chdirTemp(t)
	mustRun(t, "init")
	mustRun(t, "note", "--candidate", "--global", "Protocol direction")

	output := captureStdout(t, func() {
		mustRun(t, "capabilities", "--json")
		mustRun(t, "conformance", "--json")
	})
	assertContains(t, output, `"protocol_version":"fabric/1.0"`)
	assertContains(t, output, `"output_formats":["human","json"]`)
	assertContains(t, output, `"valid":1`)
}
