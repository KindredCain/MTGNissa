package httpapi

import (
	"net/url"
	"testing"
)

func TestDeckSearchMatchModeParameters(t *testing.T) {
	for _, test := range []struct {
		Query string
		Exact bool
	}{
		{"q=Lightning+Bolt", false},
		{"q=Lightning+Bolt&exact=false", false},
		{"q=Lightning+Bolt&exact=true&cursor=next", true},
		{"exact=true", true},
	} {
		query, _ := url.ParseQuery(test.Query)
		params, err := parseDeckSearchQuery(query)
		if err != nil || params.Exact != test.Exact || params.Query != query.Get("q") || params.Cursor != query.Get("cursor") {
			t.Fatalf("query=%s params=%+v error=%v", test.Query, params, err)
		}
	}
	for _, raw := range []string{"exact=1", "exact=TRUE", "exact=", "exact=true&exact=false", "q=a&q=b", "cursor=a&cursor=b"} {
		query, _ := url.ParseQuery(raw)
		if _, err := parseDeckSearchQuery(query); err == nil {
			t.Fatalf("invalid search parameters accepted: %s", raw)
		}
	}
	params, err := parseDeckSearchQuery(url.Values{"q": {"  Lightning Bolt  "}})
	if err != nil || params.Query != "Lightning Bolt" {
		t.Fatalf("name was not trimmed: %+v error=%v", params, err)
	}
}
