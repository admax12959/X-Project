// Command xproject runs the application maintenance tool server.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/admax12959/X-Project/internal/app"
	"github.com/admax12959/X-Project/internal/codeplatform"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := app.Load()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// No real LDAP adapter yet (#14 adds it); nil Directory = LDAP not configured.
	a, err := app.Start(ctx, cfg, app.Deps{CodePlatforms: codeplatform.New()})
	if err != nil {
		return err
	}
	defer a.Close()

	srv := &http.Server{Addr: cfg.Listen, Handler: a.Handler, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}()
	slog.Info("listening", "addr", cfg.Listen)
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
