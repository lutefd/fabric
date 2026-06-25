package core

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/lutefd/fabric/protocol"
)

type MaterializedRecord struct {
	Record      protocol.Record
	HeadEventID string
	Actor       protocol.ActorRef
	Trust       protocol.TrustClaim
	HeadActor   protocol.ActorRef
	HeadTrust   protocol.TrustClaim
	Conflict    *protocol.MaterializationConflict
}

type Snapshot struct {
	Records   map[string]MaterializedRecord
	Relations []protocol.Relation
	Conflicts []string
}

type materializedStateChange struct {
	Event  protocol.EventEnvelope
	Change protocol.RecordStateChanged
}

func Materialize(events []protocol.EventEnvelope) Snapshot {
	snapshot := Snapshot{Records: map[string]MaterializedRecord{}}
	children := map[string][]materializedStateChange{}
	for _, event := range events {
		switch event.EventType {
		case protocol.EventRecordCreated:
			var payload protocol.RecordCreated
			if err := json.Unmarshal(event.Payload, &payload); err != nil {
				snapshot.Conflicts = append(snapshot.Conflicts, fmt.Sprintf("event %s payload: %v", event.EventID, err))
				continue
			}
			id := payload.Record.RecordID
			if existing, ok := snapshot.Records[id]; ok && existing.HeadEventID != event.EventID {
				snapshot.Conflicts = append(snapshot.Conflicts, fmt.Sprintf("record %s has multiple creation events", id))
				continue
			}
			snapshot.Records[id] = MaterializedRecord{
				Record: payload.Record, HeadEventID: event.EventID,
				Actor: event.Actor, Trust: event.Trust, HeadActor: event.Actor, HeadTrust: event.Trust,
			}
		case protocol.EventRecordStateChanged:
			var payload protocol.RecordStateChanged
			if json.Unmarshal(event.Payload, &payload) == nil {
				children[event.ParentEventID] = append(children[event.ParentEventID], materializedStateChange{Event: event, Change: payload})
			}
		case protocol.EventRelationCreated:
			var payload protocol.RelationCreated
			if err := json.Unmarshal(event.Payload, &payload); err != nil {
				snapshot.Conflicts = append(snapshot.Conflicts, fmt.Sprintf("event %s payload: %v", event.EventID, err))
				continue
			}
			snapshot.Relations = append(snapshot.Relations, payload.Relation)
		}
	}

	for recordID, current := range snapshot.Records {
		for {
			var applicable []materializedStateChange
			for _, candidate := range children[current.HeadEventID] {
				if candidate.Change.RecordID == recordID {
					applicable = append(applicable, candidate)
				}
			}
			if len(applicable) == 0 {
				break
			}
			if len(applicable) > 1 {
				ids := make([]string, 0, len(applicable))
				for _, candidate := range applicable {
					ids = append(ids, candidate.Event.EventID)
				}
				sort.Strings(ids)
				message := fmt.Sprintf("record %s has %d competing children of %s", recordID, len(applicable), current.HeadEventID)
				current.Conflict = &protocol.MaterializationConflict{
					RecordID: recordID, ParentEventID: current.HeadEventID,
					CompetingEventIDs: ids, Message: message,
				}
				snapshot.Conflicts = append(snapshot.Conflicts, message)
				break
			}
			change := applicable[0].Change
			if change.Status != "" {
				current.Record.Status = change.Status
			}
			if change.Durability != "" {
				current.Record.Durability = change.Durability
			}
			if change.LifecycleReason != "" {
				current.Record.LifecycleReason = change.LifecycleReason
			}
			if change.ReviewedAt != "" {
				current.Record.ReviewedAt = change.ReviewedAt
			}
			current.HeadEventID = applicable[0].Event.EventID
			current.HeadActor = applicable[0].Event.Actor
			current.HeadTrust = applicable[0].Event.Trust
		}
		snapshot.Records[recordID] = current
	}
	sort.Slice(snapshot.Relations, func(i, j int) bool {
		return snapshot.Relations[i].RelationID < snapshot.Relations[j].RelationID
	})
	sort.Strings(snapshot.Conflicts)
	return snapshot
}
