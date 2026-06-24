package core

import (
	"encoding/json"
	"sort"
	"time"

	"github.com/lutefd/fabric/protocol"
)

const challengesExtension = "fabric.direction.challenges"

func PrepareDirection(event DirectionEvent) (DirectionEvent, protocol.EventEnvelope, []protocol.Relation, error) {
	recordID, err := protocol.NewRecordID()
	if err != nil {
		return DirectionEvent{}, protocol.EventEnvelope{}, nil, err
	}
	event.ID = recordID
	event.Status = NormalizeStatus(event.Status)
	event.Durability = NormalizeDurability(event.Durability)
	event.Actor, event.Trust = ActorAndTrust(event)
	event.HeadActor, event.HeadTrust = event.Actor, event.Trust
	envelope, err := protocol.NewEnvelope(protocol.EventRecordCreated, event.Actor, event.Trust, protocol.RecordCreated{
		Record: DirectionToRecord(event),
	})
	if err != nil {
		return DirectionEvent{}, protocol.EventEnvelope{}, nil, err
	}
	event.HeadEventID = envelope.EventID
	relations, err := AutomaticRelations(event)
	return event, envelope, relations, err
}

func StateChangeEnvelope(before, after DirectionEvent, reason string, actor protocol.ActorRef, trust protocol.TrustClaim) (DirectionEvent, protocol.EventEnvelope, error) {
	if reason != "" {
		after.LifecycleReason = reason
	}
	payload := protocol.RecordStateChanged{
		RecordID:        before.ID,
		Status:          NormalizeStatus(after.Status),
		Durability:      NormalizeDurability(after.Durability),
		LifecycleReason: after.LifecycleReason,
		ReviewedAt:      after.ReviewedAt,
	}
	envelope, err := protocol.NewEnvelope(protocol.EventRecordStateChanged, actor, trust, payload)
	if err != nil {
		return DirectionEvent{}, protocol.EventEnvelope{}, err
	}
	envelope.ParentEventID = before.HeadEventID
	if err := envelope.Validate(); err != nil {
		return DirectionEvent{}, protocol.EventEnvelope{}, err
	}
	after.HeadEventID = envelope.EventID
	after.HeadActor, after.HeadTrust = actor, trust
	return after, envelope, nil
}

func AutomaticRelations(event DirectionEvent) ([]protocol.Relation, error) {
	var targets []protocol.NodeRef
	if event.Source.URL != "" {
		targets = append(targets, protocol.NodeRef{Kind: "message", ID: event.Source.URL, URL: event.Source.URL})
	}
	if event.Source.ThreadID != "" {
		targets = append(targets, protocol.NodeRef{Kind: "thread", ID: event.Source.ThreadID})
	}
	if event.Source.PR != "" {
		targets = append(targets, protocol.NodeRef{Kind: "pr", ID: event.Source.PR})
	}
	for _, evidence := range event.Evidence {
		if evidence.URL != "" {
			targets = append(targets, protocol.NodeRef{Kind: "evidence", ID: evidence.URL, URL: evidence.URL})
		}
	}

	now := time.Now().Format(time.RFC3339Nano)
	relations := make([]protocol.Relation, 0, len(targets)+1)
	for _, target := range targets {
		id, err := protocol.NewRelationID()
		if err != nil {
			return nil, err
		}
		relations = append(relations, protocol.Relation{
			RelationID: id,
			Type:       protocol.RelationDerivedFrom,
			From:       protocol.NodeRef{Kind: "record", ID: event.ID},
			To:         target,
			CreatedAt:  now,
		})
	}

	relationType := ""
	switch event.Kind {
	case "challenge":
		relationType = protocol.RelationChallenges
	case "challenge_resolution":
		relationType = protocol.RelationResolves
	}
	if relationType != "" && event.Challenges != "" {
		id, err := protocol.NewRelationID()
		if err != nil {
			return nil, err
		}
		relations = append(relations, protocol.Relation{
			RelationID: id,
			Type:       relationType,
			From:       protocol.NodeRef{Kind: "record", ID: event.ID},
			To:         protocol.NodeRef{Kind: "record", ID: event.Challenges},
			CreatedAt:  now,
		})
	}
	return relations, nil
}

func MaterializeDirections(events []protocol.EventEnvelope) ([]DirectionEvent, []string) {
	snapshot := Materialize(events)
	directions := make([]DirectionEvent, 0, len(snapshot.Records))
	for _, record := range snapshot.Records {
		direction := RecordToDirection(record.Record, record.Actor, record.Trust)
		direction.HeadEventID = record.HeadEventID
		direction.HeadActor = record.HeadActor
		direction.HeadTrust = record.HeadTrust
		direction.Conflict = record.Conflict
		directions = append(directions, direction)
	}
	sort.Slice(directions, func(i, j int) bool {
		if directions[i].CreatedAt != directions[j].CreatedAt {
			return directions[i].CreatedAt < directions[j].CreatedAt
		}
		return directions[i].ID < directions[j].ID
	})
	return directions, snapshot.Conflicts
}

func DirectionToRecord(event DirectionEvent) protocol.Record {
	evidence := make([]protocol.EvidenceRef, 0, len(event.Evidence))
	for _, ref := range event.Evidence {
		evidence = append(evidence, protocol.EvidenceRef{Type: ref.Type, URL: ref.URL, Author: ref.Author, Text: ref.Text})
	}
	extensions := map[string]json.RawMessage{}
	if event.Challenges != "" {
		extensions[challengesExtension], _ = json.Marshal(event.Challenges)
	}
	if len(extensions) == 0 {
		extensions = nil
	}
	return protocol.Record{
		RecordID: event.ID, Kind: event.Kind, CreatedAt: event.CreatedAt,
		Scope:  protocol.Scope{Repo: event.Scope.Repo, Issue: event.Scope.Issue, PR: event.Scope.PR, Areas: event.Scope.Areas, Paths: event.Scope.Paths, Global: event.Scope.Global},
		Source: protocol.SourceRef{Type: event.Source.Type, ThreadID: event.Source.ThreadID, PR: event.Source.PR, URL: event.Source.URL},
		Text:   event.Text, Confidence: event.Confidence, TTL: event.TTL,
		Status: NormalizeStatus(event.Status), Durability: NormalizeDurability(event.Durability),
		ReviewType: event.ReviewType, Reason: event.Reason, RejectedPaths: event.RejectedPaths,
		PreferredPaths: event.PreferredPaths, Evidence: evidence, LifecycleReason: event.LifecycleReason,
		ReviewedAt: event.ReviewedAt, Extensions: extensions,
	}
}

func RecordToDirection(record protocol.Record, actor protocol.ActorRef, trust protocol.TrustClaim) DirectionEvent {
	evidence := make([]EvidenceRef, 0, len(record.Evidence))
	for _, ref := range record.Evidence {
		evidence = append(evidence, EvidenceRef{Type: ref.Type, URL: ref.URL, Author: ref.Author, Text: ref.Text})
	}
	challenges := ""
	if raw := record.Extensions[challengesExtension]; len(raw) > 0 {
		_ = json.Unmarshal(raw, &challenges)
	}
	return DirectionEvent{
		ID: record.RecordID, Kind: record.Kind, CreatedAt: record.CreatedAt,
		Scope:  EventScope{Repo: record.Scope.Repo, Issue: record.Scope.Issue, PR: record.Scope.PR, Areas: record.Scope.Areas, Paths: record.Scope.Paths, Global: record.Scope.Global},
		Source: EventSource{Type: record.Source.Type, ThreadID: record.Source.ThreadID, PR: record.Source.PR, URL: record.Source.URL},
		Text:   record.Text, Confidence: record.Confidence, TTL: record.TTL, Challenges: challenges,
		Status: record.Status, Durability: record.Durability, ReviewType: record.ReviewType, Reason: record.Reason,
		RejectedPaths: record.RejectedPaths, PreferredPaths: record.PreferredPaths, Evidence: evidence,
		LifecycleReason: record.LifecycleReason, ReviewedAt: record.ReviewedAt,
		Actor: actor, Trust: trust,
	}
}

func ActorAndTrust(event DirectionEvent) (protocol.ActorRef, protocol.TrustClaim) {
	if event.Actor.Kind != "" && event.Trust.Level != "" {
		return event.Actor, event.Trust
	}
	kind := "agent"
	switch event.Source.Type {
	case "human":
		kind = "human"
	case "review", "pr_ingest":
		kind = "reviewer"
	case "tool":
		kind = "tool"
	}
	level := event.Confidence
	if level == "" {
		level = "agent_asserted"
	}
	return protocol.ActorRef{Kind: kind, ID: event.Source.ThreadID}, protocol.TrustClaim{Level: level, Basis: event.Source.Type}
}

func NormalizeStatus(status string) string {
	if status == "" {
		return StatusActive
	}
	return status
}

func NormalizeDurability(durability string) string {
	if durability == "" {
		return DurabilityDurable
	}
	return durability
}
