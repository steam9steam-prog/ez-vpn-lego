package config

import (
	"errors"
	"os"
	"strings"
)

type Bot struct {
	Token     string
	ServerURL string
	Client    Client
}

func LoadBot() (Bot, error) {
	path := strings.TrimSpace(os.Getenv("EZVPN_TELEGRAM_TOKEN_FILE"))
	if path == "" {
		return Bot{}, errors.New("EZVPN_TELEGRAM_TOKEN_FILE is required")
	}
	token, err := readSecret(path, "Telegram bot token")
	if err != nil {
		return Bot{}, err
	}
	if len(token) < 20 || len(token) > 256 {
		return Bot{}, errors.New("Telegram bot token has an invalid length")
	}
	client, err := LoadClient()
	if err != nil {
		return Bot{}, err
	}
	return Bot{Token: token, ServerURL: strings.TrimRight(strings.TrimSpace(os.Getenv("EZVPN_TELEGRAM_API_URL")), "/"), Client: client}, nil
}
