package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/steam9steam-prog/ez-vpn-lego/core/domain"
	"github.com/steam9steam-prog/ez-vpn-lego/internal/dbgen"
)

type AccessRepository struct{ pool *pgxpool.Pool }

type AccessState struct {
	InstanceID, NodeID, PublicAddress string
	InstanceSettings                  []byte
	Credentials                       []EncryptedCredential
}

type EncryptedCredential struct {
	ID, Name string
	Secret   []byte
}

type StageAccessParams struct {
	AdminID, IdempotencyKey, Name     string
	OperationID, UserID, DeviceID     string
	CredentialID, NodeID, RevisionID  string
	Driver, RevisionHash              string
	CredentialSecret, RevisionContent []byte
	OperationInput                    []byte
}

type FinalizeAccessParams struct {
	AdminID, OperationID, UserID, DeviceID string
	CredentialID, NodeID, RevisionID       string
	Result                                 []byte
}

func NewAccessRepository(pool *pgxpool.Pool) *AccessRepository { return &AccessRepository{pool: pool} }

func (repository *AccessRepository) Load(ctx context.Context) (AccessState, error) {
	queries := dbgen.New(repository.pool)
	instance, err := queries.GetActiveRealityInstance(ctx)
	if err != nil {
		return AccessState{}, fmt.Errorf("load active Reality instance: %w", err)
	}
	rows, err := queries.ListActiveRealityCredentials(ctx, instance.NodeID)
	if err != nil {
		return AccessState{}, fmt.Errorf("load active Reality credentials: %w", err)
	}
	result := AccessState{
		InstanceID: instance.ID, NodeID: instance.NodeID,
		PublicAddress: instance.PublicAddress, InstanceSettings: instance.Settings,
		Credentials: make([]EncryptedCredential, 0, len(rows)),
	}
	for _, row := range rows {
		result.Credentials = append(result.Credentials, EncryptedCredential{ID: row.ID, Name: row.Name, Secret: row.Secret})
	}
	return result, nil
}

func (repository *AccessRepository) Stage(ctx context.Context, input StageAccessParams) error {
	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := dbgen.New(tx)
	if err := queries.LockXrayReconcile(ctx); err != nil {
		return err
	}
	if _, err := queries.GetActiveAdmin(ctx, input.AdminID); err != nil {
		return fmt.Errorf("load access administrator: %w", err)
	}
	adminID, _ := nullableUUID(input.AdminID)
	if _, err := queries.CreateOperation(ctx, dbgen.CreateOperationParams{
		ID: input.OperationID, Kind: "access.create", Status: string(domain.OperationRunning),
		RequestedBy: adminID, IdempotencyKey: input.IdempotencyKey, Input: input.OperationInput,
	}); err != nil {
		return fmt.Errorf("create access operation: %w", err)
	}
	if _, err := queries.CreateUser(ctx, dbgen.CreateUserParams{ID: input.UserID, Name: input.Name}); err != nil {
		return err
	}
	if _, err := queries.CreateDevice(ctx, dbgen.CreateDeviceParams{ID: input.DeviceID, UserID: input.UserID, Name: input.Name}); err != nil {
		return err
	}
	if err := queries.CreateCredential(ctx, dbgen.CreateCredentialParams{
		ID: input.CredentialID, DeviceID: input.DeviceID, NodeID: input.NodeID,
		Driver: input.Driver, SchemaVersion: 1, Secret: input.CredentialSecret,
	}); err != nil {
		return err
	}
	if err := queries.CreateConfigRevision(ctx, dbgen.CreateConfigRevisionParams{
		ID: input.RevisionID, NodeID: input.NodeID, ContentHash: input.RevisionHash, Content: input.RevisionContent,
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (repository *AccessRepository) Finalize(ctx context.Context, input FinalizeAccessParams) error {
	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := dbgen.New(tx)
	if err := queries.SupersedeVerifiedRevisions(ctx, input.NodeID); err != nil {
		return err
	}
	if err := queries.ActivateDevice(ctx, input.DeviceID); err != nil {
		return err
	}
	if err := queries.ActivateCredential(ctx, input.CredentialID); err != nil {
		return err
	}
	if err := queries.VerifyConfigRevision(ctx, input.RevisionID); err != nil {
		return err
	}
	if _, err := queries.CompleteOperation(ctx, dbgen.CompleteOperationParams{ID: input.OperationID, Result: input.Result}); err != nil {
		return err
	}
	actor, _ := nullableUUID(input.AdminID)
	resource, _ := nullableUUID(input.UserID)
	if err := queries.CreateAuditEvent(ctx, dbgen.CreateAuditEventParams{
		ID: input.OperationID, ActorAdminID: actor, Action: "access.create", ResourceType: "user",
		ResourceID: resource, Outcome: "succeeded", Metadata: []byte(`{}`),
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (repository *AccessRepository) Fail(ctx context.Context, operationID, revisionID, code string) error {
	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := dbgen.New(tx)
	if err := queries.FailConfigRevision(ctx, revisionID); err != nil {
		return err
	}
	if err := queries.FailOperation(ctx, dbgen.FailOperationParams{ID: operationID, ErrorCode: pgtype.Text{String: code, Valid: true}}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
