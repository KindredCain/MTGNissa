package httpapi

import (
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"mtgnissa/internal/apperror"
	"mtgnissa/internal/model"
)

func TestOperationLogParametersAndDateRanges(t *testing.T) {
	params, err := parseOperationSummaryParams(url.Values{})
	if err != nil || params.Page != 1 || params.StartDate != "" || params.EndDate != "" {
		t.Fatalf("defaults=%+v error=%v", params, err)
	}
	for _, raw := range []string{"page=0", "page=-1", "page=1.5", "page=", "page=99999999999999999999", "page=1&page=2", "start_date=2026-02-29", "start_date=2026-1-01", "end_date=x", "start_date=2026-01-01&start_date=2026-01-02"} {
		query, _ := url.ParseQuery(raw)
		if _, err := parseOperationSummaryParams(query); err == nil {
			t.Errorf("accepted summary query %s", raw)
		}
	}
	_, err = parseOperationSummaryParams(url.Values{"start_date": {"2026-11-10"}, "end_date": {"2026-11-09"}})
	var appErr *apperror.Error
	if !errors.As(err, &appErr) || appErr.Code != apperror.CodeInvalidDateRange {
		t.Fatalf("range error=%v", err)
	}
	for _, raw := range []string{"", "operation_date=2026-11-09", "action_type=add_card", "operation_date=2026-11-09&action_type=unknown", "operation_date=2026-11-09&action_type=add_card&action_type=remove_card"} {
		query, _ := url.ParseQuery(raw)
		if _, err := parseOperationLogParams(query); err == nil {
			t.Errorf("accepted detail query %s", raw)
		}
	}
	for code := uint8(1); code <= 8; code++ {
		params, err := parseOperationLogParams(url.Values{"operation_date": {"2026-11-09"}, "action_type": {model.OperationActionName(code)}})
		if err != nil || params.Page != 1 || model.OperationActionCode(params.Action) != code {
			t.Fatalf("code=%d params=%+v error=%v", code, params, err)
		}
	}
}

func TestOperationLogDTOAndRoutesWithoutRequests(t *testing.T) {
	for _, value := range []any{newOperationSummary(model.OperationSummaryPage{Page: 1}), newOperationLogPage(model.OperationLogPage{Page: 1})} {
		raw, err := json.Marshal(value)
		if err != nil || !strings.Contains(string(raw), `"items":[]`) || !strings.Contains(string(raw), `"page_size":10`) || !strings.Contains(string(raw), `"total_pages":0`) {
			t.Fatalf("empty response=%s error=%v", raw, err)
		}
	}
	page := newOperationLogPage(model.OperationLogPage{Page: 2, Total: 11, Items: []model.OperationItem{
		{ID: 9007199254740993, Action: "add_wishlist", OccurredAt: time.Date(2026, 11, 9, 0, 0, 0, 0, time.FixedZone("UTC+8", 8*60*60)), Card: &model.OperationCardSnapshot{PrintingUUID: "snapshot", Name: "historic name", LanguageDisplayName: "historical language"}},
		{ID: 2, Action: "delete_deck", Deck: &model.OperationDeckSnapshot{ID: 77, Name: "deleted deck"}},
	}})
	raw, err := json.Marshal(page)
	for _, fragment := range []string{`"operation_id":"9007199254740993"`, `"occurred_at":"2026-11-08T16:00:00Z"`, `"finish":null`, `"quantity_delta":null`, `"deck_id":"77"`, `"total_pages":2`} {
		if err != nil || !strings.Contains(string(raw), fragment) {
			t.Fatalf("missing %s in %s error=%v", fragment, raw, err)
		}
	}
	if page.Items[0].Deck != nil || page.Items[1].Card != nil {
		t.Fatal("snapshot discriminators contain unrelated objects")
	}
	routes := make(map[string]bool)
	for _, route := range New(Dependencies{}).Routes() {
		if route.Method == "GET" {
			routes[route.Path] = true
		}
	}
	if !routes["/api/v1/operation_logs"] || !routes["/api/v1/operation_logs/summary"] {
		t.Fatal("operation log routes missing")
	}
}
