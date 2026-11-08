package wishlist

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"mtgnissa/internal/apperror"
	"mtgnissa/internal/model"
)

type storeStub struct {
	cards      []model.CardSummary
	summaryIDs []string
	err        error
}

func (s *storeStub) Load(context.Context) ([]model.CardSummary, error) {
	return s.cards, s.err
}

func (s *storeStub) Summaries(_ context.Context, ids []string) (map[string]model.CardSummary, error) {
	s.summaryIDs = append(s.summaryIDs, ids...)
	result := make(map[string]model.CardSummary)
	for _, card := range s.cards {
		result[card.PrintingUUID] = card
	}
	return result, s.err
}

func TestWishlistKeepsPrintingsSeparateAndPagesEverySort(t *testing.T) {
	oracle := "same-oracle"
	date := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	cards := []model.CardSummary{
		{PrintingUUID: "00000000-0000-4000-8000-000000000003", OracleID: &oracle, Name: "Zulu", CollectorNumber: "1", CMC: 1, Colors: []string{"U"}, Set: model.SetRef{Code: "SET", ReleasedAt: &date}, Language: model.Language{Code: "en"}},
		{PrintingUUID: "00000000-0000-4000-8000-000000000002", OracleID: &oracle, Name: "Alpha", CollectorNumber: "10", CMC: 9, Colors: []string{"G"}, Set: model.SetRef{Code: "SET", ReleasedAt: &date}, Language: model.Language{Code: "ja"}},
		{PrintingUUID: "00000000-0000-4000-8000-000000000001", Name: "Alpha", CollectorNumber: "2", CMC: 2, Colors: []string{"W"}, Set: model.SetRef{Code: "SET", ReleasedAt: &date}, Language: model.Language{Code: "en"}},
	}
	for _, order := range []string{"set", "color", "mana_value", "name"} {
		t.Run(order, func(t *testing.T) {
			stub := &storeStub{cards: cards}
			service := &Service{store: stub, pageSize: 1}
			params := model.WishlistParams{Sort: order}
			var ids []string
			for {
				page, err := service.List(context.Background(), params)
				if err != nil || page.Total != 3 || len(page.Items) != 1 {
					t.Fatalf("page=%#v error=%v", page, err)
				}
				item := page.Items[0]
				if !item.Included || (item.GroupSet != nil) != (order == "set") {
					t.Fatalf("invalid wishlist item: %#v", item)
				}
				ids = append(ids, item.Card.PrintingUUID)
				if page.NextCursor == nil {
					break
				}
				params.Cursor = *page.NextCursor
				if len(ids) > len(cards) {
					t.Fatal("cursor did not advance")
				}
			}
			want := []string{cards[2].PrintingUUID, cards[1].PrintingUUID, cards[0].PrintingUUID}
			switch order {
			case "set", "mana_value":
				want = []string{cards[0].PrintingUUID, cards[2].PrintingUUID, cards[1].PrintingUUID}
			case "color":
				want = []string{cards[2].PrintingUUID, cards[0].PrintingUUID, cards[1].PrintingUUID}
			}
			if !reflect.DeepEqual(ids, want) || !reflect.DeepEqual(stub.summaryIDs, want) {
				t.Fatalf("order=%v hydrated=%v", ids, stub.summaryIDs)
			}
			params.Query = "different"
			if _, err := service.List(context.Background(), params); err == nil {
				t.Fatal("cursor accepted changed filters")
			}
		})
	}
}

func TestWishlistFiltersAreLiteralAndDoNotMerge(t *testing.T) {
	stub := &storeStub{cards: []model.CardSummary{
		{PrintingUUID: "one", Name: "Nissa%_", Set: model.SetRef{Code: "KLD"}, Language: model.Language{Code: "en"}, Colors: []string{"G"}, CMC: 5, Rarity: "mythic"},
		{PrintingUUID: "two", Name: "Nissa%_", Set: model.SetRef{Code: "KLD"}, Language: model.Language{Code: "ja"}, Colors: []string{"G"}, CMC: 5, Rarity: "mythic"},
		{PrintingUUID: "three", Name: "Nissa Other", Set: model.SetRef{Code: "KLD"}, Language: model.Language{Code: "en"}, Colors: []string{"G"}, CMC: 5, Rarity: "mythic"},
	}}
	service := &Service{store: stub, pageSize: 10}
	params := model.WishlistParams{Query: " NISSA%_ ", Sets: []string{"kld"}, Languages: []string{"EN", "ja"}, Colors: []string{"G"}, ManaValues: []string{"5", "8_plus"}, Rarities: []string{"mythic"}}
	page, err := service.List(context.Background(), params)
	if err != nil || page.Total != 2 || len(page.Items) != 2 || page.NextCursor != nil {
		t.Fatalf("filtered page=%#v error=%v", page, err)
	}
	params.ManaValues = []string{"8_plus"}
	page, err = service.List(context.Background(), params)
	if err != nil || page.Items == nil || len(page.Items) != 0 || page.Total != 0 {
		t.Fatalf("empty page=%#v error=%v", page, err)
	}
	if _, err := service.List(context.Background(), model.WishlistParams{Cursor: "bad cursor"}); err == nil {
		t.Fatal("malformed cursor accepted")
	}
}

func TestWishlistOptionsUseOnlyCurrentItemsAndStableOrder(t *testing.T) {
	date := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	stub := &storeStub{cards: []model.CardSummary{
		{Set: model.SetRef{Code: "Z"}, Language: model.Language{Code: "ja", SortOrder: 7}},
		{Set: model.SetRef{Code: "B", ReleasedAt: &date}, Language: model.Language{Code: "en", SortOrder: 1}},
		{Set: model.SetRef{Code: "A", ReleasedAt: &date}, Language: model.Language{Code: "en", SortOrder: 1}},
	}}
	service := &Service{store: stub, pageSize: 10}
	options, err := service.Options(context.Background())
	if err != nil || len(options.Sets) != 3 || options.Sets[0].Code != "A" || options.Sets[1].Code != "B" || options.Sets[2].Code != "Z" || len(options.Languages) != 2 || options.Languages[0].Code != "en" {
		t.Fatalf("options=%#v error=%v", options, err)
	}
	stub.cards = nil
	options, err = service.Options(context.Background())
	if err != nil || options.Sets == nil || options.Languages == nil || len(options.Colors) != 7 || len(options.ManaValues) != 9 || len(options.Rarities) != 6 {
		t.Fatalf("empty options=%#v error=%v", options, err)
	}
	stub.err = errors.New("private database failure")
	_, err = service.List(context.Background(), model.WishlistParams{})
	var appErr *apperror.Error
	if !errors.As(err, &appErr) || appErr.Code != apperror.CodeDependencyUnavailable {
		t.Fatalf("dependency failure=%v", err)
	}
}
