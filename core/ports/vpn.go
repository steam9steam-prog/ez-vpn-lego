package ports

import (
	"context"
	"errors"
)

var ErrVPNAlreadyBootstrapped = errors.New("VPN node is already bootstrapped")

type BootstrapVPNRequest struct {
	AdminID        string
	PublicAddress  string
	Port           uint16
	Target         string
	ServerName     string
	IdempotencyKey string
}

type BootstrapVPNResult struct {
	OperationID string `json:"operation_id"`
	UserID      string `json:"user_id"`
	DeviceID    string `json:"device_id"`
	URI         string `json:"uri"`
}

type VPNBootstrapService interface {
	Bootstrap(context.Context, BootstrapVPNRequest) (BootstrapVPNResult, error)
}
