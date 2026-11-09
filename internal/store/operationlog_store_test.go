package store

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"mtgnissa/internal/model"
)

func TestOperationSummarySQLUsesFixedOffsetAndUTCBounds(t *testing.T) {
	start := time.Date(2026, 11, 8, 16, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	for _, offset := range []int{8 * 3600, -5 * 3600, 5*3600 + 45*60} {
		query, args := operationSummarySQL(model.OperationRange{Start: &start, End: &end}, offset)
		for _, fragment := range []string{
			"DATE_ADD(l.occurred_at, INTERVAL ? SECOND)",
			"COUNT(*) AS operation_count", "l.occurred_at>=? AND l.occurred_at<?",
			"GROUP BY operation_date,l.action_type",
		} {
			if !strings.Contains(query, fragment) {
				t.Fatalf("missing %q: %s", fragment, query)
			}
		}
		if !reflect.DeepEqual(args, []any{offset, start, end}) {
			t.Fatalf("arguments=%#v", args)
		}
	}
	query, args := operationSummarySQL(model.OperationRange{}, 0)
	if !reflect.DeepEqual(args, []any{0}) || strings.Contains(query, "occurred_at>=") || strings.Contains(query, "CONVERT_TZ") {
		t.Fatalf("unbounded summary=%s %#v", query, args)
	}
}
