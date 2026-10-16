// Command xproject runs the application maintenance tool server.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"slices"
	"strconv"
	"syscall"
	"time"

	"github.com/admax12959/X-Project/internal/app"
	"github.com/admax12959/X-Project/internal/cli"
	"github.com/admax12959/X-Project/internal/codeplatform"
	"github.com/admax12959/X-Project/internal/ldapdir"
	"github.com/admax12959/X-Project/internal/mailer"
	"github.com/admax12959/X-Project/internal/objectstore"
)

func main() {
	if err := run(); err != nil {
		var exit errExit
		if errors.As(err, &exit) {
			os.Exit(int(exit))
		}
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := app.Load()
	if err != nil {
		return err
	}
	if len(os.Args) > 1 {
		return runCommand(cfg, os.Args[1:])
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	a, err := app.Start(ctx, cfg, app.Deps{Directories: ldapdir.Factory{}, CodePlatforms: codeplatform.New(), MailSenders: mailer.Factory{}, ObjectStores: objectstore.Factory{}})
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

// errExit carries the exit status of a command line command; run's caller maps it to os.Exit
// without logging it as a fatal error.
type errExit int

func (e errExit) Error() string { return "exit " + strconv.Itoa(int(e)) }

// runCommand runs a command of the container's command line (internal/cli) instead of the server.
func runCommand(cfg app.Config, args []string) error {
	if !slices.Contains(cli.Commands, args[0]) {
		fmt.Fprintln(os.Stderr, cli.Usage)
		return errExit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	svc, closeDB, err := app.OpenService(ctx, cfg)
	if err != nil {
		return err
	}
	defer closeDB()
	if code := cli.Run(ctx, svc, args, os.Stdout, os.Stderr); code != 0 {
		return errExit(code)
	}
	return nil
}
