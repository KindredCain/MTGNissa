package bootstrap

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/gin-gonic/gin"
	"mtgnissa/internal/appdata"
	"mtgnissa/internal/carddata"
	"mtgnissa/internal/config"
	"mtgnissa/internal/database"
	"mtgnissa/internal/health"
	"mtgnissa/internal/httpapi"
)

type Application struct {
	Router    *gin.Engine
	Databases *database.Databases
	Config    config.Config
	Logger    *slog.Logger
}

func Build(ctx context.Context) (*Application, error) {
	return BuildWithConfig(ctx, "")
}

func BuildWithConfig(ctx context.Context, configPath string) (*Application, error) {
	var cfg config.Config
	var err error
	if configPath == "" {
		cfg, err = config.Load()
	} else {
		cfg, err = config.LoadFile(configPath)
	}
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	level := slog.LevelInfo
	switch cfg.LogLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	dbs, err := database.Open(ctx, cfg.CardDB, cfg.AppDB)
	if err != nil {
		return nil, err
	}
	if err := appdata.Migrate(ctx, dbs.App); err != nil {
		dbs.Close()
		return nil, err
	}
	manager := carddata.NewManager(ctx, dbs.Card, cfg.CardDB.Name, cfg.CardDataDir, log)
	router := httpapi.New(log, health.Handler{Card: dbs.Card, App: dbs.App}, manager, cfg.LoadEnabled)
	return &Application{Router: router, Databases: dbs, Config: cfg, Logger: log}, nil
}
