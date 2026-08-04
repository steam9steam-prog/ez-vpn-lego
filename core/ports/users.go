package ports

import (
	"context"
	"errors"

	"github.com/steam9steam-prog/ez-vpn-lego/core/domain"
)

var (
	ErrIdempotencyConflict = errors.New("idempotency key was already used for another request")
	ErrInvalidArgument     = errors.New("invalid argument")
	ErrUnauthorizedActor   = errors.New("administrator is not active")
)

type CreateUserRequest struct {
	AdminID        string
	Name           string
	IdempotencyKey string
}

type CreateUserResult struct {
	OperationID string
	User        domain.User
	Replayed    bool
}

type UserService interface {
	List(context.Context) ([]domain.User, error)
	Create(context.Context, CreateUserRequest) (CreateUserResult, error)
}
