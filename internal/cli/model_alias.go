package cli

import domain "github.com/lutefd/fabric/internal/core"

const (
	DurabilityLive      = domain.DurabilityLive
	DurabilityCandidate = domain.DurabilityCandidate
	DurabilityDurable   = domain.DurabilityDurable

	StatusActive     = domain.StatusActive
	StatusExpired    = domain.StatusExpired
	StatusDiscarded  = domain.StatusDiscarded
	StatusSuperseded = domain.StatusSuperseded
)

type DirectionEvent = domain.DirectionEvent
type EventScope = domain.EventScope
type EventSource = domain.EventSource
type EvidenceRef = domain.EvidenceRef
type ThreadRecord = domain.ThreadRecord

type matchReason struct {
	Issue  bool
	PR     bool
	Areas  []string
	Paths  []string
	Global bool
}
