package domain

import "time"

type Admin struct {
	ID        string
	Role      AdminRole
	Status    LifecycleStatus
	CreatedAt time.Time
	UpdatedAt time.Time
}

type AdminRole string

const (
	AdminOwner    AdminRole = "owner"
	AdminOperator AdminRole = "operator"
	AdminViewer   AdminRole = "viewer"
)

type User struct {
	ID        string
	Name      string
	Status    LifecycleStatus
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Device struct {
	ID        string
	UserID    string
	Name      string
	Status    LifecycleStatus
	ExpiresAt *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Credential struct {
	ID            string
	DeviceID      string
	NodeID        string
	Driver        string
	SchemaVersion int
	Status        LifecycleStatus
	Secret        []byte
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type Node struct {
	ID        string
	Name      string
	Status    LifecycleStatus
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Operation struct {
	ID             string
	Kind           string
	Status         OperationStatus
	IdempotencyKey string
	ErrorCode      string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
