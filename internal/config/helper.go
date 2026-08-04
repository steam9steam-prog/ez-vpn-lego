package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

type Helper struct {
	SocketPath         string
	Token              string
	CandidateDirectory string
	XrayBinary         string
	XrayConfiguration  string
	XrayUnit           string
}

func LoadHelper() (Helper, error) {
	configuration := Helper{
		SocketPath:         valueOrDefault("EZVPN_HELPER_SOCKET_PATH", "/run/ez-vpn-lego/helper.sock"),
		CandidateDirectory: valueOrDefault("EZVPN_CANDIDATE_DIR", "/var/lib/ez-vpn-lego/candidates"),
		XrayBinary:         valueOrDefault("EZVPN_XRAY_BINARY", "/usr/local/bin/xray"),
		XrayConfiguration:  valueOrDefault("EZVPN_XRAY_CONFIG", "/etc/xray/config.json"),
		XrayUnit:           valueOrDefault("EZVPN_XRAY_UNIT", "xray.service"),
	}
	for _, path := range []string{configuration.SocketPath, configuration.CandidateDirectory, configuration.XrayBinary, configuration.XrayConfiguration} {
		if !filepath.IsAbs(path) {
			return Helper{}, errors.New("helper filesystem paths must be absolute")
		}
	}
	tokenPath := strings.TrimSpace(os.Getenv("EZVPN_HELPER_TOKEN_FILE"))
	if tokenPath == "" {
		return Helper{}, errors.New("EZVPN_HELPER_TOKEN_FILE is required")
	}
	var err error
	configuration.Token, err = readSecret(tokenPath, "helper token")
	if err != nil {
		return Helper{}, err
	}
	if len(configuration.Token) < 32 || len(configuration.Token) > 4096 {
		return Helper{}, errors.New("helper token must contain between 32 and 4096 characters")
	}
	return configuration, nil
}

func valueOrDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}
