package core

import "github.com/lutefd/fabric/protocol"

const (
	DurabilityLive      = "live"
	DurabilityCandidate = "candidate"
	DurabilityDurable   = "durable"

	StatusActive     = "active"
	StatusExpired    = "expired"
	StatusDiscarded  = "discarded"
	StatusSuperseded = "superseded"
)

type DirectionEvent struct {
	ID              string                            `json:"id"`
	Kind            string                            `json:"kind"`
	CreatedAt       string                            `json:"created_at"`
	Scope           EventScope                        `json:"scope"`
	Source          EventSource                       `json:"source"`
	Text            string                            `json:"text"`
	Confidence      string                            `json:"confidence"`
	TTL             string                            `json:"ttl"`
	Challenges      string                            `json:"challenges,omitempty"`
	Status          string                            `json:"status,omitempty"`
	Durability      string                            `json:"durability,omitempty"`
	ReviewType      string                            `json:"review_type,omitempty"`
	Reason          string                            `json:"reason,omitempty"`
	RejectedPaths   []string                          `json:"rejected_paths,omitempty"`
	PreferredPaths  []string                          `json:"preferred_paths,omitempty"`
	Evidence        []EvidenceRef                     `json:"evidence,omitempty"`
	LifecycleReason string                            `json:"lifecycle_reason,omitempty"`
	ReviewedAt      string                            `json:"reviewed_at,omitempty"`
	Actor           protocol.ActorRef                 `json:"actor,omitempty"`
	Trust           protocol.TrustClaim               `json:"trust,omitempty"`
	HeadActor       protocol.ActorRef                 `json:"head_actor,omitempty"`
	HeadTrust       protocol.TrustClaim               `json:"head_trust,omitempty"`
	Conflict        *protocol.MaterializationConflict `json:"conflict,omitempty"`
	HeadEventID     string                            `json:"-"`
}

type EventScope struct {
	Repo   string   `json:"repo,omitempty"`
	Issue  string   `json:"issue,omitempty"`
	PR     string   `json:"pr,omitempty"`
	Areas  []string `json:"areas,omitempty"`
	Paths  []string `json:"paths,omitempty"`
	Global bool     `json:"global,omitempty"`
}

type EventSource struct {
	Type     string `json:"type"`
	ThreadID string `json:"thread_id,omitempty"`
	PR       string `json:"pr,omitempty"`
	URL      string `json:"url,omitempty"`
}

type EvidenceRef struct {
	Type   string `json:"type,omitempty"`
	URL    string `json:"url,omitempty"`
	Author string `json:"author,omitempty"`
	Text   string `json:"text,omitempty"`
}

type ThreadRecord struct {
	ThreadID  string   `json:"thread_id"`
	CreatedAt string   `json:"created_at"`
	Issue     string   `json:"issue,omitempty"`
	PR        string   `json:"pr,omitempty"`
	Areas     []string `json:"areas,omitempty"`
	Paths     []string `json:"paths,omitempty"`
	UpdatedAt string   `json:"updated_at,omitempty"`
}

type matchReason struct {
	Issue  bool
	PR     bool
	Areas  []string
	Paths  []string
	Global bool
}
