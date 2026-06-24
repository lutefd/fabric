package store

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/lutefd/fabric/protocol"
)

const (
	RuntimeThreads     = "threads"
	RuntimeProjections = "projections"
	RuntimeReceipts    = "receipts"
	RuntimeRelations   = "relations"
)

type ImmutableRuntimeStore struct {
	RootDir string
}

var _ protocol.RuntimeStore = (*ImmutableRuntimeStore)(nil)

func (s *ImmutableRuntimeStore) PutRuntime(_ context.Context, event protocol.EventEnvelope) error {
	kind, err := runtimeKind(event.EventType)
	if err != nil {
		return err
	}
	return WriteImmutable(filepath.Join(s.RootDir, kind), event)
}

func (s *ImmutableRuntimeStore) ListRuntime(_ context.Context, kinds ...string) ([]protocol.EventEnvelope, error) {
	if len(kinds) == 0 {
		kinds = []string{RuntimeThreads, RuntimeProjections, RuntimeReceipts, RuntimeRelations}
	}
	dirs := make([]string, 0, len(kinds))
	for _, kind := range kinds {
		switch kind {
		case RuntimeThreads, RuntimeProjections, RuntimeReceipts, RuntimeRelations:
			dirs = append(dirs, filepath.Join(s.RootDir, kind))
		default:
			return nil, fmt.Errorf("unknown runtime event kind %q", kind)
		}
	}
	events, conflicts, err := Load(dirs...)
	if err != nil {
		return nil, err
	}
	if len(conflicts) > 0 {
		return nil, fmt.Errorf("runtime immutable conflicts: %v", conflicts)
	}
	return events, nil
}

func runtimeKind(eventType string) (string, error) {
	switch eventType {
	case protocol.EventThreadStarted, protocol.EventThreadScopeChanged:
		return RuntimeThreads, nil
	case protocol.EventProjectionCreated:
		return RuntimeProjections, nil
	case protocol.EventReceiptRecorded:
		return RuntimeReceipts, nil
	case protocol.EventRelationCreated:
		return RuntimeRelations, nil
	default:
		return "", fmt.Errorf("event type %q is not runtime state", eventType)
	}
}
