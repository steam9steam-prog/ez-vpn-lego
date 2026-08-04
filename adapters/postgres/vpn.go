package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/steam9steam-prog/ez-vpn-lego/core/domain"
	"github.com/steam9steam-prog/ez-vpn-lego/core/ports"
	"github.com/steam9steam-prog/ez-vpn-lego/internal/dbgen"
	"github.com/steam9steam-prog/ez-vpn-lego/internal/id"
)

type VPNRepository struct{ pool *pgxpool.Pool }

type StageVPNParams struct {
	AdminID, IdempotencyKey            string
	OperationID, UserID, DeviceID      string
	NodeID, InstanceID, CredentialID   string
	RevisionID, PublicAddress          string
	Driver                             string
	InstanceSettings, CredentialSecret []byte
	RevisionContent                    []byte
	RevisionHash                       string
	OperationInput                     []byte
}

type FinalizeVPNParams struct {
	AdminID, OperationID, UserID, DeviceID string
	NodeID, CredentialID, RevisionID       string
	Result                                 []byte
}

func NewVPNRepository(pool *pgxpool.Pool) *VPNRepository { return &VPNRepository{pool: pool} }

func (repository *VPNRepository) Stage(ctx context.Context, input StageVPNParams) error {
	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin VPN bootstrap: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := dbgen.New(tx)
	if err := queries.LockVPNBootstrap(ctx); err != nil {
		return fmt.Errorf("lock VPN bootstrap: %w", err)
	}
	if _, err := queries.GetActiveAdmin(ctx, input.AdminID); errors.Is(err, pgx.ErrNoRows) {
		return ports.ErrUnauthorizedActor
	} else if err != nil {
		return fmt.Errorf("load bootstrap administrator: %w", err)
	}
	count, err := queries.CountNodes(ctx)
	if err != nil {
		return fmt.Errorf("count nodes: %w", err)
	}
	if count != 0 {
		return ports.ErrVPNAlreadyBootstrapped
	}
	adminID, err := nullableUUID(input.AdminID)
	if err != nil {
		return fmt.Errorf("parse bootstrap administrator: %w", err)
	}
	if _, err := queries.CreateOperation(ctx, dbgen.CreateOperationParams{
		ID: input.OperationID, Kind: "vpn.bootstrap", Status: string(domain.OperationRunning),
		RequestedBy: adminID, IdempotencyKey: input.IdempotencyKey, Input: input.OperationInput,
	}); err != nil {
		return fmt.Errorf("create bootstrap operation: %w", err)
	}
	if _, err := queries.CreateUser(ctx, dbgen.CreateUserParams{ID: input.UserID, Name: "Owner"}); err != nil {
		return fmt.Errorf("create owner VPN user: %w", err)
	}
	if _, err := queries.CreateDevice(ctx, dbgen.CreateDeviceParams{ID: input.DeviceID, UserID: input.UserID, Name: "Owner device"}); err != nil {
		return fmt.Errorf("create owner device: %w", err)
	}
	if _, err := queries.CreateNode(ctx, dbgen.CreateNodeParams{ID: input.NodeID, Name: "local", PublicAddress: input.PublicAddress}); err != nil {
		return fmt.Errorf("create local node: %w", err)
	}
	if err := queries.CreateProtocolInstance(ctx, dbgen.CreateProtocolInstanceParams{
		ID: input.InstanceID, NodeID: input.NodeID, Driver: input.Driver,
		SchemaVersion: 1, Settings: input.InstanceSettings,
	}); err != nil {
		return fmt.Errorf("create protocol instance: %w", err)
	}
	if err := queries.CreateCredential(ctx, dbgen.CreateCredentialParams{
		ID: input.CredentialID, DeviceID: input.DeviceID, NodeID: input.NodeID,
		Driver: input.Driver, SchemaVersion: 1, Secret: input.CredentialSecret,
	}); err != nil {
		return fmt.Errorf("create owner credential: %w", err)
	}
	if err := queries.CreateConfigRevision(ctx, dbgen.CreateConfigRevisionParams{
		ID: input.RevisionID, NodeID: input.NodeID, ContentHash: input.RevisionHash, Content: input.RevisionContent,
	}); err != nil {
		return fmt.Errorf("create configuration revision: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit staged VPN bootstrap: %w", err)
	}
	return nil
}

func (repository *VPNRepository) Finalize(ctx context.Context, input FinalizeVPNParams) error {
	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := dbgen.New(tx)
	if err := queries.ActivateNode(ctx, input.NodeID); err != nil {
		return fmt.Errorf("activate VPN state: %w", err)
	}
	if err := queries.ActivateProtocolInstances(ctx, input.NodeID); err != nil {
		return fmt.Errorf("activate protocol instance: %w", err)
	}
	if err := queries.ActivateDevice(ctx, input.DeviceID); err != nil {
		return fmt.Errorf("activate owner device: %w", err)
	}
	if err := queries.ActivateCredential(ctx, input.CredentialID); err != nil {
		return fmt.Errorf("activate owner credential: %w", err)
	}
	if err := queries.VerifyConfigRevision(ctx, input.RevisionID); err != nil {
		return fmt.Errorf("verify configuration revision: %w", err)
	}
	if _, err := queries.CompleteOperation(ctx, dbgen.CompleteOperationParams{ID: input.OperationID, Result: input.Result}); err != nil {
		return fmt.Errorf("complete VPN bootstrap operation: %w", err)
	}
	if err := repository.recordBootstrapSuccess(ctx, queries, input); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (repository *VPNRepository) Fail(ctx context.Context, operationID, revisionID, code string) error {
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

func (repository *VPNRepository) recordBootstrapSuccess(ctx context.Context, queries *dbgen.Queries, input FinalizeVPNParams) error {
	auditID, err := id.NewUUID()
	if err != nil {
		return err
	}
	outboxID, err := id.NewUUID()
	if err != nil {
		return err
	}
	actor, _ := nullableUUID(input.AdminID)
	resource, _ := nullableUUID(input.NodeID)
	payload, _ := json.Marshal(map[string]string{"node_id": input.NodeID, "device_id": input.DeviceID})
	if err := queries.CreateAuditEvent(ctx, dbgen.CreateAuditEventParams{
		ID: auditID, ActorAdminID: actor, Action: "vpn.bootstrap", ResourceType: "node",
		ResourceID: resource, Outcome: "succeeded", Metadata: payload,
	}); err != nil {
		return err
	}
	return queries.CreateOutboxEvent(ctx, dbgen.CreateOutboxEventParams{
		ID: outboxID, Topic: "vpn.bootstrapped", AggregateType: "node", AggregateID: input.NodeID, Payload: payload,
	})
}
