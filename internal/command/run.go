package command

import (
	"fmt"
	"io"

	"github.com/steam9steam-prog/ez-vpn-lego/internal/buildinfo"
)

func Run(name string, args []string, stdout io.Writer) int {
	if len(args) == 1 && args[0] == "version" {
		fmt.Fprintf(stdout, "%s %s (%s, %s)\n", name, buildinfo.Version, buildinfo.Commit, buildinfo.Date)
		return 0
	}

	fmt.Fprintf(stdout, "%s: architecture bootstrap; available command: version\n", name)
	return 0
}
