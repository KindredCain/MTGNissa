package operationlogs

import (
	"context"
	"time"

	"mtgnissa/internal/apperror"
	"mtgnissa/internal/model"
	"mtgnissa/internal/store"
)

type logStore interface {
	Summary(context.Context, model.OperationRange, int, int) ([]model.OperationSummary, int64, error)
	List(context.Context, model.OperationRange, uint8, int) ([]model.OperationItem, int64, error)
}

// Service queries historical operations using the application timezone's current UTC offset.
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
	_, offset := time.Now().In(s.location).Zone()
	location := time.FixedZone("", offset)
	bounds := model.OperationRange{}
	if params.StartDate != "" {
		start, err := time.ParseInLocation("2006-01-02", params.StartDate, location)
		if err != nil {
			return model.OperationSummaryPage{}, err
		}
		utc := start.UTC()
		bounds.Start = &utc
	}
	if params.EndDate != "" {
		end, err := time.ParseInLocation("2006-01-02", params.EndDate, location)
		if err != nil {
			return model.OperationSummaryPage{}, err
		}
		utc := end.AddDate(0, 0, 1).UTC()
		bounds.End = &utc
	}
	items, total, err := s.store.Summary(ctx, bounds, offset, params.Page)
	if err != nil {
		return model.OperationSummaryPage{}, dependencyError(err)
	}
	return model.OperationSummaryPage{Items: append([]model.OperationSummary{}, items...), Page: params.Page, Total: total}, nil
}

// List reads one action's snapshots for a day using the current fixed UTC offset.
func (s *Service) List(ctx context.Context, params model.OperationLogParams) (model.OperationLogPage, error) {
	_, offset := time.Now().In(s.location).Zone()
	date, err := time.ParseInLocation("2006-01-02", params.Date, time.FixedZone("", offset))
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
