package sets

import (
	"context"
	"testing"
	"time"

	"mtgnissa/internal/model"
	"mtgnissa/internal/store"
)

type pageStoreStub struct {
	page     model.SetPage
	position *store.SetPosition
}

func testSetCode(value string) *string {
	return &value
}

func testSetDate(year, month, day int) *time.Time {
	value := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	return &value
}

func (s *pageStoreStub) Options(_ context.Context) (model.SetOptions, error) {
	return model.SetOptions{}, nil
}

func (s *pageStoreStub) List(_ context.Context, _ model.SetParams, position *store.SetPosition, _ int) (model.SetPage, bool, error) {
	s.position = position
	return s.page, true, nil
}

func TestListCursorUsesLastRootInsteadOfLastChild(t *testing.T) {
	for _, missingDate := range []bool{false, true} {
		root := model.SetListItem{Code: "KHM", ReleasedAt: testSetDate(2021, 2, 5)}
		if missingDate {
			root.ReleasedAt = nil
		}
		store := &pageStoreStub{page: model.SetPage{Items: []model.SetListItem{root, {
			Code: "TKHM", ParentCode: testSetCode("KHM"), Depth: 1, ReleasedAt: testSetDate(2022, 1, 1),
		}}, Total: 2}}
		service := &Service{store: store, pageSize: 1}
		page, err := service.List(context.Background(), model.SetParams{})
		if err != nil {
			t.Fatal(err)
		}
		if page.NextCursor == nil {
			t.Fatal("missing next cursor")
		}
		_, err = service.List(context.Background(), model.SetParams{Cursor: *page.NextCursor})
		if err != nil {
			t.Fatal(err)
		}
		if store.position == nil || store.position.Code != "KHM" || store.position.IsNull != missingDate || store.position.Value != cursorValue(model.SortReleasedDesc, root) {
			t.Fatalf("cursor position = %#v", store.position)
		}
	}
}

func TestCursorRejectsInconsistentNullDateAndOldVersion(t *testing.T) {
	tests := []cursorPayload{
		{Version: 1, Sort: model.SortReleasedDesc, Value: "2021-02-05", Code: "KHM"},
		{Version: cursorVersion, Sort: model.SortReleasedDesc, Value: "2021-02-05", Code: "KHM", MissingDate: true},
		{Version: cursorVersion, Sort: model.SortNameAsc, Code: "KHM", MissingDate: true},
		{Version: cursorVersion, Sort: model.SortCodeAsc, Code: "KHM", Value: "OTHER"},
	}
	for _, payload := range tests {
		encoded, err := encodeCursor(payload)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := decodeCursor(encoded, payload.Sort, ""); err == nil {
			t.Fatalf("invalid cursor was accepted: %#v", payload)
		}
	}
}

func TestCursorRoundTripAndQueryBinding(t *testing.T) {
	params := model.SetParams{Query: "Kaldheim", Languages: []string{"en", "ja"}, SetTypes: []string{"expansion"}, Sort: model.SortReleasedDesc}
	fingerprint := queryFingerprint(params)
	encoded, err := encodeCursor(cursorPayload{
		Version:     cursorVersion,
		Fingerprint: fingerprint,
		Sort:        params.Sort,
		Value:       "2021-02-05",
		Code:        "KHM",
	})
	if err != nil {
		t.Fatalf("encode cursor: %v", err)
	}
	position, err := decodeCursor(encoded, params.Sort, fingerprint)
	if err != nil {
		t.Fatalf("decode cursor: %v", err)
	}
	if position.Value != "2021-02-05" || position.Code != "KHM" || position.IsNull {
		t.Fatalf("position = %#v", position)
	}
	changed := params
	changed.Query = "Dominaria"
	if _, err := decodeCursor(encoded, changed.Sort, queryFingerprint(changed)); err == nil {
		t.Fatal("cursor was accepted for changed filters")
	}
}

func TestQueryFingerprintIgnoresMultiValueOrderAndCase(t *testing.T) {
	first := model.SetParams{Query: "KHM", Languages: []string{"ja", "EN"}, SetTypes: []string{"Expansion", "CORE"}}
	second := model.SetParams{Query: "khm", Languages: []string{"en", "JA"}, SetTypes: []string{"core", "expansion"}}
	if queryFingerprint(first) != queryFingerprint(second) {
		t.Fatal("semantically equivalent filters produced different fingerprints")
	}
}

func TestCursorValueUsesAuthoritativeSortColumn(t *testing.T) {
	item := model.SetListItem{
		Name:       "Kaldheim",
		Code:       "khm",
		CardCount:  405,
		ReleasedAt: testSetDate(2021, 2, 5),
	}
	tests := map[model.SetSort]string{
		model.SortReleasedDesc: "2021-02-05",
		model.SortNameAsc:      "Kaldheim",
		model.SortCodeDesc:     "khm",
		model.SortCardCountAsc: "405",
	}
	for sort, want := range tests {
		if got := cursorValue(sort, item); got != want {
			t.Errorf("cursorValue(%s) = %q, want %q", sort, got, want)
		}
	}
}
