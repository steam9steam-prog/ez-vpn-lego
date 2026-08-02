package main

import (
	"os"

	"github.com/steam9steam-prog/ez-vpn-lego/internal/command"
)

func main() {
	os.Exit(command.Run("lego-vpnctl", os.Args[1:], os.Stdout))
}
