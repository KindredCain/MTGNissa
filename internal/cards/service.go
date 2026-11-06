package cards

import (
	"context"
	"errors"
	"fmt"

	"mtgnissa/internal/apperror"
	"mtgnissa/internal/idempotency"
)

// Service coordinates Card DB reads and atomic personal card mutations.
type Service struct {
	cards   *CardStore
	app     *AppStore
	intents *idempotency.Cache
}

// NewService creates a service using separate Card DB and App DB stores.
func NewService(cards *CardStore, app *AppStore, intents *idempotency.Cache) *Service {
	return &Service{cards: cards, app: app, intents: intents}
}

// List returns a complete ordinary Set list in one selected printing language.
func (s *Service) List(ctx context.Context, code, language, query string) (List, error) {
	list, err := s.cards.list(ctx, code, language, query)
	return list, safeError(err)
}

// Detail returns all display, Oracle, version, and personal data for a printing.
func (s *Service) Detail(ctx context.Context, id string) (Detail, error) {
	d, err := s.cards.detail(ctx, id)
	if err != nil {
		return Detail{}, safeError(err)
	}
	if err := s.app.fillDetail(ctx, &d); err != nil {
		return Detail{}, safeError(err)
	}
	return d, nil
}

// Adjust changes exactly one printing quantity and returns current authoritative totals.
func (s *Service) Adjust(ctx context.Context, id, finish string, delta int, key string) (CollectionState, bool, error) {
	if (finish != "normal" && finish != "foil") || (delta != 1 && delta != -1) {
		return CollectionState{}, false, apperror.New(apperror.CodeValidationFailed, "invalid collection adjustment")
	}
	fingerprint := fmt.Sprintf("POST /api/v1/cards/%s/collection_adjustments|%s|%d", id, finish, delta)
	deduplicated, err := s.intents.Do(ctx, key, fingerprint, func() error {
		card, _, err := s.cards.printing(ctx, id)
		if err != nil {
			return err
		}
		return s.app.mutate(ctx, card, finish, delta, nil)
	})
	if err != nil {
		return CollectionState{}, false, safeError(err)
	}
	card, _, err := s.cards.printing(ctx, id)
	if err != nil {
		return CollectionState{}, deduplicated, safeError(err)
	}
	state, err := s.app.collectionState(ctx, card)
	return state, deduplicated, safeError(err)
}

// SetWishlist changes a printing's target wishlist state without duplicate log entries.
func (s *Service) SetWishlist(ctx context.Context, id string, included bool, key string) (Summary, bool, bool, error) {
	fingerprint := fmt.Sprintf("POST /api/v1/cards/%s/wishlist_state|%t", id, included)
	deduplicated, err := s.intents.Do(ctx, key, fingerprint, func() error {
		card, _, err := s.cards.printing(ctx, id)
		if err != nil {
			return err
		}
		return s.app.mutate(ctx, card, "", 0, &included)
	})
	if err != nil {
		return Summary{}, false, false, safeError(err)
	}
	card, _, err := s.cards.printing(ctx, id)
	if err != nil {
		return Summary{}, false, deduplicated, safeError(err)
	}
	current, err := s.app.wishlist(ctx, card.PrintingUUID)
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
