package httpapi

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	"mtgnissa/internal/model"
)

func TestCollectionQueryDefaultsAndRepeatedFilters(t *testing.T) {
	params, err := parseCollectionParams(url.Values{})
	if err != nil || params.Sort != "set" || params.MergeSets || !params.MergeLanguages {
		t.Fatalf("defaults=%#v error=%v", params, err)
	}
	query, _ := url.ParseQuery("q=+Nissa+&sort=name&merge_sets=true&merge_languages=false&set_code=KLD&set_code=KHM&language=en&language=ja&color=G&color=multicolor&mana_value=5&mana_value=8_plus&rarity=mythic")
	params, err = parseCollectionParams(query)
	if err != nil || params.Query != "Nissa" || !params.MergeSets || params.MergeLanguages || len(params.Sets) != 2 || len(params.Languages) != 2 || len(params.Colors) != 2 || len(params.ManaValues) != 2 {
		t.Fatalf("params=%#v error=%v", params, err)
	}
}

func TestCollectionQueryRejectsInvalidEnumsCombinationsAndRepeatedScalars(t *testing.T) {
	for _, raw := range []string{
		"sort=other", "sort=set&merge_sets=true", "merge_sets=1", "merge_languages=", "merge_languages=False",
		"sort=name&sort=color", "q=a&q=b", "cursor=a&cursor=b", "merge_sets=false&merge_sets=true",
		"color=W,U", "color=other", "mana_value=8", "mana_value=2.5", "rarity=gold",
		"language=en,ja", "set_code=KLD,KHM", "set_code=12345678901234567", "q=%00", "cursor=%00",
	} {
		query, _ := url.ParseQuery(raw)
		if _, err := parseCollectionParams(query); err == nil {
			t.Errorf("accepted invalid query %s", raw)
		}
	}
}

func TestCollectionDTOEmptyArraysAndNullableHeaders(t *testing.T) {
	encoded, err := json.Marshal(newCollectionPage(model.CollectionPage{Items: []model.CollectionItem{{}}}))
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{`"group_set":null`, `"group_summary":null`, `"merged_sets":false`, `"merged_languages":false`, `"next_cursor":null`} {
		if !strings.Contains(string(encoded), expected) {
			t.Errorf("missing %s in %s", expected, encoded)
		}
	}
	encoded, err = json.Marshal(newCollectionOptions(model.CollectionOptions{}))
	if err != nil || string(encoded) != `{"sets":[],"languages":[],"colors":[],"mana_values":[],"rarities":[]}` {
		t.Fatalf("options=%s error=%v", encoded, err)
	}
	encoded, err = json.Marshal(newCollectionPage(model.CollectionPage{}))
	if err != nil || !strings.Contains(string(encoded), `"items":[]`) {
		t.Fatalf("page=%s error=%v", encoded, err)
	}
}
