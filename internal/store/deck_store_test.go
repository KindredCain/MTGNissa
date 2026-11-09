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

func TestDeckSearchSQLPagination(t *testing.T) {
	matches := deckSearchMatches("50%_=")
	position := &SearchPosition{Name: "same", OracleID: "00000000-0000-0000-0000-000000000007"}
	query, args, err := deckSearchPage(matches, position, 50).ToSQL()
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{
		"GROUP BY `main`.`oracle_id`) AS `matched` WHERE",
		"LOWER(matched.name) COLLATE utf8mb4_bin > ?",
		"LOWER(matched.name) COLLATE utf8mb4_bin = ?",
		"`matched`.`oracle_id` > ?",
		"ORDER BY LOWER(matched.name) COLLATE utf8mb4_bin ASC, `matched`.`oracle_id` ASC LIMIT ?",
	} {
		if !strings.Contains(query, fragment) {
			t.Fatalf("missing %q in %s", fragment, query)
		}
	}
	if args[len(args)-1] != int64(51) {
		t.Fatalf("lookahead limit: %#v", args)
	}
	query, args, err = mysqlDialect.From(matches.As("matched")).Select(goqu.COUNT("*")).Prepared(true).ToSQL()
	if err != nil || strings.Contains(query, "LIMIT") || strings.Contains(query, "ORDER BY") || strings.Contains(query, "matched.name") {
		t.Fatalf("total must count distinct candidates before the boundary: %s %#v %v", query, args, err)
	}
	if args[len(args)-1] != "%50=%=_==%" {
		t.Fatalf("literal name match: %#v", args)
	}
}
