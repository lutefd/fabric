package direction

import (
	"github.com/lutefd/fabric/internal/core"
	"github.com/lutefd/fabric/internal/store"
	"github.com/lutefd/fabric/protocol"
)

type Repository struct {
	Ledger store.Ledger
}

func (r Repository) Create(event *core.DirectionEvent) error {
	if err := r.validate(); err != nil {
		return err
	}
	prepared, envelope, relations, err := core.PrepareDirection(*event)
	if err != nil {
		return err
	}
	if err := r.Ledger.Put(envelope, durableLike(prepared.Durability)); err != nil {
		return err
	}
	for _, relation := range relations {
		if err := r.PutRelation(relation, prepared.Durability, prepared.Actor, prepared.Trust); err != nil {
			return err
		}
	}
	*event = prepared
	return nil
}

func (r Repository) Change(before, after core.DirectionEvent, reason string, actor protocol.ActorRef, trust protocol.TrustClaim) (core.DirectionEvent, error) {
	if err := r.validate(); err != nil {
		return core.DirectionEvent{}, err
	}
	updated, envelope, err := core.StateChangeEnvelope(before, after, reason, actor, trust)
	if err != nil {
		return core.DirectionEvent{}, err
	}
	if err := r.Ledger.Put(envelope, durableLike(updated.Durability)); err != nil {
		return core.DirectionEvent{}, err
	}
	return updated, nil
}

func (r Repository) PutRelation(relation protocol.Relation, durability string, actor protocol.ActorRef, trust protocol.TrustClaim) error {
	if err := r.validate(); err != nil {
		return err
	}
	envelope, err := protocol.NewEnvelope(protocol.EventRelationCreated, actor, trust, protocol.RelationCreated{Relation: relation})
	if err != nil {
		return err
	}
	return r.Ledger.Put(envelope, durableLike(durability))
}

func (r Repository) validate() error {
	_, _, err := r.Ledger.List()
	return err
}

func (r Repository) Directions() ([]core.DirectionEvent, []string, error) {
	events, conflicts, err := r.Ledger.List()
	if err != nil {
		return nil, nil, err
	}
	directions, materializationConflicts := core.MaterializeDirections(events)
	return directions, append(conflicts, materializationConflicts...), nil
}

func durableLike(durability string) bool {
	switch core.NormalizeDurability(durability) {
	case core.DurabilityDurable, core.DurabilityCandidate:
		return true
	default:
		return false
	}
}
