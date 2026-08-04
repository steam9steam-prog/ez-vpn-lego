package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/steam9steam-prog/ez-vpn-lego/core/domain"
	"github.com/steam9steam-prog/ez-vpn-lego/core/ports"
)

type fakeHealth struct{ err error }

func (health fakeHealth) Ping(context.Context) error { return health.err }

type fakeUsers struct {
	users  []domain.User
	result ports.CreateUserResult
}

type fakeAdmins struct{ admin domain.Admin }

func (service fakeAdmins) BootstrapOwner(context.Context) (domain.Admin, error) {
	return service.admin, nil
}

func (fakeAdmins) CreateTelegramPairing(context.Context, string) (ports.PairingToken, error) {
	return ports.PairingToken{}, nil
}

func (service fakeAdmins) ClaimTelegramPairing(context.Context, string, string) (domain.Admin, error) {
	return service.admin, nil
}

func (service fakeAdmins) ResolveTelegram(context.Context, string) (domain.Admin, error) {
	return service.admin, nil
}

type fakeVPN struct{}

func (fakeVPN) Bootstrap(context.Context, ports.BootstrapVPNRequest) (ports.BootstrapVPNResult, error) {
	return ports.BootstrapVPNResult{}, nil
}

type fakeAccess struct{}

func (fakeAccess) Create(context.Context, ports.CreateAccessRequest) (ports.CreateAccessResult, error) {
	return ports.CreateAccessResult{}, nil
}

func (service *fakeUsers) List(context.Context) ([]domain.User, error) { return service.users, nil }
func (service *fakeUsers) Create(_ context.Context, request ports.CreateUserRequest) (ports.CreateUserResult, error) {
	if request.AdminID == "" || request.IdempotencyKey == "" {
		return ports.CreateUserResult{}, errors.New("missing request identity")
	}
	return service.result, nil
}

func TestHealthDoesNotRequireAuthentication(t *testing.T) {
	server := New(&fakeUsers{}, fakeAdmins{}, fakeVPN{}, fakeAccess{}, fakeHealth{}, strings.Repeat("a", 32))
	request := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", response.Code, http.StatusOK)
	}
}

func TestUsersRequireAuthentication(t *testing.T) {
	server := New(&fakeUsers{}, fakeAdmins{}, fakeVPN{}, fakeAccess{}, fakeHealth{}, strings.Repeat("a", 32))
	request := httptest.NewRequest(http.MethodGet, "/v1/users", nil)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status: got %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestCreateUser(t *testing.T) {
	created := domain.User{ID: "user-id", Name: "Alice", Status: domain.LifecycleActive, CreatedAt: time.Now()}
	server := New(&fakeUsers{result: ports.CreateUserResult{OperationID: "operation-id", User: created}}, fakeAdmins{}, fakeVPN{}, fakeAccess{}, fakeHealth{}, strings.Repeat("a", 32))
	request := httptest.NewRequest(http.MethodPost, "/v1/users", strings.NewReader(`{"name":"Alice"}`))
	request.Header.Set("Authorization", "Bearer "+strings.Repeat("a", 32))
	request.Header.Set("X-Admin-ID", "admin-id")
	request.Header.Set("Idempotency-Key", "create-user-test-0001")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d; body=%s", response.Code, http.StatusCreated, response.Body.String())
	}
	var body struct {
		OperationID string `json:"operation_id"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.OperationID != "operation-id" {
		t.Fatalf("operation ID: got %q", body.OperationID)
	}
}
