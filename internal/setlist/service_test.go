package setlist

import (
	"testing"
	"time"
)

func TestCursorRoundTripAndQueryBinding(t *testing.T) {
	params := Params{Query: "Kaldheim", Languages: []string{"en", "ja"}, SetTypes: []string{"expansion"}, Sort: SortReleasedDesc}
	fingerprint := queryFingerprint(params)
	encoded, err := encodeCursor(cursorPayload{
		Version:     cursorVersion,
		Fingerprint: fingerprint,
		Sort:        params.Sort,
		Value:       "2021-02-05",
		SetID:       "00000000-0000-0000-0000-000000000001",
	})
	if err != nil {
		t.Fatalf("encode cursor: %v", err)
	}
	position, err := decodeCursor(encoded, params.Sort, fingerprint)
	if err != nil {
		t.Fatalf("decode cursor: %v", err)
	}
	if position.value != "2021-02-05" || position.setID != "00000000-0000-0000-0000-000000000001" {
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
		ReleasedAt: time.Date(2021, time.February, 5, 0, 0, 0, 0, time.UTC),
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
