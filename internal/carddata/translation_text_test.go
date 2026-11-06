package carddata

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTranslationNewlinesDuringImport(t *testing.T) {
	for _, test := range []struct {
		name  string
		input string
		want  string
	}{
		{"Nissa abilities", `+1: first\n-3: second\n-6: third`, "+1: first\n-3: second\n-6: third"},
		{"Sarkhan mixed", "first\\nsecond\r\n", "first\nsecond\r\n"},
		{"Magical Hacker", `y0|_| /\r3 @ IVI0/\/$+3|2`, `y0|_| /\r3 @ IVI0/\/$+3|2`},
		{"Bog Wraith", "verse\\\r\nverse", "verse\\\r\nverse"},
		{"blank paragraph", `first\n\nsecond`, "first\n\nsecond"},
		{"existing newlines", "first\r\nsecond\nthird\r", "first\r\nsecond\nthird\r"},
		{"other escapes", `\r \t \u4e00 \" \/`, `\r \t \u4e00 \" \/`},
		{"paired backslashes", `\\n`, `\\n`},
		{"paired backslashes and marker", `\\\n`, "\\\\\n"},
		{"trailing backslash", `last\`, `last\`},
		{"empty", "", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			encoded, err := json.Marshal(map[string]string{"card_id": "card", "text": test.input})
			if err != nil {
				t.Fatal(err)
			}
			// Reproduce the extra file escape layer before exercising the
			// existing JSON parser and the shared import conversion together.
			upstream := strings.ReplaceAll(string(encoded), `\`, `\\`)
			input := sourceSpec{table: tableCardTranslation, required: []string{"card_id"}}
			obj, _, err := parseObject([]byte(upstream), input)
			if err != nil {
				t.Fatal(err)
			}
			values, err := rowValues(obj, input)
			if err != nil {
				t.Fatal(err)
			}
			for i, column := range mustTableSpec(input.table).columns {
				if column.name == "text" && values[i] != test.want {
					t.Fatalf("imported text = %q, want %q", values[i], test.want)
				}
			}
		})
	}
}

func TestTranslationNewlineFieldScope(t *testing.T) {
	for _, test := range []struct {
		table  tableName
		column string
		want   string
	}{
		{tableCardTranslation, "text", "first\nsecond"},
		{tableCardTranslation, "flavor_text", "first\nsecond"},
		{tableOracleTranslation, "translated_text", "first\nsecond"},
		{tableFlavorTranslation, "translated_flavor_text", "first\nsecond"},
		{tableRulingTranslation, "translation", "first\nsecond"},
		{tableOracleTranslation, "oracle_text", `first\nsecond`},
		{tableFlavorTranslation, "flavor_text", `first\nsecond`},
		{tableRulingTranslation, "comment", `first\nsecond`},
		{tableCardTranslation, "name", `first\nsecond`},
		{tableScryfallCard, "oracle_text", `first\nsecond`},
	} {
		projection := spec{table: test.table, columns: []columnSpec{col(test.column, "LONGTEXT NULL", kindString)}}
		obj := map[string]json.RawMessage{test.column: json.RawMessage(`"first\\nsecond"`)}
		values, err := rowValuesFor(obj, projection, sourceSpec{table: test.table})
		if err != nil {
			t.Fatal(err)
		}
		if values[0] != test.want {
			t.Errorf("%s.%s = %q, want %q", test.table, test.column, values[0], test.want)
		}
		obj[test.column] = json.RawMessage(`null`)
		values, err = rowValuesFor(obj, projection, sourceSpec{table: test.table})
		if err != nil || values[0] != nil {
			t.Errorf("%s.%s null = %v, error = %v", test.table, test.column, values, err)
		}
	}
}
