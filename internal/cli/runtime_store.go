package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"time"

	"github.com/lutefd/fabric/internal/store"
	"github.com/lutefd/fabric/protocol"
)

const (
	runtimeThreads     = "threads"
	runtimeProjections = "projections"
	runtimeReceipts    = "receipts"
)

func appendRuntimeEnvelope(kind string, event protocol.EventEnvelope) error {
	if err := event.Validate(); err != nil {
		return err
	}
	runtimeStore, err := runtimeFileStore()
	if err != nil {
		return err
	}
	expected := runtimeKindForEvent(event.EventType)
	if kind != expected {
		return fmt.Errorf("runtime kind %q does not match event type %q", kind, event.EventType)
	}
	return runtimeStore.PutRuntime(context.Background(), event)
}

func sharedRuntimePath(kind string) (string, error) {
	root, err := sharedRuntimeRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, kind), nil
}

func sharedRuntimeRoot() (string, error) {
	common, err := gitCommonDir()
	if err != nil {
		return "", err
	}
	if common == "" {
		return filepath.Join(".fabric", "active", "runtime"), nil
	}
	return filepath.Join(common, sharedRuntimeRel), nil
}

func runtimeFileStore() (*store.ImmutableRuntimeStore, error) {
	root, err := sharedRuntimeRoot()
	if err != nil {
		return nil, err
	}
	return &store.ImmutableRuntimeStore{RootDir: root}, nil
}

func loadRuntimeEvents(kind string) ([]protocol.EventEnvelope, error) {
	runtimeStore, err := runtimeFileStore()
	if err != nil {
		return nil, err
	}
	return runtimeStore.ListRuntime(context.Background(), kind)
}

func runtimeKindForEvent(eventType string) string {
	switch eventType {
	case protocol.EventThreadStarted, protocol.EventThreadScopeChanged:
		return runtimeThreads
	case protocol.EventProjectionCreated:
		return runtimeProjections
	case protocol.EventReceiptRecorded:
		return runtimeReceipts
	case protocol.EventRelationCreated:
		return store.RuntimeRelations
	default:
		return ""
	}
}

func saveRuntimeThread(record ThreadRecord, eventType string) error {
	if record.CreatedAt == "" {
		record.CreatedAt = nowString()
	}
	if record.UpdatedAt == "" {
		record.UpdatedAt = nowString()
	}
	payload := protocol.ThreadEvent{Thread: protocol.Thread{
		ThreadID:  record.ThreadID,
		CreatedAt: record.CreatedAt,
		UpdatedAt: record.UpdatedAt,
		Scope: protocol.Scope{
			Repo: repoName(), Issue: record.Issue, PR: record.PR,
			Areas: record.Areas, Paths: record.Paths,
		},
	}}
	envelope, err := protocol.NewEnvelope(eventType,
		protocol.ActorRef{Kind: "agent", ID: record.ThreadID},
		protocol.TrustClaim{Level: "agent_asserted", Basis: "local thread runtime"}, payload)
	if err != nil {
		return err
	}
	return appendRuntimeEnvelope(runtimeThreads, envelope)
}

func loadRuntimeThreads() (map[string]ThreadRecord, error) {
	events, err := loadRuntimeEvents(runtimeThreads)
	if err != nil {
		return nil, err
	}
	threads := map[string]ThreadRecord{}
	for _, event := range events {
		var payload protocol.ThreadEvent
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return nil, err
		}
		thread := payload.Thread
		existing, ok := threads[thread.ThreadID]
		if ok && existing.UpdatedAt > thread.UpdatedAt {
			continue
		}
		threads[thread.ThreadID] = ThreadRecord{
			ThreadID: thread.ThreadID, CreatedAt: thread.CreatedAt, UpdatedAt: thread.UpdatedAt,
			Issue: thread.Scope.Issue, PR: thread.Scope.PR, Areas: thread.Scope.Areas, Paths: thread.Scope.Paths,
		}
	}
	return threads, nil
}

func createProjection(purpose, threadID string, scope protocol.Scope, events []DirectionEvent, omitted bool) (protocol.Projection, error) {
	id, err := protocol.NewProjectionID()
	if err != nil {
		return protocol.Projection{}, err
	}
	projection := protocol.Projection{
		ProjectionID: id, ThreadID: threadID, Purpose: purpose,
		CreatedAt: time.Now().Format(time.RFC3339Nano), Scope: scope,
		Reasons: map[string][]protocol.MatchReason{}, Omitted: omitted,
	}
	for _, event := range events {
		projection.EventIDs = append(projection.EventIDs, directionRevisionIDs(event)...)
		projection.RecordIDs = append(projection.RecordIDs, event.ID)
		projection.Reasons[event.ID] = protocolReasons(event, scope)
		if event.Conflict != nil {
			projection.Conflicts = append(projection.Conflicts, *event.Conflict)
		}
	}
	payload := protocol.ProjectionCreated{Projection: projection}
	envelope, err := protocol.NewEnvelope(protocol.EventProjectionCreated,
		protocol.ActorRef{Kind: "tool", ID: "fabric"},
		protocol.TrustClaim{Level: "tool_verified", Basis: "deterministic projection"}, payload)
	if err != nil {
		return protocol.Projection{}, err
	}
	if err := appendRuntimeEnvelope(runtimeProjections, envelope); err != nil {
		return protocol.Projection{}, err
	}
	return projection, nil
}

func recordProjectionReceipt(projection protocol.Projection, state, provider string) (protocol.Receipt, error) {
	id, err := protocol.NewReceiptID()
	if err != nil {
		return protocol.Receipt{}, err
	}
	receipt := protocol.Receipt{
		ReceiptID: id, ProjectionID: projection.ProjectionID, ThreadID: projection.ThreadID,
		State: state, OccurredAt: time.Now().Format(time.RFC3339Nano),
		EventIDs: projection.EventIDs, RecordIDs: projection.RecordIDs, Provider: provider,
	}
	payload := protocol.ReceiptRecorded{Receipt: receipt}
	envelope, err := protocol.NewEnvelope(protocol.EventReceiptRecorded,
		protocol.ActorRef{Kind: "agent", ID: projection.ThreadID, Provider: provider},
		protocol.TrustClaim{Level: "agent_asserted", Basis: state + " acknowledgement"}, payload)
	if err != nil {
		return protocol.Receipt{}, err
	}
	if err := appendRuntimeEnvelope(runtimeReceipts, envelope); err != nil {
		return protocol.Receipt{}, err
	}
	return receipt, nil
}

func loadReceipts() ([]protocol.Receipt, error) {
	events, err := loadRuntimeEvents(runtimeReceipts)
	if err != nil {
		return nil, err
	}
	var receipts []protocol.Receipt
	for _, event := range events {
		var payload protocol.ReceiptRecorded
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return nil, err
		}
		receipts = append(receipts, payload.Receipt)
	}
	return receipts, nil
}

func loadProjection(id string) (protocol.Projection, error) {
	events, err := loadRuntimeEvents(runtimeProjections)
	if err != nil {
		return protocol.Projection{}, err
	}
	for _, event := range events {
		var payload protocol.ProjectionCreated
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return protocol.Projection{}, err
		}
		if payload.Projection.ProjectionID == id {
			return payload.Projection, nil
		}
	}
	return protocol.Projection{}, fmt.Errorf("projection %s not found", id)
}

func deliveredForThread(threadID string) (map[string]bool, map[string]bool, error) {
	receipts, err := loadReceipts()
	if err != nil {
		return nil, nil, err
	}
	events := map[string]bool{}
	records := map[string]bool{}
	for _, receipt := range receipts {
		if receipt.ThreadID != threadID {
			continue
		}
		for _, id := range receipt.EventIDs {
			events[id] = true
		}
		for _, id := range receipt.RecordIDs {
			records[id] = true
		}
	}
	return events, records, nil
}

func relevantUndelivered(events []DirectionEvent, thread ThreadRecord) ([]DirectionEvent, error) {
	delivered, deliveredRecords, err := deliveredForThread(thread.ThreadID)
	if err != nil {
		return nil, err
	}
	var matches []DirectionEvent
	for _, event := range events {
		if allRevisionsDelivered(event, delivered) {
			continue
		}
		if !reasonForScope(event, thread.Issue, thread.PR, thread.Areas, thread.Paths).matched() {
			continue
		}
		if isActiveEvent(event) || deliveredRecords[event.ID] {
			matches = append(matches, event)
		}
	}
	return matches, nil
}

func seenAndStaleFromReceipts(event DirectionEvent, threads map[string]ThreadRecord) ([]string, []string, error) {
	var seen, stale []string
	if !isActiveEvent(event) {
		return seen, stale, nil
	}
	for id, thread := range threads {
		if !reasonForScope(event, thread.Issue, thread.PR, thread.Areas, thread.Paths).matched() {
			continue
		}
		delivered, _, err := deliveredForThread(id)
		if err != nil {
			return nil, nil, err
		}
		if allRevisionsDelivered(event, delivered) {
			seen = append(seen, id)
		} else {
			stale = append(stale, id)
		}
	}
	sort.Strings(seen)
	sort.Strings(stale)
	return seen, stale, nil
}

func directionRevisionIDs(event DirectionEvent) []string {
	seen := map[string]bool{}
	var ids []string
	for _, id := range append([]string{event.HeadEventID}, conflictEventIDs(event)...) {
		if id != "" && !seen[id] {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	return ids
}

func conflictEventIDs(event DirectionEvent) []string {
	if event.Conflict == nil {
		return nil
	}
	return event.Conflict.CompetingEventIDs
}

func allRevisionsDelivered(event DirectionEvent, delivered map[string]bool) bool {
	ids := directionRevisionIDs(event)
	if len(ids) == 0 {
		return false
	}
	for _, id := range ids {
		if !delivered[id] {
			return false
		}
	}
	return true
}

func protocolReasons(event DirectionEvent, scope protocol.Scope) []protocol.MatchReason {
	reason := reasonForScope(event, scope.Issue, scope.PR, scope.Areas, scope.Paths)
	var result []protocol.MatchReason
	if event.Conflict != nil {
		result = append(result, protocol.MatchReason{Kind: "conflict", Value: event.Conflict.ParentEventID})
	}
	if reason.Global {
		result = append(result, protocol.MatchReason{Kind: "global"})
	}
	if reason.PR {
		result = append(result, protocol.MatchReason{Kind: "pr", Value: event.Scope.PR})
	}
	if reason.Issue {
		result = append(result, protocol.MatchReason{Kind: "issue", Value: event.Scope.Issue})
	}
	for _, path := range reason.Paths {
		result = append(result, protocol.MatchReason{Kind: "path", Value: path})
	}
	for _, area := range reason.Areas {
		result = append(result, protocol.MatchReason{Kind: "area", Value: area})
	}
	return result
}
