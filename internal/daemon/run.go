package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/steam9steam-prog/ez-vpn-lego/adapters/helperclient"
	"github.com/steam9steam-prog/ez-vpn-lego/adapters/httpapi"
	postgresadapter "github.com/steam9steam-prog/ez-vpn-lego/adapters/postgres"
	"github.com/steam9steam-prog/ez-vpn-lego/internal/config"
	"github.com/steam9steam-prog/ez-vpn-lego/internal/migrate"
	"github.com/steam9steam-prog/ez-vpn-lego/internal/secretbox"
	"github.com/steam9steam-prog/ez-vpn-lego/internal/unixsocket"
	"github.com/steam9steam-prog/ez-vpn-lego/services/access"
	"github.com/steam9steam-prog/ez-vpn-lego/services/vpnbootstrap"
)

func Run(ctx context.Context, configuration config.Daemon, logger *slog.Logger) error {
	if err := migrate.Up(ctx, configuration.DatabaseURL); err != nil {
		return err
	}
	pool, err := pgxpool.New(ctx, configuration.DatabaseURL)
	if err != nil {
		return fmt.Errorf("create PostgreSQL pool: %w", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("ping PostgreSQL: %w", err)
	}
	box, err := secretbox.New(configuration.MasterKey)
	if err != nil {
		return err
	}
	vpnService := vpnbootstrap.New(
		postgresadapter.NewVPNRepository(pool),
		helperclient.New(configuration.HelperSocket, configuration.HelperToken),
		box,
		configuration.CandidateDirectory,
	)
	helperClient := helperclient.New(configuration.HelperSocket, configuration.HelperToken)
	accessService := access.New(
		postgresadapter.NewAccessRepository(pool), helperClient, box, configuration.CandidateDirectory,
	)

	listener, err := unixsocket.Listen(configuration.SocketPath, 0o660)
	if err != nil {
		return err
	}
	defer listener.Close()

	readHeaderTimeout, readTimeout, writeTimeout, idleTimeout := httpapi.TimeoutConfig()
	api := httpapi.New(
		postgresadapter.NewUserRepository(pool),
		postgresadapter.NewAdminRepository(pool),
		vpnService,
		accessService,
		pool,
		configuration.APIToken,
	)
	server := &http.Server{
		Handler:           api.Handler(),
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}

	errorsChannel := make(chan error, 1)
	go func() {
		logger.Info("control API started", "socket", configuration.SocketPath)
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
			return fmt.Errorf("shutdown control API: %w", err)
		}
		logger.Info("control API stopped")
		return nil
	case err := <-errorsChannel:
		if err != nil {
			return fmt.Errorf("serve control API: %w", err)
		}
		return nil
	}
}
