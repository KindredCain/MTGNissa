package store

import (
	"reflect"
	"strings"
	"testing"

	"mtgnissa/internal/model"
)

func TestMatchedSearchKeepsVersionsOnlyFromSuccessfulBranches(t *testing.T) {
	name := &model.SearchExpression{Kind: "atom", Condition: model.SearchCondition{Field: "name", Value: "Miku"}}
	set := &model.SearchExpression{Kind: "atom", Condition: model.SearchCondition{Field: "set", Value: "SLD"}}
	color := &model.SearchExpression{Kind: "atom", Condition: model.SearchCondition{Field: "c", Operator: "=", ColorMask: 16}}
	expression := &model.SearchExpression{Kind: "or", Left: &model.SearchExpression{Kind: "and", Left: name, Right: set}, Right: color}
	if !containsSearchName(expression) || containsSearchName(color) || containsSearchName(nil) {
		t.Fatal("name detection changed the non-name search path")
	}
	hits, err := searchNamedOracleSet(expression)
	if err != nil {
		t.Fatal(err)
	}
	query, args, err := searchMatchedRepresentatives(hits).ToSQL()
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{
		"`left_set`.`oracle_id` = `right_set`.`oracle_id`",
		"COALESCE(LEAST(`left_set`.`printing_uuid`, `right_set`.`printing_uuid`)",
		"NULL AS `printing_uuid`",
		"`printing`.`uuid` = `matched`.`printing_uuid`",
		"COALESCE(`printing`.`card_uuid`, `matched`.`flavor_uuid`)",
		"`matched`.`flavor_uuid` IS NULL",
		"`matched`.`printing_uuid` IS NULL",
		"MIN(`fallback`.`scryfall_id`)",
		"`main`.`face_index` IN",
		"`main`.`oracle_id` = `matched`.`oracle_id`",
		"`printing`.`language_code`",
	} {
		if !strings.Contains(query, fragment) {
			t.Fatalf("missing matched-version rule %q: %s", fragment, query)
		}
	}
	if strings.Count(query, " LIKE ") != 2 || strings.Contains(query, "Miku") || strings.Contains(query, "card_translation") {
		t.Fatalf("name matching was repeated or page summaries were loaded early: %s args=%v", query, args)
	}
	for _, expression := range []*model.SearchExpression{nil, {Kind: "unknown"}} {
		if _, err := searchNamedOracleSet(expression); err == nil {
			t.Fatal("invalid expression was accepted")
		}
	}
}

func TestMatchedSearchPagesKeepNamesAndOracleTiesStable(t *testing.T) {
	rows := []searchRepresentativeRow{
		{Name: "Zulu", OracleID: "four"},
		{Name: "alpha", OracleID: "two"},
		{Name: "Beta", OracleID: "three"},
		{Name: "ALPHA", OracleID: "one"},
	}
	var ids []string
	var position *SearchPosition
	for {
		page, more := pageMatchedRepresentatives(append([]searchRepresentativeRow{}, rows...), position, 2)
		for _, row := range page {
			ids = append(ids, row.OracleID)
		}
		if !more {
			break
		}
		last := page[len(page)-1]
		position = &SearchPosition{Name: strings.ToLower(last.Name), OracleID: last.OracleID}
	}
	if !reflect.DeepEqual(ids, []string{"one", "two", "three", "four"}) {
		t.Fatalf("cursor skipped or duplicated a logical card: %v", ids)
	}
	for _, rows := range [][]searchRepresentativeRow{nil, rows} {
		page, more := pageMatchedRepresentatives(rows, &SearchPosition{Name: "zzzz", OracleID: "last"}, 2)
		if len(page) != 0 || more {
			t.Fatalf("past-end cursor or empty result: page=%v more=%v", page, more)
		}
	}
}

func TestMatchedSearchUsesTheMatchedLanguageAndCanonicalMainEntry(t *testing.T) {
	main := "main-face"
	rows := []searchEntryRow{
		{MainUUID: main, PrintingUUID: "english-front", LanguageCode: "en", LanguageOrder: 1},
		{MainUUID: main, PrintingUUID: "japanese-front", LanguageCode: "ja", LanguageOrder: 2},
	}
	for _, test := range []struct {
		Language string
		Expected string
	}{
		{"ja", "japanese-front"},
		{"", "english-front"},
	} {
		representatives := []searchRepresentativeRow{{MainUUID: &main, OracleID: "oracle", MatchedLanguageCode: test.Language}}
		entries, err := selectSearchEntries(representatives, rows)
		if err != nil || entries[main].PrintingUUID != test.Expected {
			t.Fatalf("language=%q entries=%v error=%v", test.Language, entries, err)
		}
	}
	for _, representatives := range [][]searchRepresentativeRow{
		{{MainUUID: &main, MatchedLanguageCode: "zhs"}},
		{{MainUUID: &main}},
		{{OracleID: "missing-main"}},
	} {
		candidates := rows
		if representatives[0].MainUUID != nil && representatives[0].MatchedLanguageCode == "" {
			candidates = nil
		}
		if _, err := selectSearchEntries(representatives, candidates); err == nil {
			t.Fatalf("missing matched language or main entry must not silently change versions: %v", representatives)
		}
	}
}
