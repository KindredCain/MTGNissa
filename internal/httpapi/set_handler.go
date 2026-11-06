package httpapi

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"mtgnissa/internal/apperror"
	"mtgnissa/internal/httpresponse"
	"mtgnissa/internal/sets"
)

type setLister interface {
	List(context.Context, sets.Params) (sets.Page, error)
	Options(context.Context) (sets.Options, error)
}

func getSetOptions(service setLister) gin.HandlerFunc {
	return func(c *gin.Context) {
		options, err := service.Options(c.Request.Context())
		if err != nil {
			httpresponse.WriteAppError(c, err)
			return
		}
		httpresponse.WriteJSON(c, http.StatusOK, newSetOptionsResponse(options))
	}
}

func getSets(service setLister) gin.HandlerFunc {
	return func(c *gin.Context) {
		params, err := parseSetListParams(c)
		if err != nil {
			httpresponse.WriteAppError(c, err)
			return
		}
		page, err := service.List(c.Request.Context(), params)
		if err != nil {
			httpresponse.WriteAppError(c, err)
			return
		}
		httpresponse.WriteJSON(c, http.StatusOK, newSetListResponse(page))
	}
}

func parseSetListParams(c *gin.Context) (sets.Params, error) {
	query := c.Request.URL.Query()
	for _, field := range []string{"q", "sort", "cursor"} {
		if len(query[field]) > 1 {
			return sets.Params{}, apperror.New(apperror.CodeInvalidRequest, field+" must not be repeated")
		}
	}
	params := sets.Params{
		Query:  strings.TrimSpace(query.Get("q")),
		Sort:   sets.SortReleasedDesc,
		Cursor: query.Get("cursor"),
	}
	if containsNUL(params.Query) || utf8.RuneCountInString(params.Query) > 512 {
		return sets.Params{}, validationError("q", "must_not_contain_nul_and_must_be_at_most_512_characters")
	}
	if value := query.Get("sort"); value != "" {
		params.Sort = sets.Sort(value)
	}
	if !validSetSort(params.Sort) {
		return sets.Params{}, apperror.New(apperror.CodeInvalidRequest, "sort is invalid")
	}
	if containsNUL(params.Cursor) {
		return sets.Params{}, validationError("cursor", "must_not_contain_nul")
	}
	var err error
	params.Languages, err = normalizedMultiValue(query["language"], "language")
	if err != nil {
		return sets.Params{}, err
	}
	params.SetTypes, err = normalizedMultiValue(query["set_type"], "set_type")
	if err != nil {
		return sets.Params{}, err
	}
	return params, nil
}

func normalizedMultiValue(values []string, field string) ([]string, error) {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if containsNUL(value) {
			return nil, validationError(field, "must_not_contain_nul")
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	if len(result) > 128 {
		return nil, validationError(field, "must_contain_at_most_128_unique_values")
	}
	sort.Strings(result)
	return result, nil
}

func validSetSort(value sets.Sort) bool {
	switch value {
	case sets.SortReleasedDesc,
		sets.SortReleasedAsc,
		sets.SortNameAsc,
		sets.SortNameDesc,
		sets.SortCodeAsc,
		sets.SortCodeDesc,
		sets.SortCardCountDesc,
		sets.SortCardCountAsc:
		return true
	default:
		return false
	}
}

func containsNUL(value string) bool {
	return strings.ContainsRune(value, '\x00')
}

func validationError(field, reason string) error {
	return apperror.WithDetails(apperror.CodeValidationFailed, "request validation failed", []apperror.Detail{{Field: field, Reason: reason}})
}
