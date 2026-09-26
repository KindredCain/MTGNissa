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
