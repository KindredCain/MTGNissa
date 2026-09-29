package carddata

import (
	"strings"
	"testing"
)

func TestAssociationSpecsReferenceDefinedColumns(t *testing.T) {
	tables := make(map[string]map[string]struct{})
	for _, table := range allTableSpecs() {
		columns := make(map[string]struct{}, len(table.columns))
		for _, column := range table.columns {
			columns[column.name] = struct{}{}
		}
		tables[table.table] = columns
	}
	for _, association := range associationSpecs {
		assertAssociationColumn(t, tables, association.childTable, association.childColumn)
		assertAssociationColumn(t, tables, association.parentTable, association.parentColumn)
	}
}

func TestAssociationCheckSQLUsesTemporaryTablesAndStopsAtFirstOrphan(t *testing.T) {
	association := associationSpec{
		childTable:   "zhs_card",
		childColumn:  "card_id",
		parentTable:  "scryfall_card",
		parentColumn: "uuid",
	}
	tableNames := map[string]string{
		"zhs_card":      "zhs_card__new_test",
		"scryfall_card": "scryfall_card__new_test",
	}
	query := associationCheckSQL(association, tableNames)
	for _, fragment := range []string{
		"FROM `zhs_card__new_test` AS child",
		"LEFT JOIN `scryfall_card__new_test` AS parent",
		"parent.`uuid` = child.`card_id`",
		"child.`card_id` IS NOT NULL",
		"parent.`uuid` IS NULL LIMIT 1",
	} {
		if !strings.Contains(query, fragment) {
			t.Fatalf("associationCheckSQL() = %q, missing %q", query, fragment)
		}
	}
}

func assertAssociationColumn(t *testing.T, tables map[string]map[string]struct{}, table, column string) {
	t.Helper()
	columns, ok := tables[table]
	if !ok {
		t.Fatalf("association table %q is not defined", table)
	}
	if _, ok := columns[column]; !ok {
		t.Fatalf("association column %s.%s is not defined", table, column)
	}
}
