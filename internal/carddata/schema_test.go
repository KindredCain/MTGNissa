package carddata

import (
	"encoding/json"
	"strings"
	"testing"
)

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
	var rulingsSpec spec
	for _, candidate := range specs {
		if candidate.table == "scryfall_oracle_ruling" {
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
	row := derived["zhs_ruling"][0].values
	if row[0] != values[2] || row[1] != nil || row[3] != nil || row[6] != "2025-02-07" {
		t.Fatalf("English fallback row = %#v", row)
	}
}

func TestZHSRulingColumnMapping(t *testing.T) {
	var rulingSpec spec
	for _, candidate := range specs {
		if candidate.table == "zhs_ruling" {
			rulingSpec = candidate
			break
		}
	}
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
	values, err := rowValues(obj, rulingSpec)
	if err != nil {
		t.Fatalf("rowValues() error = %v", err)
	}
	if values[0] != rulingKey("Example") || values[1] != "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb" || values[4] != "official" || values[5] != int64(9) {
		t.Fatalf("zhs ruling values = %#v", values)
	}
}

func TestProducedManaNullRemainsNull(t *testing.T) {
	values, err := rowValues(map[string]json.RawMessage{
		"produced_mana": json.RawMessage("null"),
	}, spec{columns: []columnSpec{
		col("produced_mana", "VARCHAR(64) NULL", kindStringArray),
	}})
	if err != nil {
		t.Fatalf("rowValues() error = %v", err)
	}
	if values[0] != nil {
		t.Fatalf("rowValues() produced_mana = %#v, want nil", values[0])
	}
}

func TestScryfallCardStoresProducedManaAsStringArray(t *testing.T) {
	for _, candidate := range specs {
		if candidate.table != "scryfall_card" {
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
		case "scryfall_card":
			cardSpec = candidate
		case "zhs_oracle":
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
		if candidate.table == "scryfall_card_artist" || candidate.table == "zhs_oracle_former_name" || candidate.table == "scryfall_card_mana_symbol" {
			t.Fatalf("obsolete derived table still exists: %s", candidate.table)
		}
	}
}

func TestOracleTagSpecAndDerivedRows(t *testing.T) {
	var tagSpec spec
	for _, candidate := range specs {
		if candidate.table == "oracle_tag" {
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
	relations := derived["oracle_tag_relation"]
	if len(relations) != 2 || relations[0].values[0] != "parent-a" || relations[0].values[1] != values[0] || relations[1].values[0] != values[0] || relations[1].values[1] != "child-a" {
		t.Fatalf("oracle tag relations = %#v", relations)
	}
	taggings := derived["oracle_tagging"]
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
