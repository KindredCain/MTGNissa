package carddata

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestCardSchemaMatchesDictionary(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "docs", "card-data-dictionary.md"))
	if err != nil {
		t.Fatalf("read card dictionary: %v", err)
	}
	document := string(data)
	headings := regexp.MustCompile("(?m)^### `([^`]+)`\\s*$").FindAllStringSubmatchIndex(document, -1)
	columnPattern := regexp.MustCompile("(?m)^\\| (\\w+) \\| ([^|]+) \\|")
	tables := make(map[tableName]spec)
	for _, table := range allTableSpecs() {
		tables[table.table] = table
	}
	for i, heading := range headings {
		name := tableName(document[heading[2]:heading[3]])
		table, ok := tables[name]
		if !ok {
			t.Errorf("dictionary table %q has no schema", name)
			continue
		}
		delete(tables, name)
		end := len(document)
		if i+1 < len(headings) {
			end = headings[i+1][0]
		}
		section := document[heading[1]:end]
		t.Run(string(name), func(t *testing.T) {
			columns := make(map[string]string)
			for _, column := range table.columns {
				columns[column.name] = column.ddl
			}
			for _, row := range columnPattern.FindAllStringSubmatch(section, -1) {
				want := strings.TrimSpace(strings.ReplaceAll(row[2], "`", ""))
				if !strings.HasSuffix(want, " NULL") {
					want += " NOT NULL"
				}
				if got, ok := columns[row[1]]; !ok || got != want {
					t.Errorf("column %s definition = %q, want dictionary definition %q", row[1], got, want)
				}
				delete(columns, row[1])
			}
			for column := range columns {
				t.Errorf("schema column %q is missing from dictionary", column)
			}
			if strings.Contains(section, "`bit_value` 唯一") {
				for _, index := range table.indexes {
					if index.unique && reflect.DeepEqual(index.columns, []string{"bit_value"}) {
						return
					}
				}
				t.Error("dictionary requires a unique bit_value index")
			}
		})
	}
	for name := range tables {
		t.Errorf("schema table %q is missing from dictionary", name)
	}
}

func TestBuiltInDictionaryStringsFitColumns(t *testing.T) {
	lengthPattern := regexp.MustCompile(`^(?:VAR)?CHAR\((\d+)\)`)
	for name, rows := range dictionaryRows() {
		t.Run(string(name), func(t *testing.T) {
			table := mustTableSpec(name)
			for _, row := range rows {
				if len(row.values) != len(table.columns) {
					t.Fatalf("dictionary row has %d values, want %d", len(row.values), len(table.columns))
				}
				for i, column := range table.columns {
					value := row.values[i]
					if value == nil {
						if strings.Contains(column.ddl, "NOT NULL") {
							t.Errorf("required dictionary column %s is null", column.name)
						}
						continue
					}
					text, ok := value.(string)
					match := lengthPattern.FindStringSubmatch(column.ddl)
					if !ok || len(match) != 2 {
						continue
					}
					limit, err := strconv.Atoi(match[1])
					if err != nil {
						t.Fatalf("parse column %s length: %v", column.name, err)
					}
					if length := utf8.RuneCountInString(text); length > limit {
						t.Errorf("dictionary column %s value has %d characters, limit %d", column.name, length, limit)
					}
				}
			}
		})
	}
}

func TestLanguageDictionaryDisplayNames(t *testing.T) {
	languages := dictionaryRows()[tableScryfallLanguage]
	if len(languages) != 19 {
		t.Fatalf("language row count = %d, want 19", len(languages))
	}

	wantNames := map[string]string{
		"en":  "英文",
		"ja":  "日文",
		"zhs": "简中",
		"zht": "繁中",
	}
	for _, language := range languages {
		if len(language.values) != 4 {
			t.Fatalf("language row = %#v, want code, name, display_name, sort_order", language.values)
		}
		code, ok := language.values[0].(string)
		if !ok {
			t.Fatalf("language code = %#v, want string", language.values[0])
		}
		displayName, ok := language.values[2].(string)
		if !ok || strings.TrimSpace(displayName) == "" {
			t.Errorf("language %q display_name = %#v, want non-empty string", code, language.values[2])
		}
		if want, exists := wantNames[code]; exists && displayName != want {
			t.Errorf("language %q display_name = %q, want %q", code, displayName, want)
		}
	}
}

func TestMTGJSONTableColumnCountsMatchDictionary(t *testing.T) {
	want := map[tableName]int{
		tableScryfallCard:         59,
		tableMTGNissaToken:        38,
		tableScryfallSet:          9,
		tableMTGNissaCardPrinting: 7,
		tableOracleLegality:       23,
		tableScryfallOracleRuling: 4,
		tableCardFlavorMapping:    2,
		tableCardOracleMapping:    2,
	}
	for table, count := range want {
		if got := len(mustTableSpec(table).columns); got != count {
			t.Errorf("%s column count = %d, want %d", table, got, count)
		}
	}
}

func TestCardUUIDBridgeIsNotPersistentSchema(t *testing.T) {
	for _, table := range allTableSpecs() {
		if strings.HasPrefix(string(table.table), "card_uuid_bridge") {
			t.Fatalf("task bridge table must not be a persistent schema table: %s", table.table)
		}
	}
}

func TestStringArrayColumnValue(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "single special value", raw: `["T"]`, want: "T"},
		{name: "multiple mana values", raw: `["W","U","B","R","G"]`, want: "W,U,B,R,G"},
		{name: "empty array", raw: `[]`, want: ""},
	}

	column := col("produced_mana", "VARCHAR(64) NULL", kindStringArray)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := columnValue(json.RawMessage(tt.raw), column)
			if err != nil {
				t.Fatalf("columnValue() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("columnValue() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDateTimeColumnValueConvertsToUTC(t *testing.T) {
	column := col("created_at", "DATETIME(6) NOT NULL", kindDateTime)
	got, err := columnValue(json.RawMessage(`"2026-09-16T01:32:04.857605+08:00"`), column)
	if err != nil {
		t.Fatalf("columnValue() error = %v", err)
	}
	want := "2026-09-15 17:32:04.857605"
	if got != want {
		t.Fatalf("columnValue() = %q, want %q", got, want)
	}
}

func TestRelationalKeyRejectsEmptyStringPrimaryKey(t *testing.T) {
	s := spec{
		key: []string{"id"},
		columns: []columnSpec{
			col("id", "CHAR(36) NOT NULL", kindString),
		},
	}
	_, err := relationalKey([]any{" \t"}, s)
	if err == nil || !strings.Contains(err.Error(), `primary key column "id" is empty`) {
		t.Fatalf("relationalKey() error = %v", err)
	}
}

func TestRulingKeyNormalizesTypography(t *testing.T) {
	curly := "They’re affected.\r\n“Example”"
	plain := "  They're affected.\n\"Example\"  "
	if rulingKey(curly) != rulingKey(plain) {
		t.Fatalf("equivalent ruling text produced different keys: %s != %s", rulingKey(curly), rulingKey(plain))
	}
	if len(rulingKey(curly)) != 64 {
		t.Fatalf("ruling key length = %d, want 64", len(rulingKey(curly)))
	}
}

func TestRulingsComeOnlyFromAllPrintings(t *testing.T) {
	for _, source := range sourceSpecs {
		if source.file == "rulings.jsonl" {
			t.Fatal("discarded rulings.jsonl is still an import source")
		}
	}
	obj, err := decodeObject(json.RawMessage(`{"rulings":[{"date":"2025-02-07","text":"They're affected."}]}`))
	if err != nil {
		t.Fatalf("decodeObject() error = %v", err)
	}
	state := newImportState(&loadPlan{})
	derived, err := rulingRows(obj, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", 1, state)
	if err != nil {
		t.Fatalf("rulingRows() error = %v", err)
	}
	ruling := derived[tableScryfallOracleRuling][0].values
	if ruling[0] != "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa" || ruling[1] != rulingKey("They're affected.") || ruling[2] != "2025-02-07" {
		t.Fatalf("oracle ruling row = %#v", ruling)
	}
}

func TestZHSRulingColumnMapping(t *testing.T) {
	var rulingSource sourceSpec
	for _, candidate := range sourceSpecs {
		if candidate.table == tableRulingTranslation {
			rulingSource = candidate
			break
		}
	}
	rulingSpec := mustTableSpec(rulingSource.table)
	if rulingSpec.allowExtraRows || strings.Join(rulingSpec.key, ",") != "ruling_key" {
		t.Fatalf("zhs ruling spec = %#v", rulingSpec)
	}
	obj := map[string]json.RawMessage{
		"ruling":            json.RawMessage(`"bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"`),
		"comment":           json.RawMessage(`"Example"`),
		"translation":       json.RawMessage(`"示例"`),
		"source":            json.RawMessage(`"official"`),
		"stage":             json.RawMessage(`9`),
		"last_published_at": json.RawMessage(`"2025-02-07"`),
		"extra":             json.RawMessage(`{"release_notes":{}}`),
	}
	values, err := rowValues(obj, rulingSource)
	if err != nil {
		t.Fatalf("rowValues() error = %v", err)
	}
	if values[0] != rulingKey("Example") || values[1] != "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb" || values[4] != "official" || values[5] != int64(9) {
		t.Fatalf("zhs ruling values = %#v", values)
	}
}

func TestProducedManaNullRemainsNull(t *testing.T) {
	values, err := rowValuesFor(map[string]json.RawMessage{
		"produced_mana": json.RawMessage("null"),
	}, spec{columns: []columnSpec{
		col("produced_mana", "VARCHAR(64) NULL", kindStringArray),
	}}, sourceSpec{})
	if err != nil {
		t.Fatalf("rowValuesFor() error = %v", err)
	}
	if values[0] != nil {
		t.Fatalf("rowValuesFor() produced_mana = %#v, want nil", values[0])
	}
}

func TestMTGJSONSetRowsIncludeTokenSet(t *testing.T) {
	releaseDate := "2026-10-17"
	set := mtgjsonSet{Code: "TST", Name: "Test Set", Type: "expansion", ReleaseDate: &releaseDate, Languages: []string{"English"}, Cards: make([]json.RawMessage, 2), Tokens: make([]json.RawMessage, 1)}
	derived, err := mtgjsonSetRows(set, "TTST", 3)
	if err != nil {
		t.Fatalf("mtgjsonSetRows() error = %v", err)
	}
	if len(derived) != 2 {
		t.Fatalf("set rows = %#v", derived)
	}
	want := []any{"TST", "Test Set", "2026-10-17", "expansion", int64(2), nil, "en", nil, nil}
	if !reflect.DeepEqual(derived[0].values, want) {
		t.Fatalf("set row = %#v, want %#v", derived[0].values, want)
	}
	if derived[1].values[0] != "TTST" || derived[1].values[3] != "token" || derived[1].values[6] != nil || derived[1].values[8] != "TST" {
		t.Fatalf("token set row = %#v", derived[1].values)
	}
}

func TestScryfallCardStoresProducedManaAsStringArray(t *testing.T) {
	for _, candidate := range specs {
		if candidate.table != tableScryfallCard {
			continue
		}
		for _, column := range candidate.columns {
			if column.name == "produced_mana" {
				if column.kind != kindStringArray || column.ddl != "VARCHAR(32) NULL" {
					t.Fatalf("produced_mana column = %#v", column)
				}
				return
			}
		}
		t.Fatal("produced_mana column not found")
	}
	t.Fatal("scryfall_card spec not found")
}

func TestScryfallCardNumericCombatColumns(t *testing.T) {
	cardSpec := mustTableSpec(tableScryfallCard)
	wantColumns := map[string]bool{
		"power_numeric":     false,
		"toughness_numeric": false,
		"loyalty_numeric":   false,
		"pt_numeric":        false,
	}
	for _, column := range cardSpec.columns {
		if _, ok := wantColumns[column.name]; !ok {
			continue
		}
		wantDDL := "DECIMAL(8,2) NULL"
		if column.name == "pt_numeric" {
			wantDDL = "DECIMAL(12,4) NULL"
		}
		if column.ddl != wantDDL || column.kind != kindDerivedNumber {
			t.Fatalf("%s column = %#v", column.name, column)
		}
		wantColumns[column.name] = true
	}
	for name, found := range wantColumns {
		if !found {
			t.Errorf("%s column is missing", name)
		}
	}
}

func TestDerivedCombatNumbers(t *testing.T) {
	tableSpec := spec{columns: []columnSpec{
		col("power", "VARCHAR(32) NULL", kindString),
		col("power_numeric", "DECIMAL(12,4) NULL", kindDerivedNumber),
		col("toughness", "VARCHAR(32) NULL", kindString),
		col("toughness_numeric", "DECIMAL(12,4) NULL", kindDerivedNumber),
		col("loyalty", "VARCHAR(32) NULL", kindString),
		col("loyalty_numeric", "DECIMAL(12,4) NULL", kindDerivedNumber),
		col("pt_numeric", "DECIMAL(12,4) NULL", kindDerivedNumber),
	}}
	values, err := rowValuesFor(map[string]json.RawMessage{
		"power":     json.RawMessage(`" 1.25 "`),
		"toughness": json.RawMessage(`"2.5"`),
		"loyalty":   json.RawMessage(`"*"`),
	}, tableSpec, sourceSpec{})
	if err != nil {
		t.Fatalf("rowValuesFor() error = %v", err)
	}
	if values[1] != "1.25" || values[3] != "2.5" || values[5] != nil || values[6] != "3.7500" {
		t.Fatalf("derived numeric values = %#v", values)
	}
}

func TestRawJSONObjectColumnValue(t *testing.T) {
	column := col("extra", "LONGTEXT NULL", kindRawJSONText)
	got, err := columnValue(json.RawMessage(`{"release_notes": {"note": {"card_names": ["Example"]}}}`), column)
	if err != nil {
		t.Fatalf("columnValue() error = %v", err)
	}
	want := `{"release_notes":{"note":{"card_names":["Example"]}}}`
	if got != want {
		t.Fatalf("columnValue() = %q, want %q", got, want)
	}
}

func TestMetadataArraysStayOnMainTables(t *testing.T) {
	var cardSpec, oracleSpec spec
	for _, candidate := range specs {
		switch candidate.table {
		case tableScryfallCard:
			cardSpec = candidate
		case tableOracleTranslation:
			oracleSpec = candidate
		}
	}
	assertColumn := func(s spec, name, ddl string, kind columnKind) {
		t.Helper()
		for _, column := range s.columns {
			if column.name == name {
				if column.ddl != ddl || column.kind != kind {
					t.Fatalf("%s.%s = %#v", s.table, name, column)
				}
				return
			}
		}
		t.Fatalf("%s.%s is missing", s.table, name)
	}
	assertColumn(cardSpec, "artist_ids", "VARCHAR(2048) NULL", kindStringArray)
	assertColumn(oracleSpec, "former_names", "LONGTEXT NULL", kindRawJSONText)

	formerNames, err := columnValue(json.RawMessage(`["Name, the First","Second Name"]`), col("former_names", "LONGTEXT NULL", kindRawJSONText))
	if err != nil {
		t.Fatalf("former_names columnValue() error = %v", err)
	}
	if formerNames != `["Name, the First","Second Name"]` {
		t.Fatalf("former_names = %q", formerNames)
	}
	for _, candidate := range derivedSpecs {
		if candidate.table == "scryfall_card_artist" || candidate.table == "oracle_translation_former_name" || candidate.table == "scryfall_card_mana_symbol" {
			t.Fatalf("obsolete derived table still exists: %s", candidate.table)
		}
	}
}

func TestOracleTagSpecAndDerivedRows(t *testing.T) {
	var tagSpec sourceSpec
	for _, candidate := range sourceSpecs {
		if candidate.table == tableOracleTag {
			tagSpec = candidate
			break
		}
	}
	if tagSpec.file != "oracle-tags.jsonl" || !tagSpec.standardJSON {
		t.Fatalf("oracle tag spec = %#v", tagSpec)
	}
	line := []byte(`{"object":"tag","id":"00155182-3099-4742-be68-f8b4ea259d78","label":"tutor-creature-giant","slug":"tutor-creature-giant","type":"oracle","uri":"https://tagger.scryfall.com/tags/card/tutor-creature-giant","description":"Cards that tutor Giant cards.","parent_ids":["parent-a"],"child_ids":["child-a"],"aliases":["tutor-giant","giant-tutor"],"taggings":[{"oracle_id":"2445e58b-87ed-4ab2-8209-a5e1f566fba7","weight":"median"}]}`)
	obj, normalized, err := parseObject(line, tagSpec)
	if err != nil {
		t.Fatalf("parseObject() error = %v", err)
	}
	if normalized {
		t.Fatal("standard JSON input must not use legacy escape normalization")
	}
	values, err := rowValues(obj, tagSpec)
	if err != nil {
		t.Fatalf("rowValues() error = %v", err)
	}
	if values[0] != "00155182-3099-4742-be68-f8b4ea259d78" || values[2] != "Cards that tutor Giant cards." || values[3] != "tutor-giant,giant-tutor" {
		t.Fatalf("oracle tag values = %#v", values)
	}
	derived, err := deriveRows(tagSpec, obj, 7)
	if err != nil {
		t.Fatalf("deriveRows() error = %v", err)
	}
	relations := derived[tableOracleTagRelation]
	if len(relations) != 2 || relations[0].values[0] != "parent-a" || relations[0].values[1] != values[0] || relations[1].values[0] != values[0] || relations[1].values[1] != "child-a" {
		t.Fatalf("oracle tag relations = %#v", relations)
	}
	taggings := derived[tableOracleTagging]
	if len(taggings) != 1 || taggings[0].values[0] != "2445e58b-87ed-4ab2-8209-a5e1f566fba7" || taggings[0].values[1] != values[0] || taggings[0].values[2] != "median" {
		t.Fatalf("oracle taggings = %#v", taggings)
	}
}

func TestOracleTagRejectsIllustrationType(t *testing.T) {
	obj := map[string]json.RawMessage{
		"object":     json.RawMessage(`"tag"`),
		"id":         json.RawMessage(`"tag-a"`),
		"type":       json.RawMessage(`"illustration"`),
		"parent_ids": json.RawMessage(`[]`),
		"child_ids":  json.RawMessage(`[]`),
		"taggings":   json.RawMessage(`[]`),
	}
	_, err := deriveOracleTagRows(obj, 1)
	if err == nil || !strings.Contains(err.Error(), "expected oracle tag") {
		t.Fatalf("deriveOracleTagRows() error = %v", err)
	}
}
