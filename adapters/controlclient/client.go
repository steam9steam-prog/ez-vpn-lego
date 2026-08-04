package controlclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/steam9steam-prog/ez-vpn-lego/internal/config"
)

type Client struct {
	http    *http.Client
	token   string
	adminID string
}

type APIError struct {
	Status  int
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (err *APIError) Error() string {
	return fmt.Sprintf("control API: %s (%s, HTTP %d)", err.Message, err.Code, err.Status)
}

type Health struct {
	Status  string `json:"status"`
	Version string `json:"version"`
}

type User struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type UserCreation struct {
	OperationID string `json:"operation_id"`
	User        User   `json:"user"`
	Replayed    bool   `json:"replayed"`
}

type Owner struct {
	ID        string    `json:"id"`
	Role      string    `json:"role"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type VPNBootstrap struct {
	OperationID string `json:"operation_id"`
	UserID      string `json:"user_id"`
	DeviceID    string `json:"device_id"`
	URI         string `json:"uri"`
}

type AccessCreation struct {
	OperationID string `json:"operation_id"`
	UserID      string `json:"user_id"`
	DeviceID    string `json:"device_id"`
	URI         string `json:"uri"`
}
type PairingToken struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

func New(configuration config.Client) *Client {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", configuration.SocketPath)
		},
	}
	return &Client{
		http:  &http.Client{Transport: transport, Timeout: 30 * time.Second},
		token: configuration.APIToken, adminID: configuration.AdminID,
	}
}

func (c *Client) WithAdmin(adminID string) *Client {
	return &Client{http: c.http, token: c.token, adminID: adminID}
}

func (c *Client) Health(ctx context.Context) (Health, error) {
	var result Health
	err := c.do(ctx, http.MethodGet, "/v1/health", nil, "", false, &result)
	return result, err
}

func (c *Client) BootstrapOwner(ctx context.Context) (Owner, error) {
	var result Owner
	err := c.do(ctx, http.MethodPost, "/v1/bootstrap/owner", nil, "", false, &result)
	return result, err
}

func (c *Client) CreateTelegramPairing(ctx context.Context) (PairingToken, error) {
	var result PairingToken
	err := c.do(ctx, http.MethodPost, "/v1/auth/telegram/pairing", nil, "", true, &result)
	return result, err
}

func (c *Client) ClaimTelegramPairing(ctx context.Context, token, subject string) (Owner, error) {
	payload, err := json.Marshal(map[string]string{"token": token, "subject": subject})
	if err != nil {
		return Owner{}, err
	}
	var result Owner
	err = c.do(ctx, http.MethodPost, "/v1/auth/telegram/claim", payload, "", false, &result)
	return result, err
}

func (c *Client) ResolveTelegram(ctx context.Context, subject string) (Owner, error) {
	var result Owner
	err := c.do(ctx, http.MethodGet, "/v1/auth/telegram/resolve?subject="+url.QueryEscape(subject), nil, "", false, &result)
	return result, err
}

func (c *Client) CreateAccess(ctx context.Context, name, idempotencyKey, adminID string) (AccessCreation, error) {
	payload, err := json.Marshal(map[string]string{"name": name})
	if err != nil {
		return AccessCreation{}, err
	}
	var result AccessCreation
	err = c.WithAdmin(adminID).do(ctx, http.MethodPost, "/v1/access", payload, idempotencyKey, true, &result)
	return result, err
}

func (c *Client) BootstrapVPN(ctx context.Context, publicAddress string, target string, serverName string, port uint16, idempotencyKey string) (VPNBootstrap, error) {
	payload, err := json.Marshal(map[string]any{
		"public_address": publicAddress, "target": target, "server_name": serverName, "port": port,
	})
	if err != nil {
		return VPNBootstrap{}, err
	}
	var result VPNBootstrap
	err = c.do(ctx, http.MethodPost, "/v1/bootstrap/vpn", payload, idempotencyKey, true, &result)
	return result, err
}

func (c *Client) ListUsers(ctx context.Context) ([]User, error) {
	var result []User
	err := c.do(ctx, http.MethodGet, "/v1/users", nil, "", true, &result)
	return result, err
}

func (c *Client) CreateUser(ctx context.Context, name string, idempotencyKey string) (UserCreation, error) {
	payload, err := json.Marshal(map[string]string{"name": name})
	if err != nil {
		return UserCreation{}, err
	}
	var result UserCreation
	err = c.do(ctx, http.MethodPost, "/v1/users", payload, idempotencyKey, true, &result)
	return result, err
}

func (c *Client) do(
	ctx context.Context,
	method string,
	path string,
	payload []byte,
	idempotencyKey string,
	requireAdmin bool,
	result any,
) error {
	request, err := http.NewRequestWithContext(ctx, method, "http://unix"+path, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create control API request: %w", err)
	}
	if c.token != "" {
		request.Header.Set("Authorization", "Bearer "+c.token)
	}
	if requireAdmin {
		if c.adminID == "" {
			return errorsNew("administrator ID is not configured")
		}
		request.Header.Set("X-Admin-ID", c.adminID)
	}
	if idempotencyKey != "" {
		request.Header.Set("Idempotency-Key", idempotencyKey)
	}
	if len(payload) != 0 {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := c.http.Do(request)
	if err != nil {
		return fmt.Errorf("call control API: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		apiError := &APIError{Status: response.StatusCode, Code: "unknown", Message: http.StatusText(response.StatusCode)}
		_ = json.NewDecoder(io.LimitReader(response.Body, 64<<10)).Decode(apiError)
		return apiError
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(result); err != nil {
		return fmt.Errorf("decode control API response: %w", err)
	}
	return nil
}

func errorsNew(message string) error { return fmt.Errorf("control client: %w", errors.New(message)) }
