package store

import (
	"context"
	"time"

	"github.com/jmoiron/sqlx"

	"mtgnissa/internal/model"
)

// WishlistTx reads and changes wishlist state within the caller's transaction.
type WishlistTx struct {
	tx *sqlx.Tx
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
