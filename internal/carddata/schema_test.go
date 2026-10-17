package carddata

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

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

func TestRulingsSpecAndEnglishFallback(t *testing.T) {
	var rulingsSpec sourceSpec
	for _, candidate := range sourceSpecs {
		if candidate.table == tableScryfallOracleRuling {
			rulingsSpec = candidate
			break
		}
	}
	if rulingsSpec.file != "rulings.jsonl" || !rulingsSpec.standardJSON {
		t.Fatalf("rulings spec = %#v", rulingsSpec)
	}
	line := []byte(`{"object":"ruling","oracle_id":"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa","source":"wotc","published_at":"2025-02-07","comment":"They’re affected."}`)
	obj, normalized, err := parseObject(line, rulingsSpec)
	if err != nil {
		t.Fatalf("parseObject() error = %v", err)
	}
	if normalized {
		t.Fatal("standard JSON input must not use legacy escape normalization")
	}
	values, err := rowValues(obj, rulingsSpec)
	if err != nil {
		t.Fatalf("rowValues() error = %v", err)
	}
	if values[0] != nil || values[2] != rulingKey("They're affected.") {
		t.Fatalf("mapping values = %#v", values)
	}
	derived, err := deriveRows(rulingsSpec, obj, 1)
	if err != nil {
		t.Fatalf("deriveRows() error = %v", err)
	}
	row := derived[tableRulingTranslation][0].values
	if row[0] != values[2] || row[1] != nil || row[3] != nil || row[6] != "2025-02-07" {
		t.Fatalf("English fallback row = %#v", row)
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
	if !rulingSpec.allowExtraRows || strings.Join(rulingSpec.key, ",") != "ruling_key" {
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

func TestScryfallSetDerivationIncludesReleaseDate(t *testing.T) {
	obj := map[string]json.RawMessage{
		"uuid":          json.RawMessage(`"11111111-1111-1111-1111-111111111111"`),
		"keywords":      json.RawMessage(`[]`),
		"frame_effects": json.RawMessage(`[]`),
		"promo_types":   json.RawMessage(`[]`),
		"type_line":     json.RawMessage(`"Creature — Elf"`),
		"set_id":        json.RawMessage(`"22222222-2222-2222-2222-222222222222"`),
		"set_code":      json.RawMessage(`"tst"`),
		"set_name":      json.RawMessage(`"Test Set"`),
		"set_type":      json.RawMessage(`"expansion"`),
		"released_at":   json.RawMessage(`"2026-10-17"`),
	}

	derived, err := deriveScryfallRows(obj, 3)
	if err != nil {
		t.Fatalf("deriveScryfallRows() error = %v", err)
	}
	setRows := derived[tableScryfallSet]
	if len(setRows) != 1 {
		t.Fatalf("set rows = %#v", setRows)
	}
	want := []any{"22222222-2222-2222-2222-222222222222", "tst", "Test Set", "expansion", "2026-10-17", int64(0)}
	if !reflect.DeepEqual(setRows[0].values, want) {
		t.Fatalf("set row = %#v, want %#v", setRows[0].values, want)
	}
}

func TestScryfallCardStoresProducedManaAsStringArray(t *testing.T) {
	for _, candidate := range specs {
		if candidate.table != tableScryfallCard {
			continue
		}
		for _, column := range candidate.columns {
			if column.name == "produced_mana" {
				if column.kind != kindStringArray || column.ddl != "VARCHAR(64) NULL" {
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
		if column.ddl != "DECIMAL(12,4) NULL" || column.kind != kindDerivedNumber {
			t.Fatalf("%s column = %#v", column.name, column)
		}
		wantColumns[column.name] = true
	}
	for name, found := range wantColumns {
		if !found {
			t.Errorf("%s column is missing", name)
		}
	}

	wantIndexes := map[string]bool{
		"power_numeric":     false,
		"toughness_numeric": false,
		"loyalty_numeric":   false,
		"pt_numeric":        false,
	}
	for _, index := range cardSpec.indexes {
		if len(index.columns) == 1 {
			if _, ok := wantIndexes[index.columns[0]]; ok {
				wantIndexes[index.columns[0]] = true
			}
		}
	}
	for name, found := range wantIndexes {
		if !found {
			t.Errorf("%s index is missing", name)
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
