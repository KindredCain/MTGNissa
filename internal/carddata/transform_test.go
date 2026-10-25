package carddata

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestSetTranslationAllowsUntranslatedNames(t *testing.T) {
	input := sourceSpec{table: tableSetTranslation, required: []string{"set_id"}}
	for _, record := range []string{
		`{"set_id":"95cd2c93-807b-48ea-9eba-37185ad49ae1","code":"YFRA","name":null,"source":null,"stage":0}`,
		`{"set_id":"95cd2c93-807b-48ea-9eba-37185ad49ae1","code":"YFRA","source":null,"stage":0}`,
	} {
		obj, _, err := parseObject([]byte(record), input)
		if err != nil {
			t.Fatal(err)
		}
		values, err := rowValues(obj, input)
		if err != nil {
			t.Fatalf("untranslated Set was rejected: %v", err)
		}
		want := []any{"95cd2c93-807b-48ea-9eba-37185ad49ae1", "YFRA", nil, nil, int64(0)}
		if !reflect.DeepEqual(values, want) {
			t.Fatalf("values = %#v, want %#v", values, want)
		}
	}
}

func TestAllTranslationContentsAllowMissingAndNull(t *testing.T) {
	tests := []struct {
		table  tableName
		record string
		fields []string
	}{
		{tableCardTranslation, `{"card_id":"card"}`, []string{"name", "face_name", "flavor_name", "type_line", "text", "flavor_text"}},
		{tableFlavorTranslation, `{"flavor_id":"flavor"}`, []string{"translated_flavor_name", "translated_flavor_text"}},
		{tableOracleTranslation, `{"face_oracle_id":"oracle"}`, []string{"translated_name", "translated_type", "translated_text"}},
		{tableRulingTranslation, `{"ruling":"ruling","comment":"Original ruling"}`, []string{"translation"}},
		{tableSetTranslation, `{"set_id":"set","code":"SET"}`, []string{"name"}},
		{tableTypeTranslation, `{"type_name":"Creature","type_type":"type"}`, []string{"translation"}},
	}
	for _, test := range tests {
		t.Run(string(test.table), func(t *testing.T) {
			var input sourceSpec
			for _, source := range sourceSpecs {
				if source.table == test.table {
					input = source
					break
				}
			}
			if input.file == "" {
				t.Fatal("translation source is not configured")
			}
			for _, explicitNull := range []bool{false, true} {
				obj, err := decodeObject(json.RawMessage(test.record))
				if err != nil {
					t.Fatal(err)
				}
				if explicitNull {
					for _, field := range test.fields {
						obj[field] = json.RawMessage(`null`)
					}
				}
				raw, err := json.Marshal(obj)
				if err != nil {
					t.Fatal(err)
				}
				obj, _, err = parseObject(raw, input)
				if err != nil {
					t.Fatal(err)
				}
				values, err := rowValues(obj, input)
				if err != nil {
					t.Fatalf("untranslated content was rejected (explicit null=%v): %v", explicitNull, err)
				}
				for i, column := range mustTableSpec(test.table).columns {
					for _, field := range test.fields {
						if column.name == field && values[i] != nil {
							t.Errorf("missing %s = %#v, want nil", field, values[i])
						}
					}
				}
			}
		})
	}
}

func TestTransformUniqueStringsIgnoresCaseAndWhitespace(t *testing.T) {
	got := uniqueStrings([]string{
		"Family gathering",
		"Food",
		"Family Gathering",
		" Scry ",
		"",
		"food",
	})
	want := []string{"Family gathering", "Food", "Scry"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("uniqueStrings() = %#v, want %#v", got, want)
	}
}

func TestRelationalKeyIgnoresStringCaseAndWhitespace(t *testing.T) {
	table := spec{
		key: []string{"type_name", "type_type"},
		columns: []columnSpec{
			col("type_name", "VARCHAR(255) NOT NULL", kindString),
			col("type_type", "VARCHAR(64) NOT NULL", kindString),
		},
	}

	lowerKey, err := relationalKey([]any{"family gathering", "keyword"}, table)
	if err != nil {
		t.Fatalf("relationalKey(lowercase): %v", err)
	}
	upperKey, err := relationalKey([]any{"  Family Gathering\t", " KEYWORD\r\n"}, table)
	if err != nil {
		t.Fatalf("relationalKey(mixed case): %v", err)
	}
	if lowerKey != upperKey {
		t.Fatalf("case and whitespace variants produced different keys: %q != %q", lowerKey, upperKey)
	}
}

func TestCardPrintingRowsPreferForeignDataButKeepOuterUUIDForBaseLanguage(t *testing.T) {
	obj, err := decodeObject(json.RawMessage(`{
		"language":"English",
		"name":"Base Name",
		"type":"Creature",
		"text":"Base text",
		"foreignData":[
			{"uuid":"foreign-en","language":"English","name":"Foreign English","type":"Creature — Test","text":"Foreign text","identifiers":{"scryfallId":"foreign-en-sid"}},
			{"uuid":"foreign-ja","language":"Japanese","name":"Japanese","type":"Creature","text":"Japanese text","identifiers":{"scryfallId":"foreign-ja-sid"}}
		]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	rows, err := cardPrintingRows(obj, "outer-uuid", "outer-sid", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("printing row count = %d, want 2", len(rows))
	}
	english := rows[0].values
	if english[0] != "outer-uuid" || english[1] != "outer-uuid" || english[2] != "en" || english[3] != "foreign-en-sid" || english[4] != "Foreign English" {
		t.Fatalf("English printing row = %#v", english)
	}
	if rows[1].values[0] != "foreign-ja" || rows[1].values[2] != "ja" {
		t.Fatalf("Japanese printing row = %#v", rows[1].values)
	}
}

func TestFaceIndexTreatsMeldAsIndependentCard(t *testing.T) {
	obj := map[string]json.RawMessage{
		"layout": json.RawMessage(`"meld"`),
		"side":   json.RawMessage(`"a"`),
	}
	got, err := faceIndex(obj)
	if err != nil {
		t.Fatal(err)
	}
	if got != -1 {
		t.Fatalf("faceIndex() = %d, want -1", got)
	}
}

func TestSparseBooleanDefaultsToFalse(t *testing.T) {
	column := col("content_warning", "BOOLEAN NOT NULL", kindBoolean)
	got, err := mtgjsonColumnValue(map[string]json.RawMessage{}, "hasContentWarning", column)
	if err != nil {
		t.Fatal(err)
	}
	if got != false {
		t.Fatalf("missing sparse boolean = %#v, want false", got)
	}
}

func TestLegalityUsesDocumentedPriority(t *testing.T) {
	state := newImportState(&loadPlan{})
	for _, status := range []string{"Banned", "Restricted", "Legal"} {
		obj := map[string]json.RawMessage{
			"legalities": json.RawMessage(`{"oldschool":"` + status + `"}`),
		}
		if err := collectLegality(state, obj, "oracle"); err != nil {
			t.Fatal(err)
		}
	}
	if got := state.legalityByOracle["oracle"]["oldschool"]; got != "Legal" {
		t.Fatalf("aggregated oldschool status = %q, want Legal", got)
	}
}
