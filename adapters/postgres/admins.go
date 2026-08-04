package postgres

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/steam9steam-prog/ez-vpn-lego/core/domain"
	"github.com/steam9steam-prog/ez-vpn-lego/core/ports"
	"github.com/steam9steam-prog/ez-vpn-lego/internal/dbgen"
	"github.com/steam9steam-prog/ez-vpn-lego/internal/id"
)

type AdminRepository struct {
	pool *pgxpool.Pool
}

func (r *AdminRepository) CreateTelegramPairing(ctx context.Context, adminID string) (ports.PairingToken, error) {
	var validatedAdminID pgtype.UUID
	if err := validatedAdminID.Scan(adminID); err != nil {
		return ports.PairingToken{}, ports.ErrUnauthorizedActor
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return ports.PairingToken{}, fmt.Errorf("generate pairing token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	hash := sha256.Sum256([]byte(token))
	expiresAt := time.Now().UTC().Add(15 * time.Minute)
	rows, err := dbgen.New(r.pool).CreateTelegramPairingToken(ctx, dbgen.CreateTelegramPairingTokenParams{
		TokenHash: hash[:], AdminID: adminID, ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})
	if err != nil {
		return ports.PairingToken{}, fmt.Errorf("store pairing token: %w", err)
	}
	if rows != 1 {
		return ports.PairingToken{}, ports.ErrUnauthorizedActor
	}
	return ports.PairingToken{Token: token, ExpiresAt: expiresAt}, nil
}

func (r *AdminRepository) ClaimTelegramPairing(ctx context.Context, token string, subject string) (domain.Admin, error) {
	if len(token) < 32 || subject == "" {
		return domain.Admin{}, ports.ErrPairingTokenInvalid
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.Admin{}, fmt.Errorf("begin telegram pairing: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := dbgen.New(tx)
	hash := sha256.Sum256([]byte(token))
	adminID, err := queries.ConsumeTelegramPairingToken(ctx, hash[:])
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Admin{}, ports.ErrPairingTokenInvalid
	}
	if err != nil {
		return domain.Admin{}, fmt.Errorf("consume pairing token: %w", err)
	}
	identityID, err := id.NewUUID()
	if err != nil {
		return domain.Admin{}, err
	}
	if err := queries.CreateTelegramIdentity(ctx, dbgen.CreateTelegramIdentityParams{ID: identityID, AdminID: adminID, Subject: subject}); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.Admin{}, ports.ErrIdentityAlreadyBound
		}
		return domain.Admin{}, fmt.Errorf("bind telegram identity: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Admin{}, fmt.Errorf("commit telegram pairing: %w", err)
	}
	return r.ResolveTelegram(ctx, subject)
}

func (r *AdminRepository) ResolveTelegram(ctx context.Context, subject string) (domain.Admin, error) {
	admin, err := dbgen.New(r.pool).ResolveTelegramIdentity(ctx, subject)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Admin{}, ports.ErrUnauthorizedActor
	}
	if err != nil {
		return domain.Admin{}, fmt.Errorf("resolve telegram identity: %w", err)
	}
	return domain.Admin{ID: admin.ID, Role: domain.AdminRole(admin.Role), Status: domain.LifecycleStatus(admin.Status), CreatedAt: admin.CreatedAt.Time, UpdatedAt: admin.UpdatedAt.Time}, nil
}

func NewAdminRepository(pool *pgxpool.Pool) *AdminRepository {
	return &AdminRepository{pool: pool}
}

func (r *AdminRepository) BootstrapOwner(ctx context.Context) (domain.Admin, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.Admin{}, fmt.Errorf("begin owner bootstrap: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := dbgen.New(tx)
	if err := queries.LockOwnerBootstrap(ctx); err != nil {
		return domain.Admin{}, fmt.Errorf("lock owner bootstrap: %w", err)
	}
	count, err := queries.CountAdmins(ctx)
	if err != nil {
		return domain.Admin{}, fmt.Errorf("count administrators: %w", err)
	}
	if count != 0 {
		return domain.Admin{}, ports.ErrAlreadyBootstrapped
	}
	adminID, err := id.NewUUID()
	if err != nil {
		return domain.Admin{}, err
	}
	created, err := queries.CreateOwner(ctx, adminID)
	if err != nil {
		return domain.Admin{}, fmt.Errorf("create owner: %w", err)
	}
	auditID, err := id.NewUUID()
	if err != nil {
		return domain.Admin{}, err
	}
	outboxID, err := id.NewUUID()
	if err != nil {
		return domain.Admin{}, err
	}
	resource, err := nullableUUID(adminID)
	if err != nil {
		return domain.Admin{}, err
	}
	if err := queries.CreateAuditEvent(ctx, dbgen.CreateAuditEventParams{
		ID: auditID, Action: "admin.owner.bootstrap", ResourceType: "admin",
		ResourceID: resource, Outcome: "succeeded", Metadata: []byte(`{}`),
	}); err != nil {
		return domain.Admin{}, fmt.Errorf("audit owner bootstrap: %w", err)
	}
	if err := queries.CreateOutboxEvent(ctx, dbgen.CreateOutboxEventParams{
		ID: outboxID, Topic: "admin.owner.bootstrapped", AggregateType: "admin",
		AggregateID: adminID, Payload: []byte(`{}`),
	}); err != nil {
		return domain.Admin{}, fmt.Errorf("publish owner bootstrap: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		if errors.Is(err, pgx.ErrTxCommitRollback) {
			return domain.Admin{}, ports.ErrAlreadyBootstrapped
		}
		return domain.Admin{}, fmt.Errorf("commit owner bootstrap: %w", err)
	}
	return domain.Admin{
		ID: created.ID, Role: domain.AdminRole(created.Role), Status: domain.LifecycleStatus(created.Status),
		CreatedAt: created.CreatedAt.Time, UpdatedAt: created.UpdatedAt.Time,
	}, nil
}
