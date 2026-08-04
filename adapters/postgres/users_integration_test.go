package postgres

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/steam9steam-prog/ez-vpn-lego/core/ports"
	"github.com/steam9steam-prog/ez-vpn-lego/internal/id"
)

func TestCreateUserIsAtomicAndIdempotent(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not configured")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(ctx, `TRUNCATE outbox_events, audit_events, operations, credentials, devices, users, admin_identities, admins CASCADE`); err != nil {
		t.Fatal(err)
	}

	adminID, err := id.NewUUID()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO admins (id, role, status) VALUES ($1, 'owner', 'active')`, adminID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `TRUNCATE outbox_events, audit_events, operations, credentials, devices, users, admin_identities, admins CASCADE`)
	})

	repository := NewUserRepository(pool)
	request := ports.CreateUserRequest{
		AdminID:        adminID,
		Name:           "Alice",
		IdempotencyKey: "test-create-user-0001",
	}
	created, err := repository.Create(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := repository.Create(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if !replayed.Replayed || replayed.User.ID != created.User.ID || replayed.OperationID != created.OperationID {
		t.Fatalf("unexpected replay: first=%+v replay=%+v", created, replayed)
	}

	conflicting := request
	conflicting.Name = "Bob"
	if _, err := repository.Create(ctx, conflicting); !errors.Is(err, ports.ErrIdempotencyConflict) {
		t.Fatalf("expected idempotency conflict, got %v", err)
	}

	for table, want := range map[string]int{"users": 1, "operations": 1, "audit_events": 1, "outbox_events": 1} {
		var got int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM `+table).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("%s count: got %d, want %d", table, got, want)
		}
	}
}
