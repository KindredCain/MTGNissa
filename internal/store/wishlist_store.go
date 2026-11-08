package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"

	"mtgnissa/internal/model"
)

// WishlistStore reads wishlist identities and matching metadata from the two databases.
type WishlistStore struct {
	app   *sqlx.DB
	cards *CardStore
}

type wishlistRefRow struct {
	UUID   string  `db:"printing_uuid"`
	Oracle *string `db:"oracle_id"`
	Set    string  `db:"set_code"`
	Number string  `db:"collector_number"`
	Lang   string  `db:"lang"`
}

// WishlistTx reads and changes wishlist state within the caller's transaction.
type WishlistTx struct {
	tx *sqlx.Tx
}

// NewWishlistStore creates a wishlist reader with separate App DB and Card DB access.
func NewWishlistStore(app, card *sqlx.DB) *WishlistStore {
	return &WishlistStore{app: app, cards: NewCardStore(card)}
}

// Load reads only current wishlist printings and verifies their recorded identities.
func (s *WishlistStore) Load(ctx context.Context) ([]model.CardSummary, error) {
	tx, err := s.app.BeginTxx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var refs []wishlistRefRow
	if err := tx.SelectContext(ctx, &refs, `SELECT w.printing_uuid,r.oracle_id,r.set_code,r.collector_number,r.lang
 FROM wishlist_item w LEFT JOIN card_print_ref r ON r.printing_uuid=w.printing_uuid
 ORDER BY w.printing_uuid`); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(refs))
	for _, ref := range refs {
		ids = append(ids, ref.UUID)
	}
	metadata, err := s.cards.metadata(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]model.CardSummary, 0, len(refs))
	for _, ref := range refs {
		meta, ok := metadata[ref.UUID]
		if !ok {
			return nil, fmt.Errorf("wishlist printing %s is unavailable in card data", ref.UUID)
		}
		if !sameOracle(ref.Oracle, meta.Oracle) || ref.Set != meta.SetCode || ref.Number != meta.Number || ref.Lang != meta.LanguageCode {
			return nil, fmt.Errorf("wishlist printing %s identity does not match card data", ref.UUID)
		}
		result = append(result, meta.summary())
	}
	return result, nil
}

// Summaries resolves complete summaries only for the current wishlist page.
func (s *WishlistStore) Summaries(ctx context.Context, ids []string) (map[string]model.CardSummary, error) {
	return s.cards.Summaries(ctx, ids)
}

// SetState changes the target state and reports whether a log event is needed.
func (s *WishlistTx) SetState(ctx context.Context, card model.CardSummary, included bool) (model.CardChange, bool, error) {
	now := time.Now().UTC()
	if err := lockPrintingRef(ctx, s.tx, card, now); err != nil {
		return model.CardChange{}, false, err
	}
	current, err := s.State(ctx, card.PrintingUUID)
	if err != nil {
		return model.CardChange{}, false, err
	}
	if current == included {
		return model.CardChange{}, false, nil
	}
	if included {
		_, err = s.tx.ExecContext(ctx, "INSERT INTO wishlist_item (printing_uuid,created_at) VALUES (?,?)", card.PrintingUUID, now)
	} else {
		_, err = s.tx.ExecContext(ctx, "DELETE FROM wishlist_item WHERE printing_uuid=?", card.PrintingUUID)
	}
	if err != nil {
		return model.CardChange{}, false, err
	}
	return model.CardChange{Card: card, Included: &included, OccurredAt: now}, true, nil
}

// State returns the current state of one precise printing in the active snapshot.
func (s *WishlistTx) State(ctx context.Context, id string) (bool, error) {
	var count int
	err := s.tx.GetContext(ctx, &count, "SELECT COUNT(*) FROM wishlist_item WHERE printing_uuid=?", id)
	return count > 0, err
}
