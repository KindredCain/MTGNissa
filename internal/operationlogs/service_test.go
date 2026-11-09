package operationlogs

import (
	"context"
	"errors"
	"testing"
	"time"

	"mtgnissa/internal/apperror"
	"mtgnissa/internal/model"
)

type logStoreStub struct {
	summaries []model.OperationSummary
	offset    int
	items     []model.OperationItem
	total     int64
	bounds    model.OperationRange
	action    uint8
	page      int
	err       error
}

func (s *logStoreStub) Summary(_ context.Context, bounds model.OperationRange, offset, page int) ([]model.OperationSummary, int64, error) {
	s.bounds, s.offset, s.page = bounds, offset, page
	return s.summaries, s.total, s.err
}

func (s *logStoreStub) List(_ context.Context, bounds model.OperationRange, action uint8, page int) ([]model.OperationItem, int64, error) {
	s.bounds, s.action, s.page = bounds, action, page
	return s.items, s.total, s.err
}

func TestSummaryPassesFixedOffsetBoundsAndSQLPage(t *testing.T) {
	location := time.FixedZone("UTC+8", 8*60*60)
	stub := &logStoreStub{summaries: []model.OperationSummary{{Date: "2026-11-09", Action: "add_card", Count: 2}}, total: 16}
	s := &Service{store: stub, location: location}
	page, err := s.Summary(context.Background(), model.OperationSummaryParams{StartDate: "2026-11-08", EndDate: "2026-11-09", Page: 1})
	if err != nil || page.Total != 16 || len(page.Items) != 1 || page.Items[0].Count != 2 || stub.offset != 8*3600 || stub.page != 1 {
		t.Fatalf("page=%+v error=%v", page, err)
	}
	if stub.bounds.Start.Format(time.RFC3339) != "2026-11-07T16:00:00Z" || stub.bounds.End.Format(time.RFC3339) != "2026-11-09T16:00:00Z" {
		t.Fatalf("bounds=%+v", stub.bounds)
	}
	page, err = s.Summary(context.Background(), model.OperationSummaryParams{Page: 2})
	if err != nil || stub.page != 2 || page.Page != 2 || stub.bounds.Start != nil || stub.bounds.End != nil {
		t.Fatalf("page=%+v bounds=%+v error=%v", page, stub.bounds, err)
	}
	stub.summaries = nil
	page, err = s.Summary(context.Background(), model.OperationSummaryParams{Page: 3})
	if err != nil || len(page.Items) != 0 || page.Items == nil || page.Total != 16 {
		t.Fatalf("out-of-range page=%+v error=%v", page, err)
	}
}

func TestSummaryAndDetailUseCurrentOffsetForHistoricalDays(t *testing.T) {
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	stub := &logStoreStub{items: []model.OperationItem{{ID: 42, Action: "delete_deck", Deck: &model.OperationDeckSnapshot{ID: 17, Name: "deleted deck snapshot"}}}, total: 11}
	s := &Service{store: stub, location: location}
	_, offset := time.Now().In(location).Zone()
	for _, date := range []string{"2026-03-08", "2026-11-01", "2026-01-15", "2026-07-15"} {
		if _, err := s.Summary(context.Background(), model.OperationSummaryParams{StartDate: date, EndDate: date, Page: 1}); err != nil {
			t.Fatal(err)
		}
		summaryBounds := stub.bounds
		page, err := s.List(context.Background(), model.OperationLogParams{Date: date, Action: "delete_deck", Page: 2})
		midnight, _ := time.Parse("2006-01-02", date)
		expected := midnight.Add(-time.Duration(offset) * time.Second)
		if err != nil || !stub.bounds.Start.Equal(expected) || stub.bounds.End.Sub(*stub.bounds.Start) != 24*time.Hour || !stub.bounds.Start.Equal(*summaryBounds.Start) || !stub.bounds.End.Equal(*summaryBounds.End) || stub.action != 6 || stub.page != 2 || page.Total != 11 || page.Items[0].Deck.Name != "deleted deck snapshot" {
			t.Fatalf("date=%s page=%+v bounds=%+v error=%v", date, page, stub.bounds, err)
		}
	}
	stub.items, stub.total = nil, 0
	page, err := s.List(context.Background(), model.OperationLogParams{Date: "2026-11-09", Action: "add_wishlist", Page: 1})
	if err != nil || page.Items == nil || len(page.Items) != 0 || stub.action != 3 {
		t.Fatalf("empty page=%+v error=%v", page, err)
	}
}

func TestLogDependencyErrorsRetainCause(t *testing.T) {
	cause := errors.New("database unavailable")
	s := &Service{store: &logStoreStub{err: cause}, location: time.UTC}
	_, err := s.Summary(context.Background(), model.OperationSummaryParams{Page: 1})
	var appErr *apperror.Error
	if !errors.As(err, &appErr) || appErr.Code != apperror.CodeDependencyUnavailable || !errors.Is(err, cause) {
		t.Fatalf("error=%v", err)
	}
}
