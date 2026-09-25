package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"mtgnissa/internal/bootstrap"
)

func main() {
	configPath := flag.String("config", "", "path to YAML configuration (or use CONFIG_FILE)")
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	app, err := bootstrap.BuildWithConfig(ctx, *configPath)
	if err != nil {
		slog.Error("application startup failed", "error", err)
		os.Exit(1)
	}
	server := &http.Server{Addr: app.Config.HTTPAddr, Handler: app.Router, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	errCh := make(chan error, 1)
	go func() {
		app.Logger.Info("http server started", "addr", app.Config.HTTPAddr)
		errCh <- server.ListenAndServe()
	}()
	select {
	case <-ctx.Done():
	case err = <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			app.Logger.Error("http server failed", "error", err)
		}
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), app.Config.ShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		app.Logger.Error("http shutdown failed", "error", err)
	}
	if err := app.Databases.Close(); err != nil {
		app.Logger.Error("database close failed", "error", err)
	}
}
