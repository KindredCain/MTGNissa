package httpapi

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	"mtgnissa/internal/model"
)

func TestWishlistQueryDefaultsFiltersAndRejectedMergeParameters(t *testing.T) {
	params, err := parseWishlistParams(url.Values{})
	if err != nil || params.Sort != "set" {
		t.Fatalf("defaults=%#v error=%v", params, err)
	}
	query, _ := url.ParseQuery("q=+Nissa+&sort=name&set_code=KLD&set_code=KHM&language=en&language=ja&color=G&mana_value=5&rarity=mythic")
	params, err = parseWishlistParams(query)
	if err != nil || params.Query != "Nissa" || len(params.Sets) != 2 || len(params.Languages) != 2 || params.Sort != "name" {
		t.Fatalf("params=%#v error=%v", params, err)
	}
	for _, raw := range []string{"merge_sets=false", "merge_languages=", "sort=other", "sort=name&sort=set", "q=a&q=b", "cursor=a&cursor=b", "color=gold", "mana_value=2.5", "rarity=gold", "set_code=KLD,KHM", "language=en,ja", "q=%00", "cursor=%00"} {
		query, _ := url.ParseQuery(raw)
		if _, err := parseWishlistParams(query); err == nil {
			t.Errorf("accepted invalid wishlist query %s", raw)
		}
	}
}

func TestWishlistDTOEmptyArraysAndPreciseItems(t *testing.T) {
	encoded, err := json.Marshal(newWishlistPage(model.WishlistPage{}))
	if err != nil || string(encoded) != `{"items":[],"total":0,"next_cursor":null}` {
		t.Fatalf("empty page=%s error=%v", encoded, err)
	}
	encoded, err = json.Marshal(newWishlistOptions(model.WishlistOptions{}))
	if err != nil || string(encoded) != `{"sets":[],"languages":[],"colors":[],"mana_values":[],"rarities":[]}` {
		t.Fatalf("empty options=%s error=%v", encoded, err)
	}
	encoded, err = json.Marshal(newWishlistPage(model.WishlistPage{Items: []model.WishlistItem{{Included: true}}}))
	if err != nil || !strings.Contains(string(encoded), `"group_set":null`) || !strings.Contains(string(encoded), `"included":true`) {
		t.Fatalf("item=%s error=%v", encoded, err)
	}
	for _, field := range []string{"summary", "normal_quantity", "foil_quantity", "merged_sets", "merged_languages"} {
		if strings.Contains(string(encoded), `"`+field+`"`) {
			t.Errorf("wishlist includes collection-only field %s", field)
		}
	}
}
