package protocol

import "context"

// EventStore persists immutable repository protocol events.
type EventStore interface {
	Put(context.Context, EventEnvelope) error
	List(context.Context) ([]EventEnvelope, error)
}

// RuntimeStore persists thread, projection, and receipt events for active clients.
// A future encrypted service transport can implement this interface without
// changing protocol semantics.
type RuntimeStore interface {
	PutRuntime(context.Context, EventEnvelope) error
	ListRuntime(context.Context, ...string) ([]EventEnvelope, error)
}
