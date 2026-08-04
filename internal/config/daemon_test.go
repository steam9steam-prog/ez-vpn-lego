package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadDaemon(t *testing.T) {
	directory := t.TempDir()
	tokenPath := filepath.Join(directory, "api.token")
	if err := os.WriteFile(tokenPath, []byte(strings.Repeat("s", 32)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	masterKeyPath := filepath.Join(directory, "master.key")
	if err := os.WriteFile(masterKeyPath, []byte("MDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDA"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EZVPN_DATABASE_URL", "postgres://local/test")
	t.Setenv("EZVPN_SOCKET_PATH", filepath.Join(directory, "control.sock"))
	t.Setenv("EZVPN_API_TOKEN_FILE", tokenPath)
	t.Setenv("EZVPN_MASTER_KEY_FILE", masterKeyPath)
	t.Setenv("EZVPN_HELPER_TOKEN_FILE", tokenPath)

	configuration, err := LoadDaemon()
	if err != nil {
		t.Fatal(err)
	}
	if len(configuration.APIToken) != 32 {
		t.Fatalf("token length: got %d", len(configuration.APIToken))
	}
	if len(configuration.MasterKey) != 32 {
		t.Fatalf("master key length: got %d", len(configuration.MasterKey))
	}
}
