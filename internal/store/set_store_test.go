package store

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/doug-martin/goqu/v9"

	"mtgnissa/internal/model"
)

func TestBuildSetDatasetCombinesSetFilters(t *testing.T) {
	dataset := buildSetDataset(model.SetParams{
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
	expression, err := cursorExpression(model.SortCardCountDesc, SetPosition{Value: "405", Code: "KHM"})
	if err != nil {
		t.Fatalf("cursor expression: %v", err)
	}
	query, args, err := mysqlDialect.From(goqu.T("mtgnissa_set").As("s")).
		Prepared(true).
		Where(expression).
		Order(orderExpressions(model.SortCardCountDesc)...).
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
	for _, sort := range []model.SetSort{model.SortReleasedAsc, model.SortReleasedDesc} {
		t.Run(string(sort), func(t *testing.T) {
			position := SetPosition{Value: "2021-02-05", Code: "KHM"}
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
			expression, err = cursorExpression(sort, SetPosition{Code: "KHM", IsNull: true})
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

func testSetCode(value string) *string {
	return &value
}

func testSetDate(year, month, day int) *time.Time {
	value := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	return &value
}

func TestHierarchyExpandsWholeGroupInSiblingOrder(t *testing.T) {
	links := []hierarchyRow{
		{Code: "A"}, {Code: "B"},
		{Code: "C0", ParentCode: testSetCode("A")},
		{Code: "GC", ParentCode: testSetCode("C0")},
		{Code: "CB", ParentCode: testSetCode("A")},
		{Code: "CA", ParentCode: testSetCode("A")},
		{Code: "U", ParentCode: testSetCode("A")},
		{Code: "T1", ParentCode: testSetCode("A")},
		{Code: "T2", ParentCode: testSetCode("A")},
		{Code: "BC", ParentCode: testSetCode("B")},
	}
	hierarchy, err := newSetHierarchy(links)
	if err != nil {
		t.Fatal(err)
	}
	roots := []model.SetListItem{{Code: "A", SetType: "expansion"}}
	wantCodes := []string{"C0", "CA", "CB", "GC", "T1", "T2", "U"}
	if codes := hierarchy.descendantCodes(roots); !reflect.DeepEqual(codes, wantCodes) {
		t.Fatalf("descendant codes = %#v, want %#v", codes, wantCodes)
	}
	descendants := []model.SetListItem{
		{Code: "CB", ParentCode: testSetCode("A"), ReleasedAt: testSetDate(2021, 1, 1)},
		{Code: "T2", ParentCode: testSetCode("A"), SetType: "token"},
		{Code: "U", ParentCode: testSetCode("A")},
		{Code: "CA", ParentCode: testSetCode("A"), ReleasedAt: testSetDate(2021, 1, 1)},
		{Code: "T1", ParentCode: testSetCode("A"), SetType: "token", ReleasedAt: testSetDate(1990, 1, 1)},
		{Code: "GC", ParentCode: testSetCode("C0"), SetType: "token"},
		{Code: "C0", ParentCode: testSetCode("A"), ReleasedAt: testSetDate(2020, 1, 1)},
	}
	items, err := hierarchy.expand(roots, descendants)
	if err != nil {
		t.Fatal(err)
	}
	wantOrder := []string{"A", "C0", "GC", "CA", "CB", "U", "T1", "T2"}
	wantDepth := []int{0, 1, 2, 1, 1, 1, 1, 1}
	if len(items) != len(wantOrder) {
		t.Fatalf("got %d items, want %d", len(items), len(wantOrder))
	}
	for i, item := range items {
		if item.Code != wantOrder[i] || item.Depth != wantDepth[i] {
			t.Errorf("item %d = %s depth %d, want %s depth %d", i, item.Code, item.Depth, wantOrder[i], wantDepth[i])
		}
	}
	if _, err := hierarchy.expand(roots, descendants[:len(descendants)-1]); err == nil {
		t.Error("incomplete subtree was accepted")
	}
}

func TestHierarchyRejectsMissingParentsAndCycles(t *testing.T) {
	tests := map[string][]hierarchyRow{
		"missing parent": {{Code: "A", ParentCode: testSetCode("missing")}},
		"self cycle":     {{Code: "A", ParentCode: testSetCode("A")}},
		"detached cycle": {{Code: "ROOT"}, {Code: "A", ParentCode: testSetCode("B")}, {Code: "B", ParentCode: testSetCode("A")}},
		"duplicate code": {{Code: "A"}, {Code: "a"}},
	}
	for name, rows := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := newSetHierarchy(rows); err == nil {
				t.Fatal("invalid hierarchy was accepted")
			}
		})
	}
}

func TestHierarchyKeepsSelectedRootOrder(t *testing.T) {
	hierarchy, err := newSetHierarchy([]hierarchyRow{{Code: "A"}, {Code: "B"}, {Code: "C", ParentCode: testSetCode("b")}})
	if err != nil {
		t.Fatal(err)
	}
	items, err := hierarchy.expand([]model.SetListItem{{Code: "B"}, {Code: "A"}}, []model.SetListItem{{Code: "C", ParentCode: testSetCode("b")}})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 || items[0].Code != "B" || items[1].Code != "C" || items[2].Code != "A" {
		t.Fatalf("root order was changed: %#v", items)
	}
}
