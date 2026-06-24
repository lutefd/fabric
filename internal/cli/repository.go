package cli

import (
	"path/filepath"

	"github.com/lutefd/fabric/internal/direction"
	"github.com/lutefd/fabric/internal/store"
	"github.com/lutefd/fabric/protocol"
)

func directionRepository() (direction.Repository, error) {
	sharedDir, err := sharedEventDir()
	if err != nil {
		return direction.Repository{}, err
	}
	return direction.Repository{Ledger: store.Ledger{
		DurableDir: ledgerEventsPath,
		ActiveDir:  activeEventsPath,
		SharedDir:  sharedDir,
	}}, nil
}

func sharedEventDir() (string, error) {
	common, err := gitCommonDir()
	if err != nil || common == "" {
		return "", err
	}
	return filepath.Join(common, sharedEventsRel), nil
}

func appendDirection(event *DirectionEvent) error {
	repository, err := directionRepository()
	if err != nil {
		return err
	}
	return repository.Create(event)
}

func appendDirectionState(before, after DirectionEvent, reason string) (DirectionEvent, error) {
	repository, err := directionRepository()
	if err != nil {
		return DirectionEvent{}, err
	}
	threadID, _ := loadCurrentThreadID()
	actor := protocol.ActorRef{Kind: "tool", ID: "fabric-cli"}
	if threadID != "" {
		actor.ID += ":" + threadID
	}
	trust := protocol.TrustClaim{Level: "tool_verified", Basis: "local lifecycle command"}
	return repository.Change(before, after, reason, actor, trust)
}

func loadDirectionsUnlocked() ([]DirectionEvent, []string, error) {
	repository, err := directionRepository()
	if err != nil {
		return nil, nil, err
	}
	return repository.Directions()
}

func loadProtocolEventsUnlocked() ([]protocol.EventEnvelope, []string, error) {
	repository, err := directionRepository()
	if err != nil {
		return nil, nil, err
	}
	return repository.Ledger.List()
}
