package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/steam9steam-prog/ez-vpn-lego/adapters/controlclient"
	"github.com/steam9steam-prog/ez-vpn-lego/internal/command"
	"github.com/steam9steam-prog/ez-vpn-lego/internal/config"
	"github.com/steam9steam-prog/ez-vpn-lego/internal/id"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(arguments []string) int {
	if len(arguments) == 1 && arguments[0] == "version" {
		return command.Run("lego-vpnctl", arguments, os.Stdout)
	}
	if len(arguments) == 0 {
		usage()
		return 2
	}
	configuration, err := config.LoadClient()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	client := controlclient.New(configuration)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	switch {
	case len(arguments) == 1 && arguments[0] == "status":
		result, err := client.Health(ctx)
		return printResult(result, err)
	case len(arguments) == 2 && arguments[0] == "owner" && arguments[1] == "bootstrap":
		result, err := client.BootstrapOwner(ctx)
		return printResult(result, err)
	case len(arguments) == 3 && arguments[0] == "telegram" && arguments[1] == "pairing":
		result, err := client.CreateTelegramPairing(ctx)
		if err != nil {
			return printResult(result, err)
		}
		username := strings.TrimPrefix(strings.TrimSpace(arguments[2]), "@")
		return printResult(struct {
			URL       string    `json:"url"`
			ExpiresAt time.Time `json:"expires_at"`
		}{
			URL: "https://t.me/" + username + "?start=" + result.Token, ExpiresAt: result.ExpiresAt,
		}, nil)
	case len(arguments) == 5 && arguments[0] == "vpn" && arguments[1] == "bootstrap":
		key, err := id.NewUUID()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		result, err := client.BootstrapVPN(ctx, arguments[2], arguments[3], arguments[4], 443, key)
		return printResult(result, err)
	case len(arguments) == 2 && arguments[0] == "users" && arguments[1] == "list":
		result, err := client.ListUsers(ctx)
		return printResult(result, err)
	case len(arguments) >= 3 && arguments[0] == "users" && arguments[1] == "create":
		name := strings.TrimSpace(strings.Join(arguments[2:], " "))
		key, err := id.NewUUID()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		result, err := client.CreateUser(ctx, name, key)
		return printResult(result, err)
	default:
		usage()
		return 2
	}
}

func printResult(result any, err error) int {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(result); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func usage() {
	fmt.Fprintln(os.Stderr, `usage:
  lego-vpnctl status
  lego-vpnctl owner bootstrap
  lego-vpnctl telegram pairing BOT_USERNAME
  lego-vpnctl vpn bootstrap PUBLIC_ADDRESS TARGET SERVER_NAME
  lego-vpnctl users list
  lego-vpnctl users create NAME
  lego-vpnctl version`)
}
