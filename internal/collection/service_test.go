package collection

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"mtgnissa/internal/apperror"
	"mtgnissa/internal/model"
)

type fakeStore struct {
	rows     []model.OwnedPrinting
	loaded   int
	resolved []string
	err      error
}

func (f *fakeStore) Load(context.Context) ([]model.OwnedPrinting, error) {
	f.loaded++
	return f.rows, f.err
}

func (f *fakeStore) Summaries(_ context.Context, ids []string) (map[string]model.CardSummary, error) {
	f.resolved = append(f.resolved, ids...)
	result := make(map[string]model.CardSummary)
	for _, row := range f.rows {
		for _, id := range ids {
			if row.Card.PrintingUUID == id {
				result[id] = row.Card
			}
		}
	}
	return result, nil
}

func fixtureRecord(index int, oracle, set, number, language, date string) model.OwnedPrinting {
	var oracleID *string
	if oracle != "" {
		oracleID = &oracle
	}
	var released *time.Time
	if date != "" {
		value, _ := time.Parse("2006-01-02", date)
		released = &value
	}
	return model.OwnedPrinting{Quantity: model.Quantity{Normal: 1}, Card: model.CardSummary{
		PrintingUUID: fmt.Sprintf("00000000-0000-4000-8000-%012d", index), OracleID: oracleID,
		Name: "Card", DisplayName: "Translation", Set: model.SetRef{Code: set, DisplayName: set, ReleasedAt: released},
		CollectorNumber: number, Language: model.Language{Code: language}, ReleasedAt: released, Rarity: "common", Colors: []string{"W"},
	}}
}

func ownershipFixture() []model.OwnedPrinting {
	return []model.OwnedPrinting{
		fixtureRecord(1, "oracle", "OLD", "2", "en", "2020-01-01"),
		fixtureRecord(2, "oracle", "OLD", "2", "ja", "2020-01-01"),
		fixtureRecord(3, "oracle", "OLD", "10", "en", "2020-01-01"),
		fixtureRecord(4, "oracle", "NEW", "1", "en", "2021-01-01"),
		fixtureRecord(5, "", "OLD", "3", "en", "2020-01-01"),
		fixtureRecord(6, "", "OLD", "3", "ja", "2020-01-01"),
	}
}

func TestCollectionMergeMatrixPreservesNullOraclesAndVariantNumbers(t *testing.T) {
	for _, test := range []struct {
		sets      bool
		languages bool
		groups    int
	}{
		{false, false, 6}, {false, true, 5}, {true, false, 4}, {true, true, 3},
	} {
		store := &fakeStore{rows: ownershipFixture()}
		service := Service{store: store, pageSize: 50}
		page, err := service.List(context.Background(), model.CollectionParams{Sort: "name", MergeSets: test.sets, MergeLanguages: test.languages})
		if err != nil {
			t.Fatal(err)
		}
		if len(page.Items) != test.groups || page.Total != int64(test.groups) || page.Summary.TotalQuantity != 6 || page.Summary.DistinctCards != 3 {
			t.Fatalf("merge sets=%v languages=%v: %#v", test.sets, test.languages, page)
		}
		var total uint64
		for _, item := range page.Items {
			total += item.Quantity.Normal + item.Quantity.Foil
			if item.MergedSets != test.sets || item.MergedLanguages != test.languages || item.GroupSet != nil || item.GroupSummary != nil {
				t.Fatalf("unexpected group metadata: %#v", item)
			}
			if item.Card.OracleID == nil && item.Quantity.Normal != 1 {
				t.Fatal("unrelated null-Oracle printings were merged")
			}
		}
		if total != 6 {
			t.Fatalf("lost quantities: %d", total)
		}
		if test.sets && test.languages && page.Items[0].Card.PrintingUUID != store.rows[3].Card.PrintingUUID {
			t.Fatal("merged representative is not the latest owned printing")
		}
	}
}

func TestCollectionFiltersBeforeMergeAndKeepsAuthoritativeHeaders(t *testing.T) {
	rows := ownershipFixture()
	rows[0].Quantity = model.Quantity{Normal: 2, Foil: 3}
	rows[1].Quantity = model.Quantity{Foil: 4}
	service := Service{store: &fakeStore{rows: rows}, pageSize: 50}
	page, err := service.List(context.Background(), model.CollectionParams{Sort: "set", MergeLanguages: true, Sets: []string{"OLD"}, Languages: []string{"ja"}})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 2 || page.Summary.TotalQuantity != 13 || page.Summary.DistinctCards != 3 {
		t.Fatalf("headers were filtered: %#v", page)
	}
	if page.Items[0].Quantity.Normal != 0 || page.Items[0].Quantity.Foil != 4 || page.Items[0].Card.Language.Code != "ja" {
		t.Fatalf("filter ran after grouping: %#v", page.Items[0])
	}
	for _, item := range page.Items {
		if item.GroupSet == nil || item.GroupSet.Code != "OLD" || item.GroupSummary == nil || item.GroupSummary.TotalQuantity != 12 || item.GroupSummary.DistinctCards != 3 {
			t.Fatalf("series totals were filtered: %#v", item)
		}
	}
}

func TestCollectionFiltersUseLiteralNamesOrWithinDimensionsAndExactMana(t *testing.T) {
	row := fixtureRecord(1, "oracle", "SET", "1", "en", "2020-01-01")
	row.Card.Name, row.Card.CMC, row.Card.Colors = "A%_ Card", 2.5, []string{"W", "U"}
	if !matches(row.Card, model.CollectionParams{Query: "%_", Sets: []string{"other", "set"}, Colors: []string{"multicolor"}}) {
		t.Fatal("literal substring or OR selection failed")
	}
	for _, params := range []model.CollectionParams{{Query: "translation"}, {Languages: []string{"ja"}}, {Colors: []string{"W"}}, {ManaValues: []string{"2"}}, {Rarities: []string{"rare"}}} {
		if matches(row.Card, params) {
			t.Fatalf("unexpected match: %#v", params)
		}
	}
	row.Card.CMC = 8.5
	if !matches(row.Card, model.CollectionParams{ManaValues: []string{"0", "8_plus"}}) {
		t.Fatal("8_plus did not include a fractional value above eight")
	}
}

func TestCollectionCursorPagesAllSortsAndBindsEffectiveQuery(t *testing.T) {
	for _, order := range []string{"set", "color", "mana_value", "name"} {
		rows := ownershipFixture()
		rows[4].Card.Set.ReleasedAt = nil
		store := &fakeStore{rows: rows}
		service := Service{store: store, pageSize: 2}
		params := model.CollectionParams{Sort: order, Languages: []string{"en", "ja"}}
		var seen []string
		for {
			page, err := service.List(context.Background(), params)
			if err != nil {
				t.Fatal(err)
			}
			for _, item := range page.Items {
				seen = append(seen, item.Card.PrintingUUID)
			}
			if page.NextCursor == nil {
				break
			}
			params.Cursor = *page.NextCursor
			params.Languages = []string{"ja", "en", "en"}
			if len(seen) > 6 {
				t.Fatal("cursor did not advance")
			}
		}
		if len(seen) != 6 || len(store.resolved) != 6 {
			t.Fatalf("sort=%s lost items or hydrated more than the pages: %v", order, seen)
		}
		unique := make(map[string]bool)
		for _, id := range seen {
			if unique[id] {
				t.Fatalf("duplicate printing: %s", id)
			}
			unique[id] = true
		}
		if order == "set" && (seen[0] != rows[3].Card.PrintingUUID || seen[1] != rows[0].Card.PrintingUUID || seen[2] != rows[1].Card.PrintingUUID) {
			t.Fatalf("set date or natural number ordering failed: %v", seen)
		}
		before := store.loaded
		params.Query = "different"
		_, err := service.List(context.Background(), params)
		var appErr *apperror.Error
		if !errors.As(err, &appErr) || appErr.Code != apperror.CodeInvalidCursor || store.loaded != before {
			t.Fatalf("mismatched cursor reached the store: %v", err)
		}
	}
}

func TestCollectionCursorMalformedAndEmptyResults(t *testing.T) {
	service := Service{store: &fakeStore{}, pageSize: 2}
	for _, cursor := range []string{"!", base64.RawURLEncoding.EncodeToString([]byte(`{"v":1,"unknown":true}`))} {
		if _, err := service.List(context.Background(), model.CollectionParams{Cursor: cursor}); err == nil {
			t.Fatal("malformed cursor was accepted")
		}
	}
	page, err := service.List(context.Background(), model.CollectionParams{})
	if err != nil || page.Items == nil || page.Total != 0 || page.NextCursor != nil || page.Summary != (model.OwnershipTotals{}) {
		t.Fatalf("empty page=%#v error=%v", page, err)
	}
}

func TestCollectionPrimarySortsAndRepresentativeTieBreak(t *testing.T) {
	rows := ownershipFixture()[:4]
	rows[0].Card.Name, rows[0].Card.CMC, rows[0].Card.Colors = "Zulu", 3, []string{"W"}
	rows[1].Card.Name, rows[1].Card.CMC, rows[1].Card.Colors = "alpha", 2, []string{"B"}
	rows[2].Card.Name, rows[2].Card.CMC, rows[2].Card.Colors = "Beta", 2.5, []string{"W", "U"}
	rows[3].Card.Name, rows[3].Card.CMC, rows[3].Card.Colors = "ALPHA", 2, []string{}
	for _, test := range []struct {
		order string
		want  []int
	}{
		{"name", []int{1, 3, 2, 0}}, {"color", []int{0, 1, 2, 3}}, {"mana_value", []int{1, 3, 2, 0}},
	} {
		service := Service{store: &fakeStore{rows: rows}, pageSize: 50}
		page, err := service.List(context.Background(), model.CollectionParams{Sort: test.order})
		if err != nil {
			t.Fatal(err)
		}
		for i, index := range test.want {
			if page.Items[i].Card.PrintingUUID != rows[index].Card.PrintingUUID {
				t.Fatalf("sort %s: position %d has %s", test.order, i, page.Items[i].Card.PrintingUUID)
			}
		}
	}
	rows[3].Card.ReleasedAt = rows[0].Card.ReleasedAt
	service := Service{store: &fakeStore{rows: rows}, pageSize: 50}
	page, err := service.List(context.Background(), model.CollectionParams{Sort: "name", MergeSets: true, MergeLanguages: true})
	if err != nil || len(page.Items) != 1 || page.Items[0].Card.PrintingUUID != rows[0].Card.PrintingUUID {
		t.Fatalf("representative tie break: %#v error=%v", page, err)
	}
}

func TestCollectionOptionsUseAllPositiveHoldingsAndStableDictionaryOrder(t *testing.T) {
	rows := ownershipFixture()
	rows[1].Card.Language.SortOrder = 2
	rows[0].Card.Language.SortOrder = 1
	rows[4].Card.Set = model.SetRef{Code: "UNDATED"}
	rows[5].Quantity = model.Quantity{}
	service := Service{store: &fakeStore{rows: rows}}
	options, err := service.Options(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var sets, languages []string
	for _, set := range options.Sets {
		sets = append(sets, set.Code)
	}
	for _, language := range options.Languages {
		languages = append(languages, language.Code)
	}
	if !reflect.DeepEqual(sets, []string{"NEW", "OLD", "UNDATED"}) || !reflect.DeepEqual(languages, []string{"en", "ja"}) || len(options.Colors) != 7 || len(options.ManaValues) != 9 || len(options.Rarities) != 6 {
		t.Fatalf("options=%#v", options)
	}
}
