package httpapi

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"mtgnissa/internal/apperror"
	"mtgnissa/internal/httpresponse"
	"mtgnissa/internal/model"
)

type operationLogService interface {
	Summary(context.Context, model.OperationSummaryParams) (model.OperationSummaryPage, error)
	List(context.Context, model.OperationLogParams) (model.OperationLogPage, error)
}

func getOperationSummary(service operationLogService) gin.HandlerFunc {
	return func(c *gin.Context) {
		params, err := parseOperationSummaryParams(c.Request.URL.Query())
		if err != nil {
			httpresponse.WriteAppError(c, err)
			return
		}
		page, err := service.Summary(c.Request.Context(), params)
		if err != nil {
			httpresponse.WriteAppError(c, err)
			return
		}
		httpresponse.WriteJSON(c, http.StatusOK, newOperationSummary(page))
	}
}

func getOperationLogs(service operationLogService) gin.HandlerFunc {
	return func(c *gin.Context) {
		params, err := parseOperationLogParams(c.Request.URL.Query())
		if err != nil {
			httpresponse.WriteAppError(c, err)
			return
		}
		page, err := service.List(c.Request.Context(), params)
		if err != nil {
			httpresponse.WriteAppError(c, err)
			return
		}
		httpresponse.WriteJSON(c, http.StatusOK, newOperationLogPage(page))
	}
}

func parseOperationSummaryParams(query url.Values) (model.OperationSummaryParams, error) {
	page, err := parseOperationPage(query, "start_date", "end_date")
	if err != nil {
		return model.OperationSummaryParams{}, err
	}
	start, end := query.Get("start_date"), query.Get("end_date")
	for _, field := range []string{"start_date", "end_date"} {
		if value := query.Get(field); value != "" && !validOperationDate(value) {
			return model.OperationSummaryParams{}, validationError(field, "must_be_valid_date")
		}
	}
	if start != "" && end != "" && start > end {
		return model.OperationSummaryParams{}, apperror.New(apperror.CodeInvalidDateRange, "start_date must not be after end_date")
	}
	return model.OperationSummaryParams{StartDate: start, EndDate: end, Page: page}, nil
}

func parseOperationLogParams(query url.Values) (model.OperationLogParams, error) {
	page, err := parseOperationPage(query, "operation_date", "action_type")
	if err != nil {
		return model.OperationLogParams{}, err
	}
	date, action := query.Get("operation_date"), query.Get("action_type")
	if !validOperationDate(date) {
		return model.OperationLogParams{}, validationError("operation_date", "required_valid_date")
	}
	if model.OperationActionCode(action) == 0 {
		return model.OperationLogParams{}, validationError("action_type", "required_valid_action_type")
	}
	return model.OperationLogParams{Date: date, Action: action, Page: page}, nil
}

func parseOperationPage(query url.Values, fields ...string) (int, error) {
	for _, field := range append(fields, "page") {
		if len(query[field]) > 1 {
			return 0, apperror.New(apperror.CodeInvalidRequest, field+" must not be repeated")
		}
	}
	value, present := query["page"]
	if !present {
		return 1, nil
	}
	page, err := strconv.Atoi(value[0])
	if err != nil {
		return 0, apperror.New(apperror.CodeInvalidRequest, "page must be an integer")
	}
	if page < 1 || page > (int(^uint(0)>>1)-10)/10 {
		return 0, validationError("page", "out_of_range")
	}
	return page, nil
}

func validOperationDate(value string) bool {
	parsed, err := time.Parse("2006-01-02", value)
	return err == nil && parsed.Year() >= 1 && parsed.Format("2006-01-02") == value
}
