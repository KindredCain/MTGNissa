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
	events []model.OperationEvent
	items  []model.OperationItem
	total  int64
	bounds model.OperationRange
	action uint8
	page   int
	err    error
}

func (s *logStoreStub) Events(_ context.Context, bounds model.OperationRange) ([]model.OperationEvent, error) {
	s.bounds = bounds
	return s.events, s.err
}

func (s *logStoreStub) List(_ context.Context, bounds model.OperationRange, action uint8, page int) ([]model.OperationItem, int64, error) {
	s.bounds, s.action, s.page = bounds, action, page
	return s.items, s.total, s.err
}

func TestSummaryUsesLocalCalendarActionOrderAndGroupPagination(t *testing.T) {
	location := time.FixedZone("UTC+8", 8*60*60)
	beforeMidnight := time.Date(2026, 11, 8, 15, 59, 59, 0, time.UTC)
	afterMidnight := beforeMidnight.Add(time.Second)
	stub := &logStoreStub{}
	for action := uint8(8); action > 0; action-- {
		stub.events = append(stub.events, model.OperationEvent{Action: action, OccurredAt: beforeMidnight}, model.OperationEvent{Action: action, OccurredAt: afterMidnight})
	}
	stub.events = append(stub.events, model.OperationEvent{Action: 1, OccurredAt: afterMidnight})
	s := &Service{store: stub, location: location}
	page, err := s.Summary(context.Background(), model.OperationSummaryParams{StartDate: "2026-11-08", EndDate: "2026-11-09", Page: 1})
	if err != nil || page.Total != 16 || len(page.Items) != 10 || page.Items[0].Date != "2026-11-09" || page.Items[0].Action != "add_card" || page.Items[0].Count != 2 || page.Items[7].Action != "modify_deck" || page.Items[8].Date != "2026-11-08" {
		t.Fatalf("page=%+v error=%v", page, err)
	}
	if stub.bounds.Start.Format(time.RFC3339) != "2026-11-07T16:00:00Z" || stub.bounds.End.Format(time.RFC3339) != "2026-11-09T16:00:00Z" {
		t.Fatalf("bounds=%+v", stub.bounds)
	}
	page, err = s.Summary(context.Background(), model.OperationSummaryParams{Page: 2})
	if err != nil || len(page.Items) != 6 || stub.bounds.Start != nil || stub.bounds.End != nil {
		t.Fatalf("page=%+v bounds=%+v error=%v", page, stub.bounds, err)
	}
	page, err = s.Summary(context.Background(), model.OperationSummaryParams{Page: 3})
	if err != nil || len(page.Items) != 0 || page.Items == nil || page.Total != 16 {
		t.Fatalf("out-of-range page=%+v error=%v", page, err)
	}
}

func TestDetailUsesDSTDayBoundariesAndStoredSnapshots(t *testing.T) {
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	stub := &logStoreStub{items: []model.OperationItem{{ID: 42, Action: "delete_deck", Deck: &model.OperationDeckSnapshot{ID: 17, Name: "deleted deck snapshot"}}}, total: 11}
	s := &Service{store: stub, location: location}
	for _, test := range []struct {
		date  string
		hours time.Duration
	}{{"2026-03-08", 23 * time.Hour}, {"2026-11-01", 25 * time.Hour}} {
		page, err := s.List(context.Background(), model.OperationLogParams{Date: test.date, Action: "delete_deck", Page: 2})
		if err != nil || stub.bounds.End.Sub(*stub.bounds.Start) != test.hours || stub.action != 6 || stub.page != 2 || page.Total != 11 || page.Items[0].Deck.Name != "deleted deck snapshot" {
			t.Fatalf("date=%s page=%+v bounds=%+v error=%v", test.date, page, stub.bounds, err)
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
