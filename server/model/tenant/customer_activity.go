package tenant

import "time"

type Followup struct {
	ID         uint64    `json:"id,string" swaggertype:"string"`
	TenantID   uint64    `json:"tenantId,string" swaggertype:"string"`
	CustomerID uint64    `json:"customerId,string" swaggertype:"string"`
	ActorID    uint64    `json:"actorId,string" swaggertype:"string"`
	Content    string    `json:"content"`
	CreatedAt  time.Time `json:"createdAt"`
}

func (Followup) TableName() string { return "crm_followups" }

// AuditEvent stores an allowlist of identifiers, never request bodies or contact values.
type AuditEvent struct {
	ID               uint64    `json:"id,string" swaggertype:"string"`
	TenantID         uint64    `json:"tenantId,string" swaggertype:"string"`
	CustomerID       uint64    `json:"customerId,string" swaggertype:"string"`
	ActorID          uint64    `json:"actorId,string" swaggertype:"string"`
	Action           string    `json:"action"`
	SubjectID        uint64    `json:"subjectId,string" swaggertype:"string"`
	FromOwnerID      uint64    `json:"fromOwnerId,string" swaggertype:"string"`
	ToOwnerID        uint64    `json:"toOwnerId,string" swaggertype:"string"`
	FromDepartmentID uint64    `json:"fromDepartmentId,string" swaggertype:"string"`
	ToDepartmentID   uint64    `json:"toDepartmentId,string" swaggertype:"string"`
	RequestID        string    `json:"requestId"`
	CreatedAt        time.Time `json:"createdAt"`
}

func (AuditEvent) TableName() string { return "crm_audit_events" }
