package store

import "github.com/lutefd/fabric/protocol"

// Ledger routes immutable events between the tracked durable ledger and the
// Git-common live mirror shared by sibling worktrees.
type Ledger struct {
	DurableDir string
	ActiveDir  string
	SharedDir  string
}

func (l Ledger) Put(event protocol.EventEnvelope, durable bool) error {
	if err := event.Validate(); err != nil {
		return err
	}
	if l.SharedDir != "" {
		if err := WriteImmutable(l.SharedDir, event); err != nil {
			return err
		}
	}
	if durable {
		return WriteImmutable(l.DurableDir, event)
	}
	if l.SharedDir == "" {
		return WriteImmutable(l.ActiveDir, event)
	}
	return nil
}

func (l Ledger) List() ([]protocol.EventEnvelope, []string, error) {
	return Load(l.DurableDir, l.ActiveDir, l.SharedDir)
}
