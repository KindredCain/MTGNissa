package operationlogs

import (
	"context"
	"sort"
	"time"

	"mtgnissa/internal/apperror"
	"mtgnissa/internal/model"
	"mtgnissa/internal/store"
)

type logStore interface {
	Events(context.Context, model.OperationRange) ([]model.OperationEvent, error)
	List(context.Context, model.OperationRange, uint8, int) ([]model.OperationItem, int64, error)
}

// Service groups and pages historical operations using the configured application timezone.
type Service struct {
	store    logStore
	location *time.Location
}

// NewService creates a snapshot-only log reader with application calendar boundaries.
func NewService(logs *store.OperationLogStore, location *time.Location) *Service {
	return &Service{store: logs, location: location}
}

// Summary counts events by local date and action without loading historical snapshots.
func (s *Service) Summary(ctx context.Context, params model.OperationSummaryParams) (model.OperationSummaryPage, error) {
	bounds := model.OperationRange{}
	if params.StartDate != "" {
		start, err := time.ParseInLocation("2006-01-02", params.StartDate, s.location)
		if err != nil {
			return model.OperationSummaryPage{}, err
		}
		utc := start.UTC()
		bounds.Start = &utc
	}
	if params.EndDate != "" {
		end, err := time.ParseInLocation("2006-01-02", params.EndDate, s.location)
		if err != nil {
			return model.OperationSummaryPage{}, err
		}
		utc := end.AddDate(0, 0, 1).UTC()
		bounds.End = &utc
	}
	events, err := s.store.Events(ctx, bounds)
	if err != nil {
		return model.OperationSummaryPage{}, dependencyError(err)
	}
	groups := make(map[string]map[uint8]int64)
	for _, event := range events {
		date := event.OccurredAt.In(s.location).Format("2006-01-02")
		if groups[date] == nil {
			groups[date] = make(map[uint8]int64)
		}
		groups[date][event.Action]++
	}
	items := make([]model.OperationSummary, 0)
	for date, actions := range groups {
		for action, count := range actions {
			items = append(items, model.OperationSummary{Date: date, Action: model.OperationActionName(action), Count: count})
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Date != items[j].Date {
			return items[i].Date > items[j].Date
		}
		return model.OperationActionCode(items[i].Action) < model.OperationActionCode(items[j].Action)
	})
	start := min((params.Page-1)*10, len(items))
	end := min(start+10, len(items))
	return model.OperationSummaryPage{Items: items[start:end], Page: params.Page, Total: int64(len(items))}, nil
}

// List reads one action's snapshots for a local day, including days affected by DST.
func (s *Service) List(ctx context.Context, params model.OperationLogParams) (model.OperationLogPage, error) {
	date, err := time.ParseInLocation("2006-01-02", params.Date, s.location)
	if err != nil {
		return model.OperationLogPage{}, err
	}
	start, end := date.UTC(), date.AddDate(0, 0, 1).UTC()
	items, total, err := s.store.List(ctx, model.OperationRange{Start: &start, End: &end}, model.OperationActionCode(params.Action), params.Page)
	if err != nil {
		return model.OperationLogPage{}, dependencyError(err)
	}
	return model.OperationLogPage{Items: append([]model.OperationItem{}, items...), Page: params.Page, Total: total}, nil
}

func dependencyError(err error) error {
	return apperror.Wrap(apperror.CodeDependencyUnavailable, "operation logs are temporarily unavailable", err)
}
