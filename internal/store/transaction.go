package store

import (
	"context"
	"database/sql"

	"github.com/jmoiron/sqlx"
)

// AppTransactions runs service-defined operations in one application database transaction.
type AppTransactions struct {
	db *sqlx.DB
}

// AppTx exposes business stores bound to the same transaction, without commit methods.
type AppTx struct {
	tx            *sqlx.Tx
	Collection    *CollectionTx
	Wishlist      *WishlistTx
	OperationLogs *OperationLogTx
	Decks         *DeckTx
}

// CardTx provides Card DB queries bound to one read-only snapshot.
type CardTx struct {
	tx *sqlx.Tx
}

// NewAppTransactions creates the transaction entry point for the application database.
func NewAppTransactions(db *sqlx.DB) *AppTransactions {
	return &AppTransactions{db: db}
}

// WithAppTx commits all callback operations together, or rolls them back on failure.
func (s *AppTransactions) WithAppTx(ctx context.Context, fn func(*AppTx) error) error {
	return s.withTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted}, fn)
}

// WithAppRead provides one repeatable-read snapshot for service-defined query combinations.
func (s *AppTransactions) WithAppRead(ctx context.Context, fn func(*AppTx) error) error {
	return s.withTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true}, fn)
}

// WithCardRead commits service-defined reads against one repeatable-read snapshot.
func (s *CardStore) WithCardRead(ctx context.Context, fn func(*CardTx) error) error {
	tx, err := s.db.BeginTxx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := fn(&CardTx{tx: tx}); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *AppTransactions) withTx(ctx context.Context, options *sql.TxOptions, fn func(*AppTx) error) error {
	tx, err := s.db.BeginTxx(ctx, options)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stores := &AppTx{tx: tx, Collection: &CollectionTx{tx: tx}, Wishlist: &WishlistTx{tx: tx},
		OperationLogs: &OperationLogTx{tx: tx}, Decks: &DeckTx{tx: tx}}
	if err := fn(stores); err != nil {
		return err
	}
	return tx.Commit()
}
