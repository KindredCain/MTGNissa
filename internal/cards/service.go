package cards

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

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
	data, err := s.cards.Detail(ctx, id)
	if err != nil {
		return model.CardDetail{}, safeError(err)
	}
	d := buildDetail(data)
	if err := s.app.WithAppRead(ctx, func(tx *store.AppTx) error {
		return fillCardContext(ctx, tx, &d)
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
		return s.app.WithAppTx(ctx, func(tx *store.AppTx) error {
			now := time.Now().UTC()
			quantity, err := tx.Collection.Lock(ctx, card, now)
			if err != nil {
				return err
			}
			quantity, err = adjustQuantity(quantity, finish, delta)
			if err != nil {
				return err
			}
			if err := tx.Collection.SetQuantity(ctx, card.PrintingUUID, quantity, now); err != nil {
				return err
			}
			change := model.CardChange{Card: card, Finish: finish, Delta: delta, OccurredAt: now}
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
	err = s.app.WithAppRead(ctx, func(tx *store.AppTx) error {
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
		return s.app.WithAppTx(ctx, func(tx *store.AppTx) error {
			now := time.Now().UTC()
			current, err := tx.Wishlist.Lock(ctx, card, now)
			if err != nil || current == included {
				return err
			}
			if err := tx.Wishlist.SetState(ctx, card.PrintingUUID, included, now); err != nil {
				return err
			}
			change := model.CardChange{Card: card, Included: &included, OccurredAt: now}
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
	err = s.app.WithAppRead(ctx, func(tx *store.AppTx) error {
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

func adjustQuantity(quantity model.Quantity, finish string, delta int) (model.Quantity, error) {
	if (finish != "normal" && finish != "foil") || (delta != 1 && delta != -1) {
		return model.Quantity{}, apperror.New(apperror.CodeValidationFailed, "invalid collection adjustment")
	}
	value := &quantity.Normal
	if finish == "foil" {
		value = &quantity.Foil
	}
	if delta < 0 {
		if *value == 0 {
			return model.Quantity{}, apperror.New(apperror.CodeCollectionUnderflow, "collection quantity cannot be negative")
		}
		*value--
	} else {
		if *value >= math.MaxUint32 {
			return model.Quantity{}, apperror.New(apperror.CodeValidationFailed, "collection quantity is at its maximum")
		}
		*value++
	}
	return quantity, nil
}

func buildDetail(data model.CardDetailData) model.CardDetail {
	card, faces := data.Card, data.Faces
	d := model.CardDetail{Card: card, Faces: faces, Rulings: data.Rulings, Tags: []model.OracleTag{}, Languages: []model.CardLanguageOption{}, OtherVersions: []model.CardVersion{}, Decks: []model.DeckUsage{}}
	versions := data.Versions
	groups := make(map[[2]string]*model.CardVersion)
	for _, row := range versions {
		key := [2]string{row.Set.Code, row.CollectorNumber}
		option := model.CardLanguageOption{PrintingUUID: row.PrintingUUID, OracleID: row.OracleID, Language: row.Language}
		if key == [2]string{card.Set.Code, card.CollectorNumber} {
			d.Languages = append(d.Languages, option)
			continue
		}
		v := groups[key]
		if v == nil {
			v = &model.CardVersion{Set: row.Set, CollectorNumber: row.CollectorNumber, ReleasedAt: row.ReleasedAt, Languages: []model.CardLanguageOption{}}
			groups[key] = v
		}
		v.Languages = append(v.Languages, option)
	}
	sortLanguages(d.Languages)
	for _, v := range groups {
		sortLanguages(v.Languages)
		d.OtherVersions = append(d.OtherVersions, *v)
	}
	sort.Slice(d.OtherVersions, func(i, j int) bool {
		a, b := d.OtherVersions[i], d.OtherVersions[j]
		if a.ReleasedAt == nil && b.ReleasedAt != nil {
			return false
		}
		if a.ReleasedAt != nil && b.ReleasedAt == nil {
			return true
		}
		if a.ReleasedAt != nil && !a.ReleasedAt.Equal(*b.ReleasedAt) {
			return a.ReleasedAt.After(*b.ReleasedAt)
		}
		if a.Set.Code != b.Set.Code {
			return a.Set.Code < b.Set.Code
		}
		if a.CollectorNumber != b.CollectorNumber {
			return a.CollectorNumber < b.CollectorNumber
		}
		return minPrinting(a.Languages) < minPrinting(b.Languages)
	})
	for _, tag := range data.Tags {
		label := strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(tag.Label)
		d.Tags = append(d.Tags, model.OracleTag{ID: tag.ID, Query: `otag:"` + label + `"`})
	}
	return d
}

func sortLanguages(options []model.CardLanguageOption) {
	sort.Slice(options, func(i, j int) bool {
		if options[i].Language.SortOrder != options[j].Language.SortOrder {
			return options[i].Language.SortOrder < options[j].Language.SortOrder
		}
		if options[i].Language.Code != options[j].Language.Code {
			return options[i].Language.Code < options[j].Language.Code
		}
		return options[i].PrintingUUID < options[j].PrintingUUID
	})
}

func minPrinting(options []model.CardLanguageOption) string {
	minimum := options[0].PrintingUUID
	for _, option := range options[1:] {
		if option.PrintingUUID < minimum {
			minimum = option.PrintingUUID
		}
	}
	return minimum
}

func fillCardContext(ctx context.Context, stores *store.AppTx, detail *model.CardDetail) error {
	state, err := stores.Collection.CardPersonalState(ctx, detail.Card.PrintingUUID, detail.Card.OracleID)
	if err != nil {
		return err
	}
	detail.Collection, detail.LogicalCollection, detail.Included = state.Collection, state.LogicalCollection, state.Included
	ids := make([]string, 0)
	for _, option := range detail.Languages {
		ids = append(ids, option.PrintingUUID)
	}
	for _, version := range detail.OtherVersions {
		for _, option := range version.Languages {
			ids = append(ids, option.PrintingUUID)
		}
	}
	owned, err := stores.Collection.Quantities(ctx, ids)
	if err != nil {
		return err
	}
	for i := range detail.Languages {
		detail.Languages[i].Quantity = owned[detail.Languages[i].PrintingUUID]
	}
	for i := range detail.OtherVersions {
		version := &detail.OtherVersions[i]
		version.Quantity = model.Quantity{}
		for j := range version.Languages {
			quantity := owned[version.Languages[j].PrintingUUID]
			version.Languages[j].Quantity = quantity
			version.Quantity.Normal += quantity.Normal
			version.Quantity.Foil += quantity.Foil
		}
	}
	detail.Decks, err = stores.Decks.Usage(ctx, detail.Card.OracleID)
	return err
}
