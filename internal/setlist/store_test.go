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
	for _, fragment := range []string{"`s`.`name` LIKE ?", "`st`.`name` LIKE ?", "EXISTS", "`c`.`lang` IN (?, ?)", "`s`.`set_type` IN (?, ?)"} {
		if !strings.Contains(query, fragment) {
			t.Errorf("query does not contain %q: %s", fragment, query)
		}
	}
	wantArgs := []any{"%100=%=_Real==Set%", "%100=%=_Real==Set%", "%100=%=_Real==Set%", "en", "ja", "core", "expansion"}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("args = %#v, want %#v", args, wantArgs)
	}
}

func TestCursorExpressionKeepsSetIDAscendingForDescendingSort(t *testing.T) {
	expression, err := cursorExpression(SortCardCountDesc, cursorPosition{value: "405", setID: "set-1"})
	if err != nil {
		t.Fatalf("cursor expression: %v", err)
	}
	query, args, err := mysqlDialect.From(goqu.T("scryfall_set").As("s")).
		Prepared(true).
		Where(expression).
		Order(orderExpressions(SortCardCountDesc)...).
		ToSQL()
	if err != nil {
		t.Fatalf("build query: %v", err)
	}
	for _, fragment := range []string{"`s`.`card_count` < ?", "`s`.`card_count` = ?", "`s`.`set_id` > ?", "ORDER BY `s`.`card_count` DESC, `s`.`set_id` ASC"} {
		if !strings.Contains(query, fragment) {
			t.Errorf("query does not contain %q: %s", fragment, query)
		}
	}
	wantArgs := []any{int64(405), int64(405), "set-1"}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("args = %#v, want %#v", args, wantArgs)
	}
}
