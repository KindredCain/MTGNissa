package carddata

import (
	"reflect"
	"testing"
)

func TestUniqueStringsIgnoresCaseAndWhitespace(t *testing.T) {
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
