package reality

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestOfficialXrayAcceptsRenderedConfiguration(t *testing.T) {
	binary := os.Getenv("XRAY_BINARY")
	if binary == "" {
		t.Skip("XRAY_BINARY is not configured")
	}
	keys, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	configuration, err := Render(State{Settings: Settings{
		Listen: "127.0.0.1", Port: 18443, Target: "example.com:443",
		ServerNames: []string{"example.com"}, PrivateKey: keys.Private,
	}, Credentials: []Credential{{
		ID: "integration-device", UUID: "00000000-0000-4000-8000-000000000001",
		ShortID: "0123456789abcdef", Label: "Integration device",
	}}})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "xray.json")
	if err := os.WriteFile(path, configuration, 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := ValidateWithBinary(ctx, binary, path); err != nil {
		t.Fatal(err)
	}
}
