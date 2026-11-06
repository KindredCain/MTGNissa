package httpapi

import (
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"testing"

	"mtgnissa/internal/apperror"
	"mtgnissa/internal/cards"
)

func TestCardListParameters(t *testing.T) {
	for _, test := range []struct {
		query string
		code  apperror.Code
	}{
		{"language=en", apperror.CodeValidationFailed},
		{"set_code=KLD", apperror.CodeValidationFailed},
		{"set_code=KLD&language=en&q=a&q=b", apperror.CodeInvalidRequest},
		{"set_code=KLD&language=en&q=%00", apperror.CodeValidationFailed},
		{"set_code=KLD&language=en&q=" + strings.Repeat("界", 513), apperror.CodeValidationFailed},
	} {
		query, err := url.ParseQuery(test.query)
		if err != nil {
			t.Fatal(err)
		}
		_, _, _, err = parseCardList(query)
		var appErr *apperror.Error
		if !errors.As(err, &appErr) || appErr.Code != test.code {
			t.Fatalf("query=%q error=%v", test.query, err)
		}
	}
	code, lang, q, err := parseCardList(url.Values{"set_code": {" KLD "}, "language": {" en "}, "q": {" 100%_ "}})
	if err != nil || code != "KLD" || lang != "en" || q != "100%_" {
		t.Fatalf("normalized query=%q,%q,%q error=%v", code, lang, q, err)
	}
}

func TestCardMutationJSONIsStrictAndPreservesFalse(t *testing.T) {
	for _, body := range []string{`{"finish":"normal","delta":1,"extra":true}`, `{"finish":"normal","delta":"1"}`, `{"finish":"normal","delta":1} {}`, `[]`} {
		var request adjustmentRequest
		if err := decodeCardRequest(strings.NewReader(body), &request); err == nil {
			t.Fatalf("invalid body accepted: %s", body)
		}
	}
	var request wishlistRequest
	if err := decodeCardRequest(strings.NewReader(`{"included":false}`), &request); err != nil || request.Included == nil || *request.Included {
		t.Fatalf("false target state was lost: %#v error=%v", request, err)
	}
	var missing wishlistRequest
	if err := decodeCardRequest(strings.NewReader(`{}`), &missing); err != nil || missing.Included != nil {
		t.Fatal("missing included cannot be distinguished from false")
	}
}

func TestMutationKeyAndPrintingUUIDAreValidated(t *testing.T) {
	valid := "A1234567-1234-4123-8123-123456789ABC"
	for _, values := range [][]string{nil, {"invalid"}, {valid, valid}, {"00000000-0000-0000-0000-000000000000"}} {
		if _, err := mutationKey(values); err == nil {
			t.Fatalf("invalid key accepted: %v", values)
		}
	}
	key, err := mutationKey([]string{valid})
	if err != nil || key != strings.ToLower(valid) {
		t.Fatalf("key=%q error=%v", key, err)
	}
	if _, err := printingID("bad"); err == nil {
		t.Fatal("invalid printing UUID accepted")
	}
}

func TestCardDTOsKeepArraysAndQuantitiesStable(t *testing.T) {
	detail := newCardDetail(cards.Detail{Collection: cards.Quantity{Normal: 2, Foil: 1}, LogicalCollection: cards.Quantity{Normal: 4, Foil: 3}})
	encoded, err := json.Marshal(detail)
	if err != nil {
		t.Fatal(err)
	}
	var data map[string]any
	if err := json.Unmarshal(encoded, &data); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"faces", "rulings", "oracle_tags", "languages", "other_versions"} {
		if _, ok := data[field].([]any); !ok {
			t.Errorf("%s must be an array: %s", field, encoded)
		}
	}
	if detail.LogicalCollection.Total != 7 || detail.Collection.Normal != 2 || detail.Collection.Foil != 1 {
		t.Fatal("concrete and logical quantities mixed")
	}
	adjustment := newAdjustment(cards.CollectionState{Card: cards.Summary{PrintingUUID: "print", Set: cards.Set{Code: "KLD"}}, Group: cards.Quantity{Normal: 2, Foil: 3}, Collection: cards.Totals{TotalQuantity: 42, DistinctCards: 30}})
	if adjustment.Group.Total != 5 || adjustment.Collection.Total != 42 || adjustment.SetSummary == nil {
		t.Fatalf("adjustment=%#v", adjustment)
	}
	if newCardList(cards.List{}).Items == nil {
		t.Fatal("empty card list must not be null")
	}
}
