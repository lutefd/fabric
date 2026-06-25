package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/lutefd/fabric/internal/core"
	"github.com/lutefd/fabric/protocol"
)

func appendRelation(relation protocol.Relation, durability string, actor protocol.ActorRef, trust protocol.TrustClaim) error {
	repository, err := directionRepository()
	if err != nil {
		return err
	}
	return repository.PutRelation(relation, durability, actor, trust)
}

func runRelation(args []string) error {
	if len(args) == 0 || args[0] != "add" {
		return errors.New(`expected "fabric relation add"`)
	}
	fs := flag.NewFlagSet("relation add", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	relationType := fs.String("type", "", "relation type")
	fromSpec := fs.String("from", "", "source node as kind:id or kind:provider:id")
	toSpec := fs.String("to", "", "target node as kind:id or kind:provider:id")
	fromURL := fs.String("from-url", "", "source deep link")
	toURL := fs.String("to-url", "", "target deep link")
	reason := fs.String("reason", "", "relation rationale")
	actorKind := fs.String("actor-kind", "agent", "asserting actor kind")
	actorID := fs.String("actor-id", "", "asserting actor identifier")
	actorProvider := fs.String("actor-provider", "", "asserting actor provider")
	trustLevel := fs.String("trust-level", "agent_asserted", "assertion trust level")
	trustBasis := fs.String("trust-basis", "relation command", "assertion trust basis")
	candidate := fs.Bool("candidate", false, "persist as candidate provenance")
	durable := fs.Bool("durable", false, "persist as durable provenance")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if !protocol.KnownRelationType(*relationType) {
		return fmt.Errorf("unknown relation type %q", *relationType)
	}
	from, err := parseNodeSpec(*fromSpec)
	if err != nil {
		return fmt.Errorf("--from: %w", err)
	}
	to, err := parseNodeSpec(*toSpec)
	if err != nil {
		return fmt.Errorf("--to: %w", err)
	}
	from.URL, to.URL = *fromURL, *toURL
	durability := DurabilityLive
	if *candidate {
		durability = DurabilityCandidate
	}
	if *durable {
		if *candidate {
			return errors.New("only one of --candidate or --durable may be used")
		}
		if strings.TrimSpace(*reason) == "" {
			return errors.New("durable relation requires --reason")
		}
		durability = DurabilityDurable
	}
	if *relationType == protocol.RelationSupersedes {
		if err := validateSupersedesTrust(from, to); err != nil {
			return err
		}
	}
	id, err := newRelationID()
	if err != nil {
		return err
	}
	relation := protocol.Relation{RelationID: id, Type: *relationType, From: from, To: to, CreatedAt: time.Now().Format(time.RFC3339Nano), Reason: strings.TrimSpace(*reason)}
	if *actorID == "" {
		*actorID, _ = loadCurrentThreadID()
	}
	actor := protocol.ActorRef{Kind: *actorKind, ID: *actorID, Provider: *actorProvider}
	trust := protocol.TrustClaim{Level: *trustLevel, Basis: *trustBasis}
	if err := appendRelation(relation, durability, actor, trust); err != nil {
		return err
	}
	setMachineResult(relation)
	fmt.Printf("Recorded relation %s (%s).\n", relation.RelationID, relation.Type)
	return nil
}

func parseNodeSpec(value string) (protocol.NodeRef, error) {
	parts := strings.SplitN(strings.TrimSpace(value), ":", 3)
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return protocol.NodeRef{}, errors.New("expected kind:id or kind:provider:id")
	}
	if len(parts) == 2 {
		return protocol.NodeRef{Kind: parts[0], ID: parts[1]}, nil
	}
	if parts[2] == "" {
		return protocol.NodeRef{}, errors.New("node ID is required")
	}
	return protocol.NodeRef{Kind: parts[0], Provider: parts[1], ID: parts[2]}, nil
}

func loadRelations() ([]protocol.Relation, error) {
	events, _, err := loadProtocolEventsUnlocked()
	if err != nil {
		return nil, err
	}
	snapshot := core.Materialize(events)
	relations := append([]protocol.Relation(nil), snapshot.Relations...)
	receipts, err := loadReceipts()
	if err != nil {
		return nil, err
	}
	for _, receipt := range receipts {
		relationType := protocol.RelationDeliveredTo
		if receipt.State == protocol.ReceiptExposed {
			relationType = protocol.RelationExposedTo
		}
		projectionNode := protocol.NodeRef{Kind: "projection", ID: receipt.ProjectionID}
		threadNode := protocol.NodeRef{Kind: "thread", ID: receipt.ThreadID}
		for _, recordID := range receipt.RecordIDs {
			relationID, err := protocol.DeriveRelationID(receipt.ReceiptID, relationType+":record:"+recordID)
			if err != nil {
				return nil, err
			}
			relations = append(relations, protocol.Relation{
				RelationID: relationID,
				Type:       relationType,
				From:       protocol.NodeRef{Kind: "record", ID: recordID},
				To:         projectionNode,
				CreatedAt:  receipt.OccurredAt,
			})
		}
		relationID, err := protocol.DeriveRelationID(receipt.ReceiptID, relationType+":thread:"+receipt.ThreadID)
		if err != nil {
			return nil, err
		}
		relations = append(relations, protocol.Relation{
			RelationID: relationID, Type: relationType,
			From: projectionNode, To: threadNode, CreatedAt: receipt.OccurredAt,
		})
	}
	sort.Slice(relations, func(i, j int) bool { return relations[i].RelationID < relations[j].RelationID })
	return relations, nil
}

func validateSupersedesTrust(from, to protocol.NodeRef) error {
	if from.Kind != "record" || to.Kind != "record" {
		return errors.New("supersedes requires record nodes")
	}
	events, err := loadEvents()
	if err != nil {
		return err
	}
	byID := map[string]DirectionEvent{}
	for _, event := range events {
		byID[event.ID] = event
	}
	fromRecord, fromOK := byID[from.ID]
	toRecord, toOK := byID[to.ID]
	if !fromOK || !toOK {
		return errors.New("supersedes references an unknown record")
	}
	if trustRank(fromRecord.Trust.Level) < trustRank(toRecord.Trust.Level) {
		return errors.New("lower-trust direction cannot supersede higher-trust direction")
	}
	return nil
}

func trustRank(level string) int {
	switch level {
	case "repository_reviewed":
		return 5
	case "human_confirmed":
		return 4
	case "reviewer_confirmed":
		return 3
	case "tool_verified":
		return 2
	case "agent_asserted":
		return 1
	default:
		return 0
	}
}

func explainGraph(root protocol.NodeRef, direction string, relationTypes []string, depth int) (protocol.Graph, error) {
	relations, err := loadRelations()
	if err != nil {
		return protocol.Graph{}, err
	}
	allowed := map[string]bool{}
	for _, relationType := range relationTypes {
		allowed[relationType] = true
	}
	if direction == "" {
		direction = "both"
	}
	graph := core.Traverse(root, relations, direction, allowed, depth)
	if err := enrichRelationDetails(&graph); err != nil {
		return protocol.Graph{}, err
	}
	if err := enrichGraph(&graph); err != nil {
		return protocol.Graph{}, err
	}
	return graph, nil
}

func enrichRelationDetails(graph *protocol.Graph) error {
	details := map[string]protocol.RelationDetail{}
	events, _, err := loadProtocolEventsUnlocked()
	if err != nil {
		return err
	}
	for _, event := range events {
		if event.EventType != protocol.EventRelationCreated {
			continue
		}
		var payload protocol.RelationCreated
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return err
		}
		details[payload.Relation.RelationID] = protocol.RelationDetail{
			RelationID: payload.Relation.RelationID, EventID: event.EventID,
			Actor: event.Actor, Trust: event.Trust,
		}
	}
	receiptEvents, err := loadRuntimeEvents(runtimeReceipts)
	if err != nil {
		return err
	}
	for _, event := range receiptEvents {
		var payload protocol.ReceiptRecorded
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return err
		}
		receipt := payload.Receipt
		relationType := protocol.RelationDeliveredTo
		if receipt.State == protocol.ReceiptExposed {
			relationType = protocol.RelationExposedTo
		}
		for _, recordID := range receipt.RecordIDs {
			relationID, err := protocol.DeriveRelationID(receipt.ReceiptID, relationType+":record:"+recordID)
			if err != nil {
				return err
			}
			details[relationID] = protocol.RelationDetail{RelationID: relationID, EventID: event.EventID, Actor: event.Actor, Trust: event.Trust}
		}
		relationID, err := protocol.DeriveRelationID(receipt.ReceiptID, relationType+":thread:"+receipt.ThreadID)
		if err != nil {
			return err
		}
		details[relationID] = protocol.RelationDetail{RelationID: relationID, EventID: event.EventID, Actor: event.Actor, Trust: event.Trust}
	}
	for _, relation := range graph.Relations {
		if detail, ok := details[relation.RelationID]; ok {
			graph.RelationDetails = append(graph.RelationDetails, detail)
		}
	}
	sort.Slice(graph.RelationDetails, func(i, j int) bool { return graph.RelationDetails[i].RelationID < graph.RelationDetails[j].RelationID })
	return nil
}

func enrichGraph(graph *protocol.Graph) error {
	events, _, err := loadProtocolEventsUnlocked()
	if err != nil {
		return err
	}
	snapshot := core.Materialize(events)

	projections := map[string]protocol.Projection{}
	projectionEvents, err := loadRuntimeEvents(runtimeProjections)
	if err != nil {
		return err
	}
	for _, event := range projectionEvents {
		var payload protocol.ProjectionCreated
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return err
		}
		projections[payload.Projection.ProjectionID] = payload.Projection
	}
	threads, err := loadRuntimeThreads()
	if err != nil {
		return err
	}

	for _, node := range graph.Nodes {
		detail := protocol.NodeDetail{Ref: node}
		switch node.Kind {
		case "record":
			if materialized, ok := snapshot.Records[node.ID]; ok {
				detail.Record = &protocol.RecordNodeDetail{
					Record: materialized.Record, HeadEventID: materialized.HeadEventID,
					Actor: materialized.Actor, Trust: materialized.Trust,
					HeadActor: materialized.HeadActor, HeadTrust: materialized.HeadTrust,
					Conflict: materialized.Conflict,
				}
			}
		case "projection":
			if projection, ok := projections[node.ID]; ok {
				detail.Projection = &projection
			}
		case "thread":
			if thread, ok := threads[node.ID]; ok {
				resolved := protocol.Thread{
					ThreadID: thread.ThreadID, CreatedAt: thread.CreatedAt, UpdatedAt: thread.UpdatedAt,
					Scope: protocol.Scope{Repo: repoName(), Issue: thread.Issue, PR: thread.PR, Areas: thread.Areas, Paths: thread.Paths},
				}
				detail.Thread = &resolved
			}
		}
		if detail.Record != nil || detail.Projection != nil || detail.Thread != nil {
			graph.NodeDetails = append(graph.NodeDetails, detail)
		}
	}
	sort.Slice(graph.NodeDetails, func(i, j int) bool { return graph.NodeDetails[i].Ref.Key() < graph.NodeDetails[j].Ref.Key() })
	return nil
}

func printGraph(graph protocol.Graph) {
	fmt.Printf("Explanation graph for %s\n\n", graph.Root.Key())
	if len(graph.Relations) == 0 {
		fmt.Println("No provenance relations found.")
	}
	for _, relation := range graph.Relations {
		class := "causal"
		if relation.Type == protocol.RelationDeliveredTo || relation.Type == protocol.RelationExposedTo {
			class = "availability, not proof of influence"
		}
		fmt.Printf("- %s --%s--> %s [%s]\n", relation.From.Key(), relation.Type, relation.To.Key(), class)
		if relation.Reason != "" {
			fmt.Printf("  reason: %s\n", relation.Reason)
		}
		for _, detail := range graph.RelationDetails {
			if detail.RelationID == relation.RelationID {
				fmt.Printf("  asserted by: %s (%s)\n", detail.Actor.Kind, detail.Trust.Level)
				break
			}
		}
	}
	for _, detail := range graph.NodeDetails {
		if detail.Record == nil {
			continue
		}
		record := detail.Record
		fmt.Printf("\nRecord %s\n", record.Record.RecordID)
		fmt.Printf("- text: %s\n", record.Record.Text)
		if record.Record.Reason != "" {
			fmt.Printf("- rationale: %s\n", record.Record.Reason)
		}
		fmt.Printf("- status: %s\n", record.Record.Status)
		fmt.Printf("- creation trust: %s\n", record.Trust.Level)
		if record.HeadEventID != "" && record.HeadActor.Kind != "" {
			fmt.Printf("- latest revision: %s by %s (%s)\n", record.HeadEventID, record.HeadActor.Kind, record.HeadTrust.Level)
		}
		if record.Conflict != nil {
			fmt.Printf("- conflict: %s\n", record.Conflict.Message)
		}
	}
}

func marshalGraph(graph protocol.Graph) json.RawMessage {
	raw, _ := json.Marshal(graph)
	return raw
}
