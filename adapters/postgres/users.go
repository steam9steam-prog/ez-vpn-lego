package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/steam9steam-prog/ez-vpn-lego/core/domain"
	"github.com/steam9steam-prog/ez-vpn-lego/core/ports"
	"github.com/steam9steam-prog/ez-vpn-lego/internal/dbgen"
	"github.com/steam9steam-prog/ez-vpn-lego/internal/id"
)

type UserRepository struct {
	pool *pgxpool.Pool
}

type createUserInput struct {
	UserID string `json:"user_id"`
	Name   string `json:"name"`
}

type createUserOutput struct {
	UserID string `json:"user_id"`
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

func (r *UserRepository) List(ctx context.Context) ([]domain.User, error) {
	rows, err := dbgen.New(r.pool).ListUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	users := make([]domain.User, 0, len(rows))
	for _, row := range rows {
		users = append(users, mapUser(row))
	}
	return users, nil
}

func (r *UserRepository) Create(ctx context.Context, request ports.CreateUserRequest) (ports.CreateUserResult, error) {
	request.Name = strings.TrimSpace(request.Name)
	if request.Name == "" || len([]rune(request.Name)) > 80 {
		return ports.CreateUserResult{}, fmt.Errorf("%w: user name must contain between 1 and 80 characters", ports.ErrInvalidArgument)
	}
	if len(request.IdempotencyKey) < 16 || len(request.IdempotencyKey) > 128 {
		return ports.CreateUserResult{}, fmt.Errorf("%w: idempotency key must contain between 16 and 128 characters", ports.ErrInvalidArgument)
	}

	adminID, err := nullableUUID(request.AdminID)
	if err != nil {
		return ports.CreateUserResult{}, fmt.Errorf("%w: administrator ID", ports.ErrInvalidArgument)
	}
	userID, err := id.NewUUID()
	if err != nil {
		return ports.CreateUserResult{}, err
	}
	operationID, err := id.NewUUID()
	if err != nil {
		return ports.CreateUserResult{}, err
	}
	input, err := json.Marshal(createUserInput{UserID: userID, Name: request.Name})
	if err != nil {
		return ports.CreateUserResult{}, fmt.Errorf("encode operation input: %w", err)
	}

	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return ports.CreateUserResult{}, fmt.Errorf("begin create user transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := dbgen.New(tx)
	if _, err := queries.GetActiveAdmin(ctx, request.AdminID); errors.Is(err, pgx.ErrNoRows) {
		return ports.CreateUserResult{}, ports.ErrUnauthorizedActor
	} else if err != nil {
		return ports.CreateUserResult{}, fmt.Errorf("load administrator: %w", err)
	}

	operation, err := queries.CreateOperation(ctx, dbgen.CreateOperationParams{
		ID:             operationID,
		Kind:           "user.create",
		Status:         string(domain.OperationRunning),
		RequestedBy:    adminID,
		IdempotencyKey: request.IdempotencyKey,
		Input:          input,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return r.replayExisting(ctx, tx, queries, adminID, request)
	}
	if err != nil {
		return ports.CreateUserResult{}, fmt.Errorf("create operation: %w", err)
	}

	created, err := queries.CreateUser(ctx, dbgen.CreateUserParams{ID: userID, Name: request.Name})
	if err != nil {
		return ports.CreateUserResult{}, fmt.Errorf("create user: %w", err)
	}
	if err := r.recordCreation(ctx, queries, request.AdminID, userID, request.Name); err != nil {
		return ports.CreateUserResult{}, err
	}
	result, err := json.Marshal(createUserOutput{UserID: userID})
	if err != nil {
		return ports.CreateUserResult{}, fmt.Errorf("encode operation result: %w", err)
	}
	if _, err := queries.CompleteOperation(ctx, dbgen.CompleteOperationParams{ID: operation.ID, Result: result}); err != nil {
		return ports.CreateUserResult{}, fmt.Errorf("complete operation: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return ports.CreateUserResult{}, fmt.Errorf("commit create user transaction: %w", err)
	}

	return ports.CreateUserResult{OperationID: operation.ID, User: mapUser(created)}, nil
}

func (r *UserRepository) replayExisting(
	ctx context.Context,
	tx pgx.Tx,
	queries *dbgen.Queries,
	adminID pgtype.UUID,
	request ports.CreateUserRequest,
) (ports.CreateUserResult, error) {
	operation, err := queries.GetOperationByIdempotencyKey(ctx, dbgen.GetOperationByIdempotencyKeyParams{
		RequestedBy:    adminID,
		IdempotencyKey: request.IdempotencyKey,
	})
	if err != nil {
		return ports.CreateUserResult{}, fmt.Errorf("load idempotent operation: %w", err)
	}
	var input createUserInput
	if err := json.Unmarshal(operation.Input, &input); err != nil {
		return ports.CreateUserResult{}, fmt.Errorf("decode existing operation input: %w", err)
	}
	if operation.Kind != "user.create" || input.Name != request.Name {
		return ports.CreateUserResult{}, ports.ErrIdempotencyConflict
	}
	created, err := queries.GetUser(ctx, input.UserID)
	if err != nil {
		return ports.CreateUserResult{}, fmt.Errorf("load idempotently created user: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return ports.CreateUserResult{}, fmt.Errorf("commit idempotent read: %w", err)
	}
	return ports.CreateUserResult{OperationID: operation.ID, User: mapUser(created), Replayed: true}, nil
}

func (r *UserRepository) recordCreation(
	ctx context.Context,
	queries *dbgen.Queries,
	adminID string,
	userID string,
	name string,
) error {
	auditID, err := id.NewUUID()
	if err != nil {
		return err
	}
	outboxID, err := id.NewUUID()
	if err != nil {
		return err
	}
	actor, err := nullableUUID(adminID)
	if err != nil {
		return err
	}
	resource, err := nullableUUID(userID)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(map[string]string{"user_id": userID, "name": name})
	if err != nil {
		return fmt.Errorf("encode user event: %w", err)
	}
	if err := queries.CreateAuditEvent(ctx, dbgen.CreateAuditEventParams{
		ID:           auditID,
		ActorAdminID: actor,
		Action:       "user.create",
		ResourceType: "user",
		ResourceID:   resource,
		Outcome:      "succeeded",
		Metadata:     payload,
	}); err != nil {
		return fmt.Errorf("create audit event: %w", err)
	}
	if err := queries.CreateOutboxEvent(ctx, dbgen.CreateOutboxEventParams{
		ID:            outboxID,
		Topic:         "user.created",
		AggregateType: "user",
		AggregateID:   userID,
		Payload:       payload,
	}); err != nil {
		return fmt.Errorf("create outbox event: %w", err)
	}
	return nil
}

func nullableUUID(value string) (pgtype.UUID, error) {
	var parsed pgtype.UUID
	if err := parsed.Scan(value); err != nil {
		return pgtype.UUID{}, err
	}
	return parsed, nil
}

func mapUser(user dbgen.User) domain.User {
	return domain.User{
		ID:        user.ID,
		Name:      user.Name,
		Status:    domain.LifecycleStatus(user.Status),
		CreatedAt: user.CreatedAt.Time,
		UpdatedAt: user.UpdatedAt.Time,
	}
}
