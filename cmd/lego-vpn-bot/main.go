package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-telegram/bot"
	"github.com/steam9steam-prog/ez-vpn-lego/adapters/controlclient"
	"github.com/steam9steam-prog/ez-vpn-lego/internal/config"
	"github.com/steam9steam-prog/ez-vpn-lego/internal/telegrambot"
)

func main() {
	configuration, err := config.LoadBot()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	app := telegrambot.New(controlclient.New(configuration.Client))
	options := app.Options()
	if configuration.ServerURL != "" {
		options = append(options, bot.WithServerURL(configuration.ServerURL))
	}
	b, err := bot.New(configuration.Token, options...)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	b.Start(ctx)
}
