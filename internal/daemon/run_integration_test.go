package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/steam9steam-prog/ez-vpn-lego/internal/config"
	"github.com/steam9steam-prog/ez-vpn-lego/internal/migrate"
)

func TestDaemonOverUnixSocket(t *testing.T) {
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
	if _, err := pool.Exec(ctx, `TRUNCATE outbox_events, audit_events, operations, credentials, devices, users, admin_identities, admins CASCADE`); err != nil {
		t.Fatal(err)
	}
	socketPath := filepath.Join(t.TempDir(), "control.sock")
	token := strings.Repeat("t", 32)
	daemonContext, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		result <- Run(daemonContext, config.Daemon{
			DatabaseURL:        databaseURL,
			SocketPath:         socketPath,
			APIToken:           token,
			MasterKey:          bytes.Repeat([]byte{1}, 32),
			HelperSocket:       filepath.Join(t.TempDir(), "helper.sock"),
			HelperToken:        strings.Repeat("h", 32),
			CandidateDirectory: filepath.Join(t.TempDir(), "candidates"),
		}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-result:
			if err != nil {
				t.Errorf("daemon shutdown: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Error("daemon did not stop")
		}
	})
	waitForSocket(t, socketPath)

	client := unixHTTPClient(socketPath)
	healthResponse, err := client.Get("http://unix/v1/health")
	if err != nil {
		t.Fatal(err)
	}
	defer healthResponse.Body.Close()
	if healthResponse.StatusCode != http.StatusOK {
		t.Fatalf("health status: %d", healthResponse.StatusCode)
	}
	request, _ := http.NewRequest(http.MethodPost, "http://unix/v1/bootstrap/owner", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	var owner struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(response.Body).Decode(&owner); err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusCreated || owner.ID == "" {
		t.Fatalf("bootstrap owner status=%d id=%q", response.StatusCode, owner.ID)
	}

	request, err = http.NewRequest(http.MethodPost, "http://unix/v1/users", bytes.NewBufferString(`{"name":"Alice"}`))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("X-Admin-ID", owner.ID)
	request.Header.Set("Idempotency-Key", "daemon-create-user-0001")
	response, err = client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("create status: %d; body=%s", response.StatusCode, body)
	}

	request, _ = http.NewRequest(http.MethodGet, "http://unix/v1/users", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("X-Admin-ID", owner.ID)
	response, err = client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var users []struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(response.Body).Decode(&users); err != nil {
		t.Fatal(err)
	}
	if len(users) != 1 || users[0].Name != "Alice" {
		t.Fatalf("unexpected users: %+v", users)
	}
}

func waitForSocket(t *testing.T, socketPath string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		connection, err := net.DialTimeout("unix", socketPath, 100*time.Millisecond)
		if err == nil {
			_ = connection.Close()
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("socket %q was not created", socketPath)
}

func unixHTTPClient(socketPath string) *http.Client {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", socketPath)
		},
	}
	return &http.Client{Transport: transport, Timeout: 5 * time.Second}
}
