package httpapi

import (
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"testing"

	"mtgnissa/internal/apperror"
	"mtgnissa/internal/tokens"
)

func TestParseTokenSetCode(t *testing.T) {
	for _, test := range []struct {
		query string
		code  apperror.Code
	}{
		{"", apperror.CodeValidationFailed},
		{"set_code=+", apperror.CodeValidationFailed},
		{"set_code=TKLD&set_code=TKHM", apperror.CodeInvalidRequest},
		{"set_code=%00", apperror.CodeValidationFailed},
		{"set_code=" + strings.Repeat("a", 17), apperror.CodeValidationFailed},
	} {
		query, err := url.ParseQuery(test.query)
		if err != nil {
			t.Fatal(err)
		}
		_, err = parseTokenSetCode(query)
		var appErr *apperror.Error
		if !errors.As(err, &appErr) || appErr.Code != test.code {
			t.Fatalf("query %q: error = %v", test.query, err)
		}
	}
	code, err := parseTokenSetCode(url.Values{"set_code": {" TKLD "}})
	if err != nil || code != "TKLD" {
		t.Fatalf("code = %q, error = %v", code, err)
	}
}

func TestTokenDTOHasEmptyItemsAndNullableImages(t *testing.T) {
	response := newTokenListResponse(tokens.List{Set: tokens.Set{Code: "TKLD"}})
	if response.Items == nil || response.Total != 0 || response.Set.ReleasedAt != nil {
		t.Fatalf("empty response = %#v", response)
	}
	response = newTokenListResponse(tokens.List{Items: []tokens.Token{{UUID: "token"}}})
	encoded, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"front_url":null,"back_url":null,"has_distinct_back":false`) || strings.Contains(string(encoded), "printing_uuid") {
		t.Fatalf("incorrect Token image or identity fields: %s", encoded)
	}
}
