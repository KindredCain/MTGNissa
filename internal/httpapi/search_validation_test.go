package httpapi

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	"mtgnissa/internal/model"
)

func TestSearchParametersRequireQueryAndRejectRepeatedScalars(t *testing.T) {
	for _, raw := range []string{"", "q=+", "q=%00", "q=a&q=b", "q=a&cursor=x&cursor=y", "q=a&cursor=%00"} {
		query, _ := url.ParseQuery(raw)
		if _, err := parseSearchParams(query); err == nil {
			t.Errorf("accepted invalid query %s", raw)
		}
	}
	if _, err := parseSearchParams(url.Values{"q": {strings.Repeat("妮", 513)}}); err == nil {
		t.Fatal("overlength query accepted")
	}
	params, err := parseSearchParams(url.Values{"q": {" c:r lang:ja "}})
	if err != nil || params.Query != " c:r lang:ja " {
		t.Fatalf("query text was rewritten before parsing: %#v error=%v", params, err)
	}
	if _, err := parseSearchParams(url.Values{"q": {string([]byte{0xff})}}); err == nil {
		t.Fatal("invalid UTF-8 accepted")
	}
}

func TestSearchDTOAndRouteWithoutSendingRequests(t *testing.T) {
	raw, err := json.Marshal(newSearchPage(model.SearchPage{Input: "function:tutor", Mode: "syntax", ExecutedQuery: `otag:"tutor"`}))
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{`"items":[]`, `"total":0`, `"next_cursor":null`, `"mode":"syntax"`, `"input":"function:tutor"`} {
		if !strings.Contains(string(raw), field) {
			t.Errorf("missing %s in %s", field, raw)
		}
	}
	router := New(Dependencies{})
	for _, route := range router.Routes() {
		if route.Method == "GET" && route.Path == "/api/v1/search" {
			return
		}
	}
	t.Fatal("global search route is missing")
}
