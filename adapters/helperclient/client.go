package helperclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/steam9steam-prog/ez-vpn-lego/internal/helper"
)

type Client struct {
	http  *http.Client
	token string
}

func New(socketPath string, token string) *Client {
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", socketPath)
	}}
	return &Client{http: &http.Client{Transport: transport, Timeout: 45 * time.Second}, token: token}
}

func (client *Client) ApplyXray(ctx context.Context, revisionID string, checksum string) error {
	payload, err := json.Marshal(helper.ApplyRequest{RevisionID: revisionID, SHA256: checksum})
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://unix/v1/xray/apply", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+client.token)
	request.Header.Set("Content-Type", "application/json")
	response, err := client.http.Do(request)
	if err != nil {
		return fmt.Errorf("call privileged helper: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 64<<10))
		return fmt.Errorf("privileged helper rejected apply (HTTP %d): %s", response.StatusCode, bytes.TrimSpace(body))
	}
	return nil
}
