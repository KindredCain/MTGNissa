package store

import (
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/doug-martin/goqu/v9"

	"mtgnissa/internal/apperror"
	"mtgnissa/internal/model"
)

func TestAdjustmentDoesNotBorrowOtherFinishAndChecksBounds(t *testing.T) {
	_, err := adjustQuantity(model.Quantity{Foil: 3}, "normal", -1)
	var appErr *apperror.Error
	if !errors.As(err, &appErr) || appErr.Code != apperror.CodeCollectionUnderflow {
		t.Fatalf("underflow error=%v", err)
	}
	q, err := adjustQuantity(model.Quantity{Normal: 1, Foil: 3}, "normal", -1)
	if err != nil || q.Normal != 0 || q.Foil != 3 {
		t.Fatalf("quantity=%#v error=%v", q, err)
	}
	q, err = adjustQuantity(model.Quantity{Normal: 1}, "normal", -1)
	if err != nil || q.Normal+q.Foil != 0 {
		t.Fatalf("zero state=%#v error=%v", q, err)
	}
	if _, err := adjustQuantity(model.Quantity{Foil: math.MaxUint32}, "foil", 1); err == nil {
		t.Fatal("quantity overflow accepted")
	}
	if _, err := adjustQuantity(model.Quantity{}, "etched", 1); err == nil {
		t.Fatal("unsupported finish accepted")
	}
}

func TestPersonalFiltersStayInAppDBAndTreatNamesLiterally(t *testing.T) {
	for _, table := range []string{"collection_item", "wishlist_item"} {
		params := model.CollectionParams{Query: "a%_=", Sets: []string{"old", "new"}, Languages: []string{"en", "ja"}, Colors: []string{"W", "multicolor", "colorless"}, ManaValues: []string{"2", "8_plus"}, Rarities: []string{"common", "rare"}}
		query, args, err := personalColumns(personalFilters(personalBase(table), params)).ToSQL()
		if err != nil {
			t.Fatal(err)
		}
		for _, fragment := range []string{"FROM `" + table + "` AS `c` INNER JOIN `card_print_ref` AS `r`", "LOWER(r.name) COLLATE utf8mb4_bin LIKE ? ESCAPE '='", "`r`.`set_code` IN (?, ?)", "`r`.`lang` IN (?, ?)", "`r`.`rarity` IN (?, ?)", "BIT_COUNT(r.colors_mask)>1", "`r`.`cmc` >= ?", "`r`.`cmc` = ?"} {
			if !strings.Contains(query, fragment) {
				t.Fatalf("missing %q: %s", fragment, query)
			}
		}
		found := false
		for _, arg := range args {
			found = found || arg == "%a=%=_==%"
		}
		if !found || strings.Contains(query, "is_real") || strings.Contains(query, "mtgnissa_card") {
			t.Fatalf("invalid data scope or literal name: %s %#v", query, args)
		}
		if table == "collection_item" && (!strings.Contains(query, "`c`.`normal_quantity` > ?") || !strings.Contains(query, "`c`.`foil_quantity` > ?")) {
			t.Fatalf("zero holdings included: %s", query)
		}
	}
}

func TestCollectionSQLGroupsFilteredRowsAndKeepsExactMergeKeys(t *testing.T) {
	for _, test := range []struct{ sets, languages bool }{{false, false}, {false, true}, {true, false}, {true, true}} {
		groups := collectionGroups(personalFilters(personalBase("collection_item"), model.CollectionParams{Languages: []string{"ja"}}), test.sets, test.languages)
		query, _, err := groups.ToSQL()
		if err != nil {
			t.Fatal(err)
		}
		index := strings.LastIndex(query, " GROUP BY ")
		if index < 0 || !strings.Contains(query[:index], "`r`.`lang` IN (?)") || !strings.Contains(query, "SUM(`c`.`normal_quantity`)") || !strings.Contains(query, "SUM(`c`.`foil_quantity`)") {
			t.Fatalf("filters or quantities not inside grouping: %s", query)
		}
		keys := query[index:]
		if strings.Contains(keys, "NULLIF(r.oracle_id,'')") != (test.sets || test.languages) || !strings.Contains(keys, "CONCAT('print:',r.printing_uuid)") || strings.Contains(keys, "CAST(r.set_code AS BINARY)") != (!test.sets && test.languages) || strings.Contains(keys, "CAST(r.collector_number AS BINARY)") != (!test.sets && test.languages) || strings.Contains(keys, "CAST(r.lang AS BINARY)") != (test.sets && !test.languages) {
			t.Fatalf("merge sets=%v languages=%v: %s", test.sets, test.languages, keys)
		}
		if !strings.Contains(query, "MIN(CONCAT(LPAD(99999999-COALESCE(") || !strings.Contains(query, "r.printing_uuid)),9)") {
			t.Fatalf("representative must prefer newest non-NULL release and smallest UUID: %s", query)
		}
		count, _, err := mysqlDialect.From(groups.As("groups")).Select(goqu.COUNT("*")).Prepared(true).ToSQL()
		if err != nil || strings.Contains(count, "LIMIT") || strings.Contains(count, "ORDER BY") {
			t.Fatalf("total includes page bounds: %s %v", count, err)
		}
	}
	query, args, err := collectionTotals(personalBase("collection_item")).ToSQL()
	if err != nil || strings.Contains(query, "LIKE") || strings.Contains(query, "`r`.`lang` IN") || !strings.Contains(query, "COUNT(DISTINCT CAST(CASE WHEN NULLIF(r.oracle_id,'') IS NULL") || len(args) != 2 {
		t.Fatalf("unfiltered logical totals: %s %#v %v", query, args, err)
	}
}

func TestPersonalSQLCursorUsesAllTiesAndNaturalNumberKey(t *testing.T) {
	boundary := &PersonalPosition{SetDate: "2020-01-01", SetCode: "SET", Number: "107a", Color: 2, Mana: 2.5, Name: "same", UUID: "00000000-0000-4000-8000-000000000001"}
	for _, order := range []string{"set", "color", "mana_value", "name"} {
		query, args, err := personalPage(personalColumns(personalBase("wishlist_item")), order, boundary, 50).ToSQL()
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(query, "r.printing_uuid > ?") || !strings.Contains(query, "r.printing_uuid ASC LIMIT ?") || args[len(args)-1] != int64(51) || strings.Contains(query, "OFFSET") {
			t.Fatalf("missing unique boundary or lookahead: %s %#v", query, args)
		}
		if order == "set" {
			for _, fragment := range []string{"COALESCE(DATE_FORMAT(r.set_released_at, '%Y-%m-%d'),'') < ?", "r.set_code COLLATE utf8mb4_bin = ?", "r.collector_sort_key > ?", "r.collector_sort_key = ?"} {
				if !strings.Contains(query, fragment) {
					t.Fatalf("missing mixed-direction cursor %q: %s", fragment, query)
				}
			}
			found := false
			for _, arg := range args {
				if bytes, ok := arg.([]byte); ok && reflect.DeepEqual(bytes, collectorSortKey("107a")) {
					found = true
				}
			}
			if !found {
				t.Fatalf("collector boundary is not binary: %#v", args)
			}
		} else if !strings.Contains(query, "CAST(LOWER(r.name) AS BINARY) = ?") {
			t.Fatalf("same-name ties can be lost: %s", query)
		}
		first, _, err := personalPage(personalColumns(personalBase("wishlist_item")), order, nil, 50).ToSQL()
		if err != nil || strings.Contains(first, " WHERE ") {
			t.Fatalf("first page has a cursor filter: %s %v", first, err)
		}
	}
}
