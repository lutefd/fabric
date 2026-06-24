package protocol

import "encoding/json"

const (
	SchemaVersion = "fabric/1.0"

	EventRecordCreated      = "record.created"
	EventRecordStateChanged = "record.state_changed"
	EventRelationCreated    = "relation.created"
	EventThreadStarted      = "thread.started"
	EventThreadScopeChanged = "thread.scope_changed"
	EventProjectionCreated  = "projection.created"
	EventReceiptRecorded    = "receipt.recorded"

	RelationDerivedFrom = "derived_from"
	RelationInformedBy  = "informed_by"
	RelationImplements  = "implements"
	RelationSupersedes  = "supersedes"
	RelationChallenges  = "challenges"
	RelationResolves    = "resolves"
	RelationDeliveredTo = "delivered_to"
	RelationExposedTo   = "exposed_to"

	ReceiptDelivered = "delivered"
	ReceiptExposed   = "exposed"
)

// EventEnvelope is the immutable unit exchanged by Fabric stores and transports.
type EventEnvelope struct {
	SchemaVersion string                     `json:"schema_version"`
	EventID       string                     `json:"event_id"`
	EventType     string                     `json:"event_type"`
	OccurredAt    string                     `json:"occurred_at"`
	Actor         ActorRef                   `json:"actor"`
	Trust         TrustClaim                 `json:"trust"`
	ParentEventID string                     `json:"parent_event_id,omitempty"`
	CausationID   string                     `json:"causation_id,omitempty"`
	CorrelationID string                     `json:"correlation_id,omitempty"`
	Payload       json.RawMessage            `json:"payload"`
	Extensions    map[string]json.RawMessage `json:"extensions,omitempty"`
}

type ActorRef struct {
	Kind     string `json:"kind"`
	ID       string `json:"id,omitempty"`
	Provider string `json:"provider,omitempty"`
}

type TrustClaim struct {
	Level string `json:"level"`
	Basis string `json:"basis,omitempty"`
}

type Scope struct {
	Repo   string   `json:"repo,omitempty"`
	Issue  string   `json:"issue,omitempty"`
	PR     string   `json:"pr,omitempty"`
	Areas  []string `json:"areas,omitempty"`
	Paths  []string `json:"paths,omitempty"`
	Global bool     `json:"global,omitempty"`
}

type SourceRef struct {
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

type Record struct {
	RecordID        string                     `json:"record_id"`
	Kind            string                     `json:"kind"`
	CreatedAt       string                     `json:"created_at"`
	Scope           Scope                      `json:"scope"`
	Source          SourceRef                  `json:"source"`
	Text            string                     `json:"text"`
	Confidence      string                     `json:"confidence"`
	TTL             string                     `json:"ttl"`
	Status          string                     `json:"status"`
	Durability      string                     `json:"durability"`
	ReviewType      string                     `json:"review_type,omitempty"`
	Reason          string                     `json:"reason,omitempty"`
	RejectedPaths   []string                   `json:"rejected_paths,omitempty"`
	PreferredPaths  []string                   `json:"preferred_paths,omitempty"`
	Evidence        []EvidenceRef              `json:"evidence,omitempty"`
	LifecycleReason string                     `json:"lifecycle_reason,omitempty"`
	ReviewedAt      string                     `json:"reviewed_at,omitempty"`
	Extensions      map[string]json.RawMessage `json:"extensions,omitempty"`
}

type RecordCreated struct {
	Record Record `json:"record"`
}

type RecordStateChanged struct {
	RecordID        string `json:"record_id"`
	Status          string `json:"status,omitempty"`
	Durability      string `json:"durability,omitempty"`
	LifecycleReason string `json:"lifecycle_reason,omitempty"`
	ReviewedAt      string `json:"reviewed_at,omitempty"`
}

type NodeRef struct {
	Kind     string `json:"kind"`
	ID       string `json:"id"`
	Provider string `json:"provider,omitempty"`
	URL      string `json:"url,omitempty"`
}

func (n NodeRef) Key() string {
	if n.Provider == "" {
		return n.Kind + ":" + n.ID
	}
	return n.Kind + ":" + n.Provider + ":" + n.ID
}

type Relation struct {
	RelationID string  `json:"relation_id"`
	Type       string  `json:"type"`
	From       NodeRef `json:"from"`
	To         NodeRef `json:"to"`
	CreatedAt  string  `json:"created_at"`
	Reason     string  `json:"reason,omitempty"`
}

type RelationCreated struct {
	Relation Relation `json:"relation"`
}

type Thread struct {
	ThreadID  string `json:"thread_id"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	Scope     Scope  `json:"scope"`
}

type ThreadEvent struct {
	Thread Thread `json:"thread"`
}

type MatchReason struct {
	Kind  string `json:"kind"`
	Value string `json:"value,omitempty"`
}

type MaterializationConflict struct {
	RecordID          string   `json:"record_id"`
	ParentEventID     string   `json:"parent_event_id"`
	CompetingEventIDs []string `json:"competing_event_ids"`
	Message           string   `json:"message"`
}

type Projection struct {
	ProjectionID string                    `json:"projection_id"`
	ThreadID     string                    `json:"thread_id,omitempty"`
	Purpose      string                    `json:"purpose"`
	CreatedAt    string                    `json:"created_at"`
	Scope        Scope                     `json:"scope"`
	EventIDs     []string                  `json:"event_ids"`
	RecordIDs    []string                  `json:"record_ids"`
	Reasons      map[string][]MatchReason  `json:"reasons,omitempty"`
	Conflicts    []MaterializationConflict `json:"conflicts,omitempty"`
	Omitted      bool                      `json:"omitted"`
}

type ProjectionCreated struct {
	Projection Projection `json:"projection"`
}

type Receipt struct {
	ReceiptID    string   `json:"receipt_id"`
	ProjectionID string   `json:"projection_id"`
	ThreadID     string   `json:"thread_id"`
	State        string   `json:"state"`
	OccurredAt   string   `json:"occurred_at"`
	EventIDs     []string `json:"event_ids"`
	RecordIDs    []string `json:"record_ids"`
	Provider     string   `json:"provider,omitempty"`
}

type ReceiptRecorded struct {
	Receipt Receipt `json:"receipt"`
}

type APIError struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

type APIResponse struct {
	ProtocolVersion string    `json:"protocol_version"`
	Command         string    `json:"command"`
	OK              bool      `json:"ok"`
	Data            any       `json:"data,omitempty"`
	Warnings        []string  `json:"warnings,omitempty"`
	Error           *APIError `json:"error,omitempty"`
}

type Graph struct {
	Root            NodeRef          `json:"root"`
	Nodes           []NodeRef        `json:"nodes"`
	NodeDetails     []NodeDetail     `json:"node_details,omitempty"`
	Relations       []Relation       `json:"relations"`
	RelationDetails []RelationDetail `json:"relation_details,omitempty"`
}

// RelationDetail identifies the immutable event that asserted an explanation edge.
type RelationDetail struct {
	RelationID string     `json:"relation_id"`
	EventID    string     `json:"event_id"`
	Actor      ActorRef   `json:"actor"`
	Trust      TrustClaim `json:"trust"`
}

type NodeDetail struct {
	Ref        NodeRef           `json:"ref"`
	Record     *RecordNodeDetail `json:"record,omitempty"`
	Projection *Projection       `json:"projection,omitempty"`
	Thread     *Thread           `json:"thread,omitempty"`
}

type RecordNodeDetail struct {
	Record      Record                   `json:"record"`
	HeadEventID string                   `json:"head_event_id"`
	Actor       ActorRef                 `json:"actor"`
	Trust       TrustClaim               `json:"trust"`
	HeadActor   ActorRef                 `json:"head_actor"`
	HeadTrust   TrustClaim               `json:"head_trust"`
	Conflict    *MaterializationConflict `json:"conflict,omitempty"`
}
