package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

type Client struct {
	SocketPath string
	APIToken   string
	AdminID    string
}

func LoadClient() (Client, error) {
	configuration := Client{SocketPath: strings.TrimSpace(os.Getenv("EZVPN_SOCKET_PATH"))}
	if configuration.SocketPath == "" {
		configuration.SocketPath = defaultSocketPath
	}
	if !filepath.IsAbs(configuration.SocketPath) {
		return Client{}, errors.New("EZVPN_SOCKET_PATH must be absolute")
	}
	tokenPath := strings.TrimSpace(os.Getenv("EZVPN_API_TOKEN_FILE"))
	if tokenPath == "" {
		return Client{}, errors.New("EZVPN_API_TOKEN_FILE is required")
	}
	var err error
	configuration.APIToken, err = readSecret(tokenPath, "API token")
	if err != nil {
		return Client{}, err
	}
	if len(configuration.APIToken) < 32 || len(configuration.APIToken) > 4096 {
		return Client{}, errors.New("API token must contain between 32 and 4096 characters")
	}
	configuration.AdminID = strings.TrimSpace(os.Getenv("EZVPN_ADMIN_ID"))
	if path := strings.TrimSpace(os.Getenv("EZVPN_ADMIN_ID_FILE")); path != "" {
		configuration.AdminID, err = readSecret(path, "administrator ID")
		if err != nil {
			return Client{}, err
		}
	}
	return configuration, nil
}
