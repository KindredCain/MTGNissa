package httpapi

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"testing"

	"mtgnissa/internal/model"
)

func TestDeckSaveRequiredFieldsStrictJSONAndQuantityBounds(t *testing.T) {
	valid := `{"name":"Empty","cover_printing_uuid":null,"sections":[]}`
	var request deckSaveRequest
	if err := decodeCardRequest(strings.NewReader(valid), &request); err != nil {
		t.Fatal(err)
	}
	if parsed, err := parseDeckSave(request); err != nil || parsed.Name != "Empty" || parsed.CoverPrintingUUID != nil || parsed.Sections == nil {
		t.Fatalf("empty save=%#v error=%v", parsed, err)
	}
	for _, raw := range []string{
		`{"name":"Test","sections":[]}`,
		`{"cover_printing_uuid":null,"sections":[]}`,
		`{"name":"Test","cover_printing_uuid":null}`,
		`{"name":"Test","cover_printing_uuid":null,"sections":[{"type":"commander"}]}`,
		`{"name":"Test","cover_printing_uuid":null,"sections":[{"type":"custom","cards":[]}]}`,
		`{"name":"Test","cover_printing_uuid":null,"sections":[{"type":"mainboard","cards":[]},{"type":"mainboard","cards":[]}]}`,
	} {
		var request deckSaveRequest
		if err := decodeCardRequest(strings.NewReader(raw), &request); err != nil {
			t.Fatal(err)
		}
		if _, err := parseDeckSave(request); err == nil {
			t.Errorf("accepted missing or invalid save fields: %s", raw)
		}
	}
	for _, raw := range []string{
		`{"name":"Test","cover_printing_uuid":null,"sections":[],"oracle_id":"bad"}`,
		valid + ` {}`,
		`{"name":1,"cover_printing_uuid":null,"sections":[]}`,
		`{"name":"Test","cover_printing_uuid":null,"sections":[{"type":"mainboard","cards":[{"printing_uuid":"id","quantity":1.5}]}]}`,
	} {
		var request deckSaveRequest
		if err := decodeCardRequest(strings.NewReader(raw), &request); err == nil {
			t.Errorf("accepted invalid JSON structure: %s", raw)
		}
	}
	for _, quantity := range []int64{-1, 0, 1, 4294967295, 4294967296} {
		raw := fmt.Sprintf(`{"name":"Test","cover_printing_uuid":null,"sections":[{"type":"mainboard","cards":[{"printing_uuid":"00000000-0000-4000-8000-000000000001","quantity":%d}]}]}`, quantity)
		var request deckSaveRequest
		if err := decodeCardRequest(strings.NewReader(raw), &request); err != nil {
			t.Fatal(err)
		}
		_, err := parseDeckSave(request)
		if (err == nil) != (quantity >= 1 && quantity <= 4294967295) {
			t.Errorf("quantity %d validation=%v", quantity, err)
		}
	}
}

func TestDeckQueryIDsContextBatchesAndBodylessActions(t *testing.T) {
	for _, raw := range []string{"0", "-1", "+1", "1.1", "18446744073709551616", "123456789012345678901"} {
		if _, err := deckID(raw); err == nil {
			t.Errorf("accepted deck ID %s", raw)
		}
	}
	if _, err := deckID("18446744073709551615"); err != nil {
		t.Fatal(err)
	}
	params, err := parseDeckQuery(url.Values{})
	if err != nil || params.Sort != "updated_desc" {
		t.Fatalf("query defaults=%#v error=%v", params, err)
	}
	for _, raw := range []string{"q=a&q=b", "sort=name", "sort=name_asc&sort=updated_desc", "q=%00", "cursor=%00"} {
		query, _ := url.ParseQuery(raw)
		if _, err := parseDeckQuery(query); err == nil {
			t.Errorf("accepted query %s", raw)
		}
	}
	ids := []string{"00000000-0000-4000-8000-000000000001"}
	if parsed, err := parseDeckContextIDs(deckContextsRequest{PrintingUUIDs: &ids}); err != nil || len(parsed) != 1 {
		t.Fatalf("context IDs=%v error=%v", parsed, err)
	}
	for _, ids := range [][]string{nil, {}, {"bad"}, {ids[0], ids[0]}, make([]string, 201)} {
		if _, err := parseDeckContextIDs(deckContextsRequest{PrintingUUIDs: &ids}); err == nil {
			t.Errorf("accepted invalid context batch %v", ids)
		}
	}
	if noDeckBody(strings.NewReader(" \n")) != nil || noDeckBody(strings.NewReader("{}")) == nil {
		t.Fatal("bodyless action accepted JSON or rejected whitespace")
	}
}

func TestDeckDTOsKeepEmptyArraysNullsFixedKeysAndStringIDs(t *testing.T) {
	raw, err := json.Marshal(newDeckPage(model.DeckPage{}))
	if err != nil || string(raw) != `{"items":[],"total":0,"next_cursor":null}` {
		t.Fatalf("empty page=%s error=%v", raw, err)
	}
	raw, err = json.Marshal(newDeckPage(model.DeckPage{Items: []model.DeckListItem{{ID: 18446744073709551615}}}))
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{`"deck_id":"18446744073709551615"`, `"cover_card":null`, `"exception":null`, `"commander":0`, `"mainboard":0`, `"sideboard":0`, `"maybeboard":0`, `"W":0`, `"C":0`} {
		if !strings.Contains(string(raw), fragment) {
			t.Errorf("missing %s in %s", fragment, raw)
		}
	}
	raw, err = json.Marshal(newDeckContext(model.DeckEditCardContext{}))
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{`"colors":[]`, `"produced_mana":[]`, `"type_groups":[]`, `"oracle_tags":[]`, `"translated_oracle_text":null`} {
		if !strings.Contains(string(raw), fragment) {
			t.Errorf("missing %s in %s", fragment, raw)
		}
	}
	if strings.Contains(string(raw), `"all_decks_quantity"`) || strings.Contains(string(raw), `"exception"`) {
		t.Fatal("edit context included saved-state-only fields")
	}
}

func TestDeckRoutesRegisterTenEndpointsWithoutRequests(t *testing.T) {
	router := New(Dependencies{})
	count := 0
	for _, route := range router.Routes() {
		if route.Path == "/api/v1/decks" || strings.HasPrefix(route.Path, "/api/v1/decks/") {
			count++
		}
	}
	if count != 10 {
		t.Fatalf("registered %d deck endpoints", count)
	}
}
