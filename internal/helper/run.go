package helper

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/steam9steam-prog/ez-vpn-lego/internal/config"
	"github.com/steam9steam-prog/ez-vpn-lego/internal/unixsocket"
)

func Run(ctx context.Context, configuration config.Helper, logger *slog.Logger) error {
	engine := NewEngine(
		configuration.CandidateDirectory,
		configuration.XrayConfiguration,
		XrayValidator{BinaryPath: configuration.XrayBinary},
		SystemdService{Unit: configuration.XrayUnit},
	)
	listener, err := unixsocket.Listen(configuration.SocketPath, 0o660)
	if err != nil {
		return err
	}
	defer listener.Close()
	server := &http.Server{
		Handler:           NewServer(engine, configuration.Token).Handler(),
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second,
		WriteTimeout: 45 * time.Second, IdleTimeout: 60 * time.Second,
	}
	errorsChannel := make(chan error, 1)
	go func() {
		logger.Info("privileged helper started", "socket", configuration.SocketPath)
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errorsChannel <- err
		}
		close(errorsChannel)
	}()
	select {
	case <-ctx.Done():
		shutdownContext, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownContext); err != nil {
			return fmt.Errorf("shutdown privileged helper: %w", err)
		}
		return nil
	case err := <-errorsChannel:
		if err != nil {
			return fmt.Errorf("serve privileged helper: %w", err)
		}
		return nil
	}
}
