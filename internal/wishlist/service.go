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
	"strconv"
	"strings"
	"time"

	"mtgnissa/internal/apperror"
	"mtgnissa/internal/model"
	"mtgnissa/internal/store"
)

type wishlistStore interface {
	Load(context.Context) ([]model.CardSummary, error)
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

// Options returns series and languages present in the complete wishlist.
func (s *Service) Options(ctx context.Context) (model.WishlistOptions, error) {
	rows, err := s.store.Load(ctx)
	if err != nil {
		return model.WishlistOptions{}, dependencyError(err)
	}
	result := model.WishlistOptions{Sets: []model.SetRef{}, Languages: []model.Language{},
		Colors:     []string{"W", "U", "B", "R", "G", "multicolor", "colorless"},
		ManaValues: []string{"0", "1", "2", "3", "4", "5", "6", "7", "8_plus"},
		Rarities:   []string{"common", "uncommon", "rare", "mythic", "special", "bonus"}}
	sets, languages := make(map[string]bool), make(map[string]bool)
	for _, row := range rows {
		if !sets[row.Set.Code] {
			sets[row.Set.Code] = true
			result.Sets = append(result.Sets, row.Set)
		}
		if !languages[row.Language.Code] {
			languages[row.Language.Code] = true
			result.Languages = append(result.Languages, row.Language)
		}
	}
	sort.Slice(result.Sets, func(i, j int) bool {
		if order := compareDates(result.Sets[i].ReleasedAt, result.Sets[j].ReleasedAt); order != 0 {
			return order < 0
		}
		return result.Sets[i].Code < result.Sets[j].Code
	})
	sort.Slice(result.Languages, func(i, j int) bool {
		if result.Languages[i].SortOrder != result.Languages[j].SortOrder {
			return result.Languages[i].SortOrder < result.Languages[j].SortOrder
		}
		return result.Languages[i].Code < result.Languages[j].Code
	})
	return result, nil
}

// List filters and pages precise wishlist printings without merging languages or versions.
func (s *Service) List(ctx context.Context, params model.WishlistParams) (model.WishlistPage, error) {
	params = normalizeParams(params)
	fingerprint := queryFingerprint(params)
	var position *sortPosition
	if params.Cursor != "" {
		decoded, err := decodeCursor(params.Cursor, params.Sort, fingerprint)
		if err != nil {
			return model.WishlistPage{}, apperror.Wrap(apperror.CodeInvalidCursor, "cursor is invalid or does not match this query", err)
		}
		position = &decoded
	}
	rows, err := s.store.Load(ctx)
	if err != nil {
		return model.WishlistPage{}, dependencyError(err)
	}
	page := model.WishlistPage{Items: []model.WishlistItem{}}
	items := make([]model.WishlistItem, 0, len(rows))
	for _, card := range rows {
		if !matches(card, params) {
			continue
		}
		item := model.WishlistItem{Card: card, Included: true}
		if params.Sort == "set" {
			set := card.Set
			item.GroupSet = &set
		}
		items = append(items, item)
	}
	page.Total = int64(len(items))
	sort.Slice(items, func(i, j int) bool {
		return comparePositions(itemPosition(items[i]), itemPosition(items[j]), params.Sort) < 0
	})
	start := 0
	if position != nil {
		start = sort.Search(len(items), func(i int) bool {
			return comparePositions(itemPosition(items[i]), *position, params.Sort) > 0
		})
	}
	end := min(start+s.pageSize, len(items))
	page.Items = items[start:end]
	if end < len(items) {
		payload := cursorPayload{Version: 1, Fingerprint: fingerprint, Sort: params.Sort, Position: itemPosition(items[end-1])}
		encoded, err := json.Marshal(payload)
		if err != nil {
			return model.WishlistPage{}, apperror.Wrap(apperror.CodeInternalError, "could not create pagination cursor", err)
		}
		cursor := base64.RawURLEncoding.EncodeToString(encoded)
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
				return model.WishlistPage{}, apperror.New(apperror.CodeDependencyUnavailable, "wishlist printing is unavailable")
			}
			page.Items[i].Card = card
		}
	}
	return page, nil
}

func matches(card model.CardSummary, params model.WishlistParams) bool {
	if !strings.Contains(strings.ToLower(card.Name), params.Query) ||
		!selected(params.Sets, strings.ToLower(card.Set.Code)) ||
		!selected(params.Languages, strings.ToLower(card.Language.Code)) ||
		!selected(params.Colors, colorGroup(card.Colors)) || !selected(params.Rarities, card.Rarity) {
		return false
	}
	if len(params.ManaValues) == 0 {
		return true
	}
	for _, value := range params.ManaValues {
		if value == "8_plus" && card.CMC >= 8 {
			return true
		}
		if number, err := strconv.ParseFloat(value, 64); err == nil && card.CMC == number {
			return true
		}
	}
	return false
}

func selected(values []string, value string) bool {
	if len(values) == 0 {
		return true
	}
	for _, choice := range values {
		if choice == value {
			return true
		}
	}
	return false
}

func colorGroup(colors []string) string {
	if len(colors) == 0 {
		return "colorless"
	}
	if len(colors) > 1 {
		return "multicolor"
	}
	return colors[0]
}

func itemPosition(item model.WishlistItem) sortPosition {
	card := item.Card
	date := ""
	if card.Set.ReleasedAt != nil {
		date = card.Set.ReleasedAt.Format("2006-01-02")
	}
	color := 6
	for i, choice := range []string{"W", "U", "B", "R", "G", "multicolor", "colorless"} {
		if choice == colorGroup(card.Colors) {
			color = i
			break
		}
	}
	return sortPosition{SetDate: date, SetCode: card.Set.Code, Number: card.CollectorNumber, Color: color,
		Mana: card.CMC, Name: strings.ToLower(card.Name), UUID: card.PrintingUUID}
}

func comparePositions(a, b sortPosition, order string) int {
	switch order {
	case "set":
		if value := strings.Compare(b.SetDate, a.SetDate); value != 0 {
			return value
		}
		if value := strings.Compare(a.SetCode, b.SetCode); value != 0 {
			return value
		}
		if value := model.CompareCollectorNumbers(a.Number, b.Number); value != 0 {
			return value
		}
	case "color":
		if a.Color != b.Color {
			return a.Color - b.Color
		}
	case "mana_value":
		if a.Mana < b.Mana {
			return -1
		}
		if a.Mana > b.Mana {
			return 1
		}
	}
	if order != "set" {
		if value := strings.Compare(a.Name, b.Name); value != 0 {
			return value
		}
	}
	return strings.Compare(a.UUID, b.UUID)
}

func compareDates(a, b *time.Time) int {
	if a == nil && b == nil {
		return 0
	}
	if a == nil {
		return 1
	}
	if b == nil {
		return -1
	}
	return b.Compare(*a)
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
