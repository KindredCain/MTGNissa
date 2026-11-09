package wishlist

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math"
	"sort"
	"strings"
	"time"

	"mtgnissa/internal/apperror"
	"mtgnissa/internal/model"
	"mtgnissa/internal/store"
)

type wishlistStore interface {
	Options(context.Context) ([]model.SetRef, []model.Language, error)
	List(context.Context, model.WishlistParams, *store.PersonalPosition, int) (model.WishlistPage, *store.PersonalPosition, error)
	Summaries(context.Context, []string) (map[string]model.CardSummary, error)
}

// Service coordinates precise wishlist filters, ordering, and stable cursor pages.
type Service struct {
	store    wishlistStore
	pageSize int
}

type sortPosition struct {
	SetDate string  `json:"date"`
	SetCode string  `json:"set"`
	Number  string  `json:"number"`
	Color   int     `json:"color"`
	Mana    float64 `json:"mana"`
	Name    string  `json:"name"`
	UUID    string  `json:"id"`
}

type cursorPayload struct {
	Version     int          `json:"v"`
	Fingerprint string       `json:"f"`
	Sort        string       `json:"s"`
	Position    sortPosition `json:"p"`
}

// NewService creates a wishlist service with the configured cursor batch size.
func NewService(store *store.WishlistStore, pageSize int) *Service {
	return &Service{store: store, pageSize: pageSize}
}

// Options returns Card DB metadata for the series and languages present in personal data.
func (s *Service) Options(ctx context.Context) (model.WishlistOptions, error) {
	sets, languages, err := s.store.Options(ctx)
	if err != nil {
		return model.WishlistOptions{}, dependencyError(err)
	}
	return model.WishlistOptions{Sets: append([]model.SetRef{}, sets...), Languages: append([]model.Language{}, languages...),
		Colors:     []string{"W", "U", "B", "R", "G", "multicolor", "colorless"},
		ManaValues: []string{"0", "1", "2", "3", "4", "5", "6", "7", "8_plus"},
		Rarities:   []string{"common", "uncommon", "rare", "mythic", "special", "bonus"}}, nil
}

// List pages application query metadata and hydrates only the current page from Card DB.
func (s *Service) List(ctx context.Context, params model.WishlistParams) (model.WishlistPage, error) {
	params = normalizeParams(params)
	fingerprint := queryFingerprint(params)
	var position *store.PersonalPosition
	if params.Cursor != "" {
		decoded, err := decodeCursor(params.Cursor, params.Sort, fingerprint)
		if err != nil {
			return model.WishlistPage{}, apperror.Wrap(apperror.CodeInvalidCursor, "cursor is invalid or does not match this query", err)
		}
		boundary := store.PersonalPosition(decoded)
		position = &boundary
	}
	page, next, err := s.store.List(ctx, params, position, s.pageSize)
	if err != nil {
		return model.WishlistPage{}, dependencyError(err)
	}
	page.Items = append([]model.WishlistItem{}, page.Items...)
	if next != nil {
		payload := cursorPayload{Version: 1, Fingerprint: fingerprint, Sort: params.Sort, Position: sortPosition(*next)}
		raw, err := json.Marshal(payload)
		if err != nil {
			return model.WishlistPage{}, apperror.Wrap(apperror.CodeInternalError, "could not create pagination cursor", err)
		}
		cursor := base64.RawURLEncoding.EncodeToString(raw)
		page.NextCursor = &cursor
	}
	ids := make([]string, 0, len(page.Items))
	for _, item := range page.Items {
		ids = append(ids, item.Card.PrintingUUID)
	}
	if len(ids) > 0 {
		summaries, err := s.store.Summaries(ctx, ids)
		if err != nil {
			return model.WishlistPage{}, dependencyError(err)
		}
		for i := range page.Items {
			card, ok := summaries[page.Items[i].Card.PrintingUUID]
			if !ok {
				return model.WishlistPage{}, apperror.New(apperror.CodeDependencyUnavailable, "personal printing is unavailable")
			}
			page.Items[i].Card = card
			if params.Sort == "set" {
				set := card.Set
				page.Items[i].GroupSet = &set
			}
		}
	}
	return page, nil
}

func normalizeParams(params model.WishlistParams) model.WishlistParams {
	params.Query = strings.ToLower(strings.TrimSpace(params.Query))
	if params.Sort == "" {
		params.Sort = "set"
	}
	params.Sets = canonicalValues(params.Sets, true)
	params.Languages = canonicalValues(params.Languages, true)
	params.Colors = canonicalValues(params.Colors, false)
	params.ManaValues = canonicalValues(params.ManaValues, false)
	params.Rarities = canonicalValues(params.Rarities, false)
	return params
}

func canonicalValues(values []string, fold bool) []string {
	unique := make(map[string]bool)
	result := make([]string, 0, len(values))
	for _, value := range values {
		if fold {
			value = strings.ToLower(value)
		}
		if !unique[value] {
			unique[value] = true
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result
}

func queryFingerprint(params model.WishlistParams) string {
	params.Cursor = ""
	raw, _ := json.Marshal(params)
	sum := sha256.Sum256(append([]byte("wishlist:"), raw...))
	return hex.EncodeToString(sum[:])
}

func decodeCursor(value, order, fingerprint string) (sortPosition, error) {
	if len(value) > 4096 {
		return sortPosition{}, errors.New("cursor is too long")
	}
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return sortPosition{}, err
	}
	var payload cursorPayload
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		return sortPosition{}, err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return sortPosition{}, errors.New("cursor contains trailing data")
	}
	p := payload.Position
	if payload.Version != 1 || payload.Fingerprint != fingerprint || payload.Sort != order || !model.ValidUUID(p.UUID) ||
		p.Color < 0 || p.Color > 6 || math.IsNaN(p.Mana) || math.IsInf(p.Mana, 0) || p.Mana < 0 || p.SetCode == "" {
		return sortPosition{}, errors.New("cursor metadata mismatch")
	}
	if p.SetDate != "" {
		if _, err := time.Parse("2006-01-02", p.SetDate); err != nil {
			return sortPosition{}, err
		}
	}
	return p, nil
}

func dependencyError(err error) error {
	return apperror.Wrap(apperror.CodeDependencyUnavailable, "wishlist or card data is temporarily unavailable", err)
}
