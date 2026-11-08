package cards

import (
	"context"
	"errors"
	"fmt"

	"mtgnissa/internal/apperror"
	"mtgnissa/internal/idempotency"
	"mtgnissa/internal/model"
	"mtgnissa/internal/store"
)

// Service coordinates card queries and service-defined personal card transactions.
type Service struct {
	cards   *store.CardStore
	app     *store.AppTransactions
	intents *idempotency.Cache
}

// NewService creates a service using separate Card DB and App DB stores.
func NewService(cards *store.CardStore, app *store.AppTransactions, intents *idempotency.Cache) *Service {
	return &Service{cards: cards, app: app, intents: intents}
}

// List returns a complete ordinary Set list in one selected printing language.
func (s *Service) List(ctx context.Context, code, language, query string) (model.CardList, error) {
	list, err := s.cards.List(ctx, code, language, query)
	return list, safeError(err)
}

// Detail returns all display, Oracle, version, and personal data for a printing.
func (s *Service) Detail(ctx context.Context, id string) (model.CardDetail, error) {
	d, err := s.cards.Detail(ctx, id)
	if err != nil {
		return model.CardDetail{}, safeError(err)
	}
	if err := s.app.WithAppRead(ctx, func(tx *store.TxStores) error {
		return store.FillCardContext(ctx, tx, &d)
	}); err != nil {
		return model.CardDetail{}, safeError(err)
	}
	return d, nil
}

// Adjust changes exactly one printing quantity and returns current authoritative totals.
func (s *Service) Adjust(ctx context.Context, id, finish string, delta int, key string) (model.CollectionState, bool, error) {
	if (finish != "normal" && finish != "foil") || (delta != 1 && delta != -1) {
		return model.CollectionState{}, false, apperror.New(apperror.CodeValidationFailed, "invalid collection adjustment")
	}
	fingerprint := fmt.Sprintf("POST /api/v1/cards/%s/collection_adjustments|%s|%d", id, finish, delta)
	deduplicated, err := s.intents.Do(ctx, key, fingerprint, func() error {
		card, _, err := s.cards.Printing(ctx, id)
		if err != nil {
			return err
		}
		return s.app.WithAppTx(ctx, func(tx *store.TxStores) error {
			change, err := tx.Collection.Adjust(ctx, card, finish, delta)
			if err != nil {
				return err
			}
			return tx.OperationLogs.AppendCard(ctx, change)
		})
	})
	if err != nil {
		return model.CollectionState{}, false, safeError(err)
	}
	card, _, err := s.cards.Printing(ctx, id)
	if err != nil {
		return model.CollectionState{}, deduplicated, safeError(err)
	}
	var state model.CollectionState
	err = s.app.WithAppRead(ctx, func(tx *store.TxStores) error {
		var err error
		state, err = tx.Collection.State(ctx, card)
		return err
	})
	return state, deduplicated, safeError(err)
}

// SetWishlist changes a printing's target wishlist state without duplicate log entries.
func (s *Service) SetWishlist(ctx context.Context, id string, included bool, key string) (model.CardSummary, bool, bool, error) {
	fingerprint := fmt.Sprintf("POST /api/v1/cards/%s/wishlist_state|%t", id, included)
	deduplicated, err := s.intents.Do(ctx, key, fingerprint, func() error {
		card, _, err := s.cards.Printing(ctx, id)
		if err != nil {
			return err
		}
		return s.app.WithAppTx(ctx, func(tx *store.TxStores) error {
			change, changed, err := tx.Wishlist.SetState(ctx, card, included)
			if err != nil || !changed {
				return err
			}
			return tx.OperationLogs.AppendCard(ctx, change)
		})
	})
	if err != nil {
		return model.CardSummary{}, false, false, safeError(err)
	}
	card, _, err := s.cards.Printing(ctx, id)
	if err != nil {
		return model.CardSummary{}, false, deduplicated, safeError(err)
	}
	var current bool
	err = s.app.WithAppRead(ctx, func(tx *store.TxStores) error {
		var err error
		current, err = tx.Wishlist.State(ctx, card.PrintingUUID)
		return err
	})
	return card, current, deduplicated, safeError(err)
}

func safeError(err error) error {
	if err == nil {
		return nil
	}
	var appErr *apperror.Error
	if errors.As(err, &appErr) {
		return err
	}
	return apperror.Wrap(apperror.CodeDependencyUnavailable, "card or application data is temporarily unavailable", err)
}
