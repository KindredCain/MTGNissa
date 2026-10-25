package setlist

import (
	"reflect"
	"strings"
	"testing"

	"github.com/doug-martin/goqu/v9"
)

func TestBuildSetDatasetCombinesSetFilters(t *testing.T) {
	dataset := buildSetDataset(Params{
		Query:     `100%_Real=Set`,
		Languages: []string{"en", "ja"},
		SetTypes:  []string{"core", "expansion"},
	})
	query, args, err := dataset.Select(goqu.COUNT("*")).ToSQL()
	if err != nil {
		t.Fatalf("build query: %v", err)
	}
	for _, fragment := range []string{"`mtgnissa_set` AS `s`", "`s`.`parent_code` IS NULL", "`s`.`name` LIKE ?", "`st`.`name` LIKE ?", "FIND_IN_SET(?, `s`.`languages`) > 0", "`s`.`set_type` IN (?, ?)"} {
		if !strings.Contains(query, fragment) {
			t.Errorf("query does not contain %q: %s", fragment, query)
		}
	}
	for _, obsolete := range []string{"scryfall_card", "scryfall_set", "set_id"} {
		if strings.Contains(query, obsolete) {
			t.Errorf("query still uses %q: %s", obsolete, query)
		}
	}
	wantArgs := []any{"%100=%=_Real==Set%", "%100=%=_Real==Set%", "%100=%=_Real==Set%", "en", "ja", "core", "expansion"}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("args = %#v, want %#v", args, wantArgs)
	}
}

func TestCursorExpressionKeepsCodeAscendingForDescendingSort(t *testing.T) {
	expression, err := cursorExpression(SortCardCountDesc, cursorPosition{value: "405", code: "KHM"})
	if err != nil {
		t.Fatalf("cursor expression: %v", err)
	}
	query, args, err := mysqlDialect.From(goqu.T("mtgnissa_set").As("s")).
		Prepared(true).
		Where(expression).
		Order(orderExpressions(SortCardCountDesc)...).
		ToSQL()
	if err != nil {
		t.Fatalf("build query: %v", err)
	}
	for _, fragment := range []string{"`s`.`card_count` < ?", "`s`.`card_count` = ?", "`s`.`code` > ?", "ORDER BY `s`.`card_count` DESC, `s`.`code` ASC"} {
		if !strings.Contains(query, fragment) {
			t.Errorf("query does not contain %q: %s", fragment, query)
		}
	}
	wantArgs := []any{int64(405), int64(405), "KHM"}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("args = %#v, want %#v", args, wantArgs)
	}
}

func TestReleaseDateCursorIncludesNullDatesLast(t *testing.T) {
	for _, sort := range []Sort{SortReleasedAsc, SortReleasedDesc} {
		t.Run(string(sort), func(t *testing.T) {
			position := cursorPosition{value: "2021-02-05", code: "KHM"}
			expression, err := cursorExpression(sort, position)
			if err != nil {
				t.Fatal(err)
			}
			query, _, err := mysqlDialect.From(goqu.T("mtgnissa_set").As("s")).Prepared(true).
				Where(expression).Order(orderExpressions(sort)...).ToSQL()
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(query, "OR (`s`.`released_at` IS NULL)") || !strings.Contains(query, "ORDER BY `s`.`released_at` IS NULL ASC") {
				t.Fatalf("dated cursor does not include null dates last: %s", query)
			}
			expression, err = cursorExpression(sort, cursorPosition{code: "KHM", isNull: true})
			if err != nil {
				t.Fatal(err)
			}
			query, args, err := mysqlDialect.From(goqu.T("mtgnissa_set").As("s")).Prepared(true).Where(expression).ToSQL()
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(query, "`s`.`released_at` IS NULL") || !strings.Contains(query, "`s`.`code` > ?") || !reflect.DeepEqual(args, []any{"KHM"}) {
				t.Fatalf("null cursor query = %s, args = %#v", query, args)
			}
		})
	}
}
