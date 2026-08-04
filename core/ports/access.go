package ports

import "context"

type CreateAccessRequest struct {
	AdminID        string
	Name           string
	IdempotencyKey string
}

type CreateAccessResult struct {
	OperationID string `json:"operation_id"`
	UserID      string `json:"user_id"`
	DeviceID    string `json:"device_id"`
	URI         string `json:"uri"`
}

type AccessService interface {
	Create(context.Context, CreateAccessRequest) (CreateAccessResult, error)
}
