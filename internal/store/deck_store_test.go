package store

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/doug-martin/goqu/v9"
)

func TestDeckListSQLPagination(t *testing.T) {
	position := &DeckPosition{ID: 7, Name: "same", Time: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Count: 60}
	for _, test := range []struct {
		order     string
		key       string
		ascending bool
	}{
		{"updated_desc", "d.updated_at", false},
		{"created_desc", "d.created_at", false},
		{"name_asc", "LOWER(d.name) COLLATE utf8mb4_bin", true},
		{"card_count_desc", "COALESCE(counts.quantity,0)", false},
	} {
		t.Run(test.order, func(t *testing.T) {
			query, args, err := deckListPage(deckListBase("50%_="), test.order, position, 50).ToSQL()
			if err != nil {
				t.Fatal(err)
			}
			direction, comparison := "DESC", "<"
			if test.ascending {
				direction, comparison = "ASC", ">"
			}
			for _, fragment := range []string{
				"ORDER BY " + test.key + " " + direction + ", `d`.`id` ASC",
				test.key + " " + comparison + " ?",
				test.key + " = ?", "`d`.`id` > ?", "LIMIT ?", "ESCAPE '='",
			} {
				if !strings.Contains(query, fragment) {
					t.Fatalf("missing %q in %s", fragment, query)
				}
			}
			if args[len(args)-1] != int64(51) {
				t.Fatalf("lookahead limit: %#v", args)
			}
			if test.order == "card_count_desc" && (!strings.Contains(query, "SUM(`quantity`)") || strings.Contains(query, "maybeboard")) {
				t.Fatalf("invalid counted quantities: %s", query)
			}
		})
	}
	query, args, err := deckListBase("50%_=").Select(goqu.COUNT("*")).ToSQL()
	if err != nil || !reflect.DeepEqual(args, []any{"%50=%=_==%"}) || strings.Contains(query, "LIMIT") || strings.Contains(query, "deck_card") {
		t.Fatalf("unpaged total: %s %#v %v", query, args, err)
	}
	query, _, err = deckListPage(deckListBase(""), "updated_desc", nil, 50).ToSQL()
	if err != nil || strings.Contains(query, "WHERE") {
		t.Fatalf("first page has a boundary: %s %v", query, err)
	}
}

func TestDeckSearchSQLMatching(t *testing.T) {
	for _, exact := range []bool{false, true} {
		query, args, err := deckSearchRepresentatives("50%_=", exact).ToSQL()
		if err != nil {
			t.Fatal(err)
		}
		for _, fragment := range []string{"MIN(`p`.`uuid`)", "GROUP BY `c`.`oracle_id`", "`c`.`oracle_id` IS NOT NULL", "`main`.`face_index` IN", "`printing`.`language_code`"} {
			if !strings.Contains(query, fragment) {
				t.Fatalf("missing matched version rule %q: %s", fragment, query)
			}
		}
		if strings.Contains(query, "COUNT(") || strings.Contains(query, "flavor_name") || strings.Contains(query, "card_translation") || strings.Contains(query, "LIMIT") {
			t.Fatalf("matching must only collect lightweight candidates once: %s", query)
		}
		value := "%50=%=_==%"
		if exact {
			value = "50%_="
			if strings.Contains(query, " LIKE ") || !strings.Contains(query, "`p`.`printed_name` = ?") {
				t.Fatalf("suggestion click must match the whole printed name: %s", query)
			}
		} else if strings.Count(query, " LIKE ") != 1 {
			t.Fatalf("manual contains matching must occur once: %s", query)
		}
		found := false
		for _, arg := range args {
			if arg == value {
				found = true
			}
		}
		if !found {
			t.Fatalf("missing bound name value %q: %#v", value, args)
		}
	}
	query, _, err := deckSearchRepresentatives("", false).ToSQL()
	if err != nil || strings.Contains(query, "printed_name") || !strings.Contains(query, "MIN(`c`.`scryfall_id`)") {
		t.Fatalf("blank search must retain the global default representative: %s error=%v", query, err)
	}
}
