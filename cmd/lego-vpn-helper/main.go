package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/steam9steam-prog/ez-vpn-lego/internal/buildinfo"
	"github.com/steam9steam-prog/ez-vpn-lego/internal/command"
	"github.com/steam9steam-prog/ez-vpn-lego/internal/config"
	"github.com/steam9steam-prog/ez-vpn-lego/internal/helper"
)

func main() {
	if len(os.Args) == 2 && os.Args[1] == "version" {
		os.Exit(command.Run("lego-vpn-helper", os.Args[1:], os.Stdout))
	}
	if len(os.Args) > 2 || (len(os.Args) == 2 && os.Args[1] != "serve") {
		fmt.Fprintln(os.Stderr, "usage: lego-vpn-helper [serve|version]")
		os.Exit(2)
	}
	configuration, err := config.LoadHelper()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("service", "lego-vpn-helper", "version", buildinfo.Version)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := helper.Run(ctx, configuration, logger); err != nil {
		logger.Error("helper stopped with error", "error", err)
		os.Exit(1)
	}
}
