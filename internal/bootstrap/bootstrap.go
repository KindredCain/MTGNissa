package bootstrap

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/gin-gonic/gin"

	"mtgnissa/internal/appdata"
	"mtgnissa/internal/carddata"
	"mtgnissa/internal/cards"
	"mtgnissa/internal/config"
	"mtgnissa/internal/database"
	"mtgnissa/internal/health"
	"mtgnissa/internal/httpapi"
	"mtgnissa/internal/idempotency"
	"mtgnissa/internal/sets"
	"mtgnissa/internal/tokens"
)

// Application contains the initialized server dependencies and runtime configuration.
type Application struct {
	Router    *gin.Engine
	Databases *database.Databases
	CardData  *carddata.Manager
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
	if err := appdata.Migrate(ctx, dbs.App.DB); err != nil {
		dbs.Close()
		return nil, err
	}
	manager := carddata.NewManager(ctx, dbs.Card.DB, cfg.CardDB.Name, cfg.CardDataDir, log)
	setService := sets.NewService(sets.NewStore(dbs.Card), cfg.CursorPageSize)
	router := httpapi.New(httpapi.Dependencies{
		Logger:      log,
		Health:      health.Handler{Card: dbs.Card.DB, App: dbs.App.DB},
		Loader:      manager,
		LoadEnabled: cfg.LoadEnabled,
		App: httpapi.AppConfig{
			DisplayName: cfg.DisplayName,
			TimeZone:    cfg.TimeZone,
		},
		Sets:   setService,
		Tokens: tokens.NewService(tokens.NewStore(dbs.Card)),
		Cards:  cards.NewService(cards.NewCardStore(dbs.Card), cards.NewAppStore(dbs.App), idempotency.New(4096)),
	})
	return &Application{Router: router, Databases: dbs, CardData: manager, Config: cfg, Logger: log}, nil
}
