package protocol

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"
)

func DecodeEvent(raw []byte) (EventEnvelope, error) {
	var event EventEnvelope
	if err := decodeStrict(raw, &event); err != nil {
		return EventEnvelope{}, err
	}
	return event, event.Validate()
}

func decodeStrict(raw []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return errors.New("multiple JSON values are not allowed")
		}
		return err
	}
	return nil
}

func NewEnvelope(eventType string, actor ActorRef, trust TrustClaim, payload any) (EventEnvelope, error) {
	eventID, err := NewEventID()
	if err != nil {
		return EventEnvelope{}, err
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return EventEnvelope{}, err
	}
	event := EventEnvelope{
		SchemaVersion: SchemaVersion,
		EventID:       eventID,
		EventType:     eventType,
		OccurredAt:    time.Now().Format(time.RFC3339Nano),
		Actor:         actor,
		Trust:         trust,
		Payload:       raw,
	}
	return event, nil
}

func (e EventEnvelope) Validate() error {
	if e.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported schema_version %q", e.SchemaVersion)
	}
	if !ValidTypedID(e.EventID, "evt") {
		return errors.New("event_id must be an evt_ UUIDv7")
	}
	if !KnownEventType(e.EventType) {
		return fmt.Errorf("unsupported event_type %q", e.EventType)
	}
	if _, err := time.Parse(time.RFC3339Nano, e.OccurredAt); err != nil {
		return fmt.Errorf("occurred_at: %w", err)
	}
	if !KnownActorKind(e.Actor.Kind) {
		return errors.New("actor.kind must be human, reviewer, agent, or tool")
	}
	if e.Trust.Level == "" {
		return errors.New("trust.level is required")
	}
	if len(e.Payload) == 0 || !json.Valid(e.Payload) {
		return errors.New("payload must be valid JSON")
	}
	if e.ParentEventID != "" && !ValidTypedID(e.ParentEventID, "evt") {
		return errors.New("parent_event_id must be an evt_ UUIDv7")
	}
	return validatePayload(e)
}

func KnownEventType(value string) bool {
	switch value {
	case EventRecordCreated, EventRecordStateChanged, EventRelationCreated,
		EventThreadStarted, EventThreadScopeChanged, EventProjectionCreated,
		EventReceiptRecorded:
		return true
	default:
		return false
	}
}

func KnownRelationType(value string) bool {
	switch value {
	case RelationDerivedFrom, RelationInformedBy, RelationImplements,
		RelationSupersedes, RelationChallenges, RelationResolves,
		RelationDeliveredTo, RelationExposedTo:
		return true
	default:
		return false
	}
}

func KnownActorKind(value string) bool {
	switch value {
	case "human", "reviewer", "agent", "tool":
		return true
	default:
		return false
	}
}

func KnownStatus(value string) bool {
	switch value {
	case "active", "expired", "discarded", "superseded", "open", "accepted", "rejected":
		return true
	default:
		return false
	}
}

func KnownDurability(value string) bool {
	return value == "live" || value == "candidate" || value == "durable"
}

func validatePayload(event EventEnvelope) error {
	switch event.EventType {
	case EventRecordCreated:
		var payload RecordCreated
		if err := decodeStrict(event.Payload, &payload); err != nil {
			return err
		}
		record := payload.Record
		if !ValidTypedID(record.RecordID, "rec") || record.Kind == "" || record.Text == "" ||
			record.Source.Type == "" || record.Confidence == "" || record.TTL == "" ||
			record.Status == "" || record.Durability == "" {
			return errors.New("record.created requires a typed record_id and complete record fields")
		}
		if _, err := time.Parse(time.RFC3339Nano, record.CreatedAt); err != nil {
			return fmt.Errorf("record.created_at: %w", err)
		}
		if err := validateScope(record.Scope); err != nil {
			return fmt.Errorf("record scope: %w", err)
		}
		if !KnownStatus(record.Status) {
			return errors.New("record status is not recognized")
		}
		if !KnownDurability(record.Durability) {
			return errors.New("record durability must be live, candidate, or durable")
		}
	case EventRecordStateChanged:
		var payload RecordStateChanged
		if err := decodeStrict(event.Payload, &payload); err != nil {
			return err
		}
		if !ValidTypedID(payload.RecordID, "rec") || event.ParentEventID == "" {
			return errors.New("record.state_changed requires a typed record_id and parent_event_id")
		}
		if payload.Status == "" && payload.Durability == "" && payload.LifecycleReason == "" && payload.ReviewedAt == "" {
			return errors.New("record.state_changed requires at least one changed field")
		}
		if payload.Status != "" && !KnownStatus(payload.Status) {
			return errors.New("record state change status is not recognized")
		}
		if payload.Durability != "" && !KnownDurability(payload.Durability) {
			return errors.New("record state change durability is not recognized")
		}
		if payload.ReviewedAt != "" {
			if _, err := time.Parse(time.RFC3339Nano, payload.ReviewedAt); err != nil {
				return fmt.Errorf("record reviewed_at: %w", err)
			}
		}
	case EventRelationCreated:
		var payload RelationCreated
		if err := decodeStrict(event.Payload, &payload); err != nil {
			return err
		}
		relation := payload.Relation
		if !ValidTypedID(relation.RelationID, "rel") || !KnownRelationType(relation.Type) {
			return errors.New("relation.created requires a typed relation_id and known type")
		}
		if relation.From.Kind == "" || relation.From.ID == "" || relation.To.Kind == "" || relation.To.ID == "" {
			return errors.New("relation endpoints require IDs")
		}
		if _, err := time.Parse(time.RFC3339Nano, relation.CreatedAt); err != nil {
			return fmt.Errorf("relation.created_at: %w", err)
		}
	case EventThreadStarted, EventThreadScopeChanged:
		var payload ThreadEvent
		if err := decodeStrict(event.Payload, &payload); err != nil {
			return err
		}
		if payload.Thread.ThreadID == "" {
			return errors.New("thread event requires thread_id")
		}
		if err := validateScope(payload.Thread.Scope); err != nil {
			return fmt.Errorf("thread scope: %w", err)
		}
		if _, err := time.Parse(time.RFC3339Nano, payload.Thread.CreatedAt); err != nil {
			return fmt.Errorf("thread.created_at: %w", err)
		}
		if _, err := time.Parse(time.RFC3339Nano, payload.Thread.UpdatedAt); err != nil {
			return fmt.Errorf("thread.updated_at: %w", err)
		}
	case EventProjectionCreated:
		var payload ProjectionCreated
		if err := decodeStrict(event.Payload, &payload); err != nil {
			return err
		}
		projection := payload.Projection
		if !ValidTypedID(projection.ProjectionID, "prj") || projection.Purpose == "" {
			return errors.New("projection.created requires a typed projection_id and purpose")
		}
		if err := validateScope(projection.Scope); err != nil {
			return fmt.Errorf("projection scope: %w", err)
		}
		if _, err := time.Parse(time.RFC3339Nano, projection.CreatedAt); err != nil {
			return fmt.Errorf("projection.created_at: %w", err)
		}
		if !uniqueStrings(projection.EventIDs) || !uniqueStrings(projection.RecordIDs) {
			return errors.New("projection event_ids and record_ids must be unique sets")
		}
		for _, eventID := range projection.EventIDs {
			if !ValidTypedID(eventID, "evt") {
				return errors.New("projection members require typed event IDs")
			}
		}
		for _, recordID := range projection.RecordIDs {
			if !ValidTypedID(recordID, "rec") {
				return errors.New("projection members require typed record IDs")
			}
		}
		for _, conflict := range projection.Conflicts {
			if !ValidTypedID(conflict.RecordID, "rec") || !ValidTypedID(conflict.ParentEventID, "evt") || len(conflict.CompetingEventIDs) < 2 || !uniqueStrings(conflict.CompetingEventIDs) || conflict.Message == "" {
				return errors.New("projection conflict is incomplete")
			}
			for _, eventID := range conflict.CompetingEventIDs {
				if !ValidTypedID(eventID, "evt") {
					return errors.New("projection conflict requires typed competing event IDs")
				}
			}
		}
	case EventReceiptRecorded:
		var payload ReceiptRecorded
		if err := decodeStrict(event.Payload, &payload); err != nil {
			return err
		}
		receipt := payload.Receipt
		if !ValidTypedID(receipt.ReceiptID, "rcp") || !ValidTypedID(receipt.ProjectionID, "prj") || receipt.ThreadID == "" {
			return errors.New("receipt.recorded requires typed receipt/projection IDs and thread_id")
		}
		if receipt.State != ReceiptDelivered && receipt.State != ReceiptExposed {
			return errors.New("receipt state must be delivered or exposed")
		}
		if _, err := time.Parse(time.RFC3339Nano, receipt.OccurredAt); err != nil {
			return fmt.Errorf("receipt.occurred_at: %w", err)
		}
		if !uniqueStrings(receipt.EventIDs) || !uniqueStrings(receipt.RecordIDs) {
			return errors.New("receipt event_ids and record_ids must be unique sets")
		}
		for _, eventID := range receipt.EventIDs {
			if !ValidTypedID(eventID, "evt") {
				return errors.New("receipt members require typed event IDs")
			}
		}
		for _, recordID := range receipt.RecordIDs {
			if !ValidTypedID(recordID, "rec") {
				return errors.New("receipt members require typed record IDs")
			}
		}
	}
	return nil
}

func usefulScope(scope Scope) bool {
	return scope.Global || scope.Issue != "" || scope.PR != "" || len(scope.Areas) > 0 || len(scope.Paths) > 0
}

func validateScope(scope Scope) error {
	if !usefulScope(scope) {
		return errors.New("issue, pr, area, path, or global is required")
	}
	if !uniqueNonEmptyStrings(scope.Areas) {
		return errors.New("areas must be unique and non-empty")
	}
	if !uniqueNonEmptyStrings(scope.Paths) {
		return errors.New("paths must be unique and non-empty")
	}
	return nil
}

func uniqueStrings(values []string) bool {
	seen := map[string]bool{}
	for _, value := range values {
		if seen[value] {
			return false
		}
		seen[value] = true
	}
	return true
}

func uniqueNonEmptyStrings(values []string) bool {
	for _, value := range values {
		if value == "" {
			return false
		}
	}
	return uniqueStrings(values)
}
