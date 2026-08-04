package helper

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/steam9steam-prog/ez-vpn-lego/drivers/xray/reality"
)

type XrayValidator struct{ BinaryPath string }

func (validator XrayValidator) Validate(ctx context.Context, configurationPath string) error {
	return reality.ValidateWithBinary(ctx, validator.BinaryPath, configurationPath)
}

type SystemdService struct{ Unit string }

func (service SystemdService) Restart(ctx context.Context) error {
	if output, err := exec.CommandContext(ctx, "systemctl", "restart", service.Unit).CombinedOutput(); err != nil {
		return fmt.Errorf("restart %s: %w: %s", service.Unit, err, strings.TrimSpace(string(output)))
	}
	if output, err := exec.CommandContext(ctx, "systemctl", "is-active", "--quiet", service.Unit).CombinedOutput(); err != nil {
		return fmt.Errorf("verify %s: %w: %s", service.Unit, err, strings.TrimSpace(string(output)))
	}
	return nil
}
