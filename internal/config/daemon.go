package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/steam9steam-prog/ez-vpn-lego/internal/secretbox"
)

const defaultSocketPath = "/run/ez-vpn-lego/control.sock"

type Daemon struct {
	DatabaseURL        string
	SocketPath         string
	APIToken           string
	MasterKey          []byte
	HelperSocket       string
	HelperToken        string
	CandidateDirectory string
}

func LoadDaemon() (Daemon, error) {
	configuration := Daemon{
		DatabaseURL:        strings.TrimSpace(os.Getenv("EZVPN_DATABASE_URL")),
		SocketPath:         strings.TrimSpace(os.Getenv("EZVPN_SOCKET_PATH")),
		HelperSocket:       valueOrDefault("EZVPN_HELPER_SOCKET_PATH", "/run/ez-vpn-lego/helper.sock"),
		CandidateDirectory: valueOrDefault("EZVPN_CANDIDATE_DIR", "/var/lib/ez-vpn-lego/candidates"),
	}
	if configuration.DatabaseURL == "" {
		return Daemon{}, errors.New("EZVPN_DATABASE_URL is required")
	}
	if configuration.SocketPath == "" {
		configuration.SocketPath = defaultSocketPath
	}
	if !filepath.IsAbs(configuration.SocketPath) {
		return Daemon{}, errors.New("EZVPN_SOCKET_PATH must be absolute")
	}
	if !filepath.IsAbs(configuration.HelperSocket) || !filepath.IsAbs(configuration.CandidateDirectory) {
		return Daemon{}, errors.New("helper socket and candidate directory must be absolute")
	}

	tokenPath := strings.TrimSpace(os.Getenv("EZVPN_API_TOKEN_FILE"))
	if tokenPath == "" {
		return Daemon{}, errors.New("EZVPN_API_TOKEN_FILE is required")
	}
	apiToken, err := readSecret(tokenPath, "API token")
	if err != nil {
		return Daemon{}, err
	}
	configuration.APIToken = apiToken
	if len(configuration.APIToken) < 32 || len(configuration.APIToken) > 4096 {
		return Daemon{}, errors.New("API token must contain between 32 and 4096 characters")
	}
	masterKeyPath := strings.TrimSpace(os.Getenv("EZVPN_MASTER_KEY_FILE"))
	if masterKeyPath == "" {
		return Daemon{}, errors.New("EZVPN_MASTER_KEY_FILE is required")
	}
	encodedMasterKey, err := readSecret(masterKeyPath, "master key")
	if err != nil {
		return Daemon{}, err
	}
	configuration.MasterKey, err = secretbox.ParseKey(encodedMasterKey)
	if err != nil {
		return Daemon{}, err
	}
	helperTokenPath := strings.TrimSpace(os.Getenv("EZVPN_HELPER_TOKEN_FILE"))
	if helperTokenPath == "" {
		return Daemon{}, errors.New("EZVPN_HELPER_TOKEN_FILE is required")
	}
	configuration.HelperToken, err = readSecret(helperTokenPath, "helper token")
	if err != nil {
		return Daemon{}, err
	}
	if len(configuration.HelperToken) < 32 || len(configuration.HelperToken) > 4096 {
		return Daemon{}, errors.New("helper token must contain between 32 and 4096 characters")
	}
	return configuration, nil
}

func readSecret(path string, label string) (string, error) {
	value, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s file: %w", label, err)
	}
	return strings.TrimSpace(string(value)), nil
}
