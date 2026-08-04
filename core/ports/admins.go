package ports

import (
	"context"
	"errors"
	"time"

	"github.com/steam9steam-prog/ez-vpn-lego/core/domain"
)

var ErrAlreadyBootstrapped = errors.New("owner is already bootstrapped")
var ErrPairingTokenInvalid = errors.New("pairing token is invalid or expired")
var ErrIdentityAlreadyBound = errors.New("telegram identity is already bound")

type PairingToken struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

type AdminService interface {
	BootstrapOwner(context.Context) (domain.Admin, error)
	CreateTelegramPairing(context.Context, string) (PairingToken, error)
	ClaimTelegramPairing(context.Context, string, string) (domain.Admin, error)
	ResolveTelegram(context.Context, string) (domain.Admin, error)
}
