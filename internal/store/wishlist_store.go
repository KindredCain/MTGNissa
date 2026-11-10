package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/jmoiron/sqlx"

	"mtgnissa/internal/model"
)

// WishlistStore reads wishlist identities and matching metadata from the two databases.
type WishlistStore struct {
	app   *sqlx.DB
	cards *CardStore
}

// WishlistTx reads and changes wishlist state within the caller's transaction.
type WishlistTx struct {
	tx *sqlx.Tx
}

// NewWishlistStore creates a wishlist reader with separate App DB and Card DB access.
func NewWishlistStore(app, card *sqlx.DB) *WishlistStore {
	return &WishlistStore{app: app, cards: NewCardStore(card)}
}

// Options resolves only current wishlist series and language codes through Card DB.
func (s *WishlistStore) Options(ctx context.Context) ([]model.SetRef, []model.Language, error) {
	return personalOptions(ctx, s.app, s.cards, "wishlist_item")
}

// List filters and pages saved query metadata in one application snapshot.
func (s *WishlistStore) List(ctx context.Context, params model.WishlistParams, position *PersonalPosition, pageSize int) (model.WishlistPage, *PersonalPosition, error) {
	page := model.WishlistPage{Items: []model.WishlistItem{}}
	tx, err := s.app.BeginTxx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return page, nil, err
	}
	defer tx.Rollback()
	filters := model.CollectionParams{Query: params.Query, Sets: params.Sets, Languages: params.Languages, Colors: params.Colors, ManaValues: params.ManaValues, Rarities: params.Rarities}
	base := personalFilters(personalBase("wishlist_item"), filters)
	page.Total, err = personalCount(ctx, tx, base)
	if err != nil {
		return page, nil, err
	}
	rows, next, err := readPersonalPage(ctx, tx, personalPage(personalColumns(base), params.Sort, position, pageSize), pageSize)
	if err != nil {
		return page, nil, err
	}
	for _, row := range rows {
		page.Items = append(page.Items, model.WishlistItem{Card: model.CardSummary{PrintingUUID: row.UUID}, Included: true})
	}
	if err := tx.Commit(); err != nil {
		return page, nil, err
	}
	return page, next, nil
}

// Summaries resolves complete summaries only for the current wishlist page.
func (s *WishlistStore) Summaries(ctx context.Context, ids []string) (map[string]model.CardSummary, error) {
	return s.cards.Summaries(ctx, ids)
}

// Lock ensures the reference exists and reads wishlist state under its mutation lock.
func (s *WishlistTx) Lock(ctx context.Context, card model.CardSummary, now time.Time) (bool, error) {
	if err := lockPrintingRef(ctx, s.tx, card, now); err != nil {
		return false, err
	}
	return s.State(ctx, card.PrintingUUID)
}

// SetState persists a target state after the service has locked and compared it.
func (s *WishlistTx) SetState(ctx context.Context, id string, included bool, now time.Time) error {
	if included {
		_, err := s.tx.ExecContext(ctx, "INSERT INTO wishlist_item (printing_uuid,created_at) VALUES (?,?)", id, now)
		return err
	}
	_, err := s.tx.ExecContext(ctx, "DELETE FROM wishlist_item WHERE printing_uuid=?", id)
	return err
}

// State returns the current state of one precise printing in the active snapshot.
func (s *WishlistTx) State(ctx context.Context, id string) (bool, error) {
	var count int
	err := s.tx.GetContext(ctx, &count, "SELECT COUNT(*) FROM wishlist_item WHERE printing_uuid=?", id)
	return count > 0, err
}
