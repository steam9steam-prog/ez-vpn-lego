package postgres

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/steam9steam-prog/ez-vpn-lego/core/ports"
	"github.com/steam9steam-prog/ez-vpn-lego/internal/migrate"
)

func TestTelegramPairingIsSingleUse(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not configured")
	}
	ctx := context.Background()
	if err := migrate.Up(ctx, databaseURL); err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, `TRUNCATE admin_pairing_tokens, outbox_events, audit_events, operations, credentials, devices, users, admin_identities, admins CASCADE`); err != nil {
		t.Fatal(err)
	}

	repository := NewAdminRepository(pool)
	owner, err := repository.BootstrapOwner(ctx)
	if err != nil {
		t.Fatal(err)
	}
	pairing, err := repository.CreateTelegramPairing(ctx, owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.ResolveTelegram(ctx, "123456789"); !errors.Is(err, ports.ErrUnauthorizedActor) {
		t.Fatalf("resolve before claim: %v", err)
	}
	claimed, err := repository.ClaimTelegramPairing(ctx, pairing.Token, "123456789")
	if err != nil {
		t.Fatal(err)
	}
	if claimed.ID != owner.ID {
		t.Fatalf("claimed owner = %s, want %s", claimed.ID, owner.ID)
	}
	if _, err := repository.ClaimTelegramPairing(ctx, pairing.Token, "987654321"); !errors.Is(err, ports.ErrPairingTokenInvalid) {
		t.Fatalf("reuse pairing token: %v", err)
	}
	resolved, err := repository.ResolveTelegram(ctx, "123456789")
	if err != nil {
		t.Fatal(err)
	}
	if resolved.ID != owner.ID {
		t.Fatalf("resolved owner = %s, want %s", resolved.ID, owner.ID)
	}
}
