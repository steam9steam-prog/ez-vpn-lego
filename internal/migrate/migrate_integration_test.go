package migrate

import (
	"context"
	"database/sql"
	"os"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestUpAndDown(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not configured")
	}
	ctx := context.Background()
	if err := Up(ctx, databaseURL); err != nil {
		t.Fatal(err)
	}
	assertTableCount(t, databaseURL, 15) // 14 application tables plus goose_db_version.
	if err := DownToZero(ctx, databaseURL); err != nil {
		t.Fatal(err)
	}
	assertTableCount(t, databaseURL, 1) // Goose retains its version table.
	if err := Up(ctx, databaseURL); err != nil {
		t.Fatal(err)
	}
}

func assertTableCount(t *testing.T, databaseURL string, want int) {
	t.Helper()
	database, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	var got int
	if err := database.QueryRow(`SELECT count(*) FROM information_schema.tables WHERE table_schema = 'public'`).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("table count: got %d, want %d", got, want)
	}
}
