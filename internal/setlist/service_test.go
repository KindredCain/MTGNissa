package setlist

import (
	"context"
	"testing"
)

type pageStoreStub struct {
	page     Page
	position *cursorPosition
}

func (s *pageStoreStub) List(_ context.Context, _ Params, position *cursorPosition, _ int) (Page, bool, error) {
	s.position = position
	return s.page, true, nil
}

func TestListCursorUsesLastRootInsteadOfLastChild(t *testing.T) {
	for _, missingDate := range []bool{false, true} {
		root := Set{Code: "KHM", ReleasedAt: testSetDate(2021, 2, 5)}
		if missingDate {
			root.ReleasedAt = nil
		}
		store := &pageStoreStub{page: Page{Items: []Set{root, {
			Code: "TKHM", ParentCode: testSetCode("KHM"), Depth: 1, ReleasedAt: testSetDate(2022, 1, 1),
		}}, Total: 2}}
		service := &Service{store: store, pageSize: 1}
		page, err := service.List(context.Background(), Params{})
		if err != nil {
			t.Fatal(err)
		}
		if page.NextCursor == nil {
			t.Fatal("missing next cursor")
		}
		_, err = service.List(context.Background(), Params{Cursor: *page.NextCursor})
		if err != nil {
			t.Fatal(err)
		}
		if store.position == nil || store.position.code != "KHM" || store.position.isNull != missingDate || store.position.value != cursorValue(SortReleasedDesc, root) {
			t.Fatalf("cursor position = %#v", store.position)
		}
	}
}

func TestCursorRejectsInconsistentNullDateAndOldVersion(t *testing.T) {
	tests := []cursorPayload{
		{Version: 1, Sort: SortReleasedDesc, Value: "2021-02-05", Code: "KHM"},
		{Version: cursorVersion, Sort: SortReleasedDesc, Value: "2021-02-05", Code: "KHM", MissingDate: true},
		{Version: cursorVersion, Sort: SortNameAsc, Code: "KHM", MissingDate: true},
		{Version: cursorVersion, Sort: SortCodeAsc, Code: "KHM", Value: "OTHER"},
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
	params := Params{Query: "Kaldheim", Languages: []string{"en", "ja"}, SetTypes: []string{"expansion"}, Sort: SortReleasedDesc}
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
	if position.value != "2021-02-05" || position.code != "KHM" || position.isNull {
		t.Fatalf("position = %#v", position)
	}
	changed := params
	changed.Query = "Dominaria"
	if _, err := decodeCursor(encoded, changed.Sort, queryFingerprint(changed)); err == nil {
		t.Fatal("cursor was accepted for changed filters")
	}
}

func TestQueryFingerprintIgnoresMultiValueOrderAndCase(t *testing.T) {
	first := Params{Query: "KHM", Languages: []string{"ja", "EN"}, SetTypes: []string{"Expansion", "CORE"}}
	second := Params{Query: "khm", Languages: []string{"en", "JA"}, SetTypes: []string{"core", "expansion"}}
	if queryFingerprint(first) != queryFingerprint(second) {
		t.Fatal("semantically equivalent filters produced different fingerprints")
	}
}

func TestCursorValueUsesAuthoritativeSortColumn(t *testing.T) {
	item := Set{
		Name:       "Kaldheim",
		Code:       "khm",
		CardCount:  405,
		ReleasedAt: testSetDate(2021, 2, 5),
	}
	tests := map[Sort]string{
		SortReleasedDesc: "2021-02-05",
		SortNameAsc:      "Kaldheim",
		SortCodeDesc:     "khm",
		SortCardCountAsc: "405",
	}
	for sort, want := range tests {
		if got := cursorValue(sort, item); got != want {
			t.Errorf("cursorValue(%s) = %q, want %q", sort, got, want)
		}
	}
}
