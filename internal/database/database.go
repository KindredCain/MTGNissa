package database

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"time"

	"github.com/go-sql-driver/mysql"
	"mtgnissa/internal/config"
)

type Databases struct{ Card, App *sql.DB }

func Open(ctx context.Context, cardCfg, appCfg config.DB) (*Databases, error) {
	card, err := open(ctx, cardCfg)
	if err != nil {
		return nil, fmt.Errorf("open card database: %w", err)
	}
	app, err := open(ctx, appCfg)
	if err != nil {
		card.Close()
		return nil, fmt.Errorf("open app database: %w", err)
	}
	return &Databases{Card: card, App: app}, nil
}

func open(ctx context.Context, c config.DB) (*sql.DB, error) {
	mysqlConfig := mysql.Config{
		User:      c.User,
		Passwd:    c.Password,
		Net:       "tcp",
		Addr:      net.JoinHostPort(c.Host, fmt.Sprint(c.Port)),
		DBName:    c.Name,
		ParseTime: true,
		Loc:       time.Local,
		Collation: "utf8mb4_unicode_ci",
	}
	dsn := mysqlConfig.FormatDSN()
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(c.MaxOpen)
	db.SetMaxIdleConns(c.MaxIdle)
	db.SetConnMaxIdleTime(c.MaxIdleTime)
	db.SetConnMaxLifetime(c.MaxLifetime)
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err = db.PingContext(pingCtx); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func (d *Databases) Close() error {
	var first error
	if d.Card != nil {
		first = d.Card.Close()
	}
	if d.App != nil {
		if err := d.App.Close(); first == nil {
			first = err
		}
	}
	return first
}
