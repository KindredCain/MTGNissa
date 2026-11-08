package httpapi

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"mtgnissa/internal/apperror"
	"mtgnissa/internal/httpresponse"
	"mtgnissa/internal/model"
)

type searchLister interface {
	List(context.Context, model.SearchParams) (model.SearchPage, error)
}

func getSearch(service searchLister) gin.HandlerFunc {
	return func(c *gin.Context) {
		params, err := parseSearchParams(c.Request.URL.Query())
		if err != nil {
			httpresponse.WriteAppError(c, err)
			return
		}
		page, err := service.List(c.Request.Context(), params)
		if err != nil {
			httpresponse.WriteAppError(c, err)
			return
		}
		httpresponse.WriteJSON(c, http.StatusOK, newSearchPage(page))
	}
}

func parseSearchParams(query url.Values) (model.SearchParams, error) {
	for _, field := range []string{"q", "cursor"} {
		if len(query[field]) > 1 {
			return model.SearchParams{}, apperror.New(apperror.CodeInvalidRequest, field+" must not be repeated")
		}
	}
	q := query.Get("q")
	trimmed := strings.TrimSpace(q)
	if trimmed == "" {
		return model.SearchParams{}, validationError("q", "required")
	}
	if !utf8.ValidString(q) || containsNUL(q) || utf8.RuneCountInString(trimmed) > 512 {
		return model.SearchParams{}, validationError("q", "must_be_utf8_without_nul_and_at_most_512_characters")
	}
	cursor := query.Get("cursor")
	if containsNUL(cursor) {
		return model.SearchParams{}, validationError("cursor", "must_not_contain_nul")
	}
	return model.SearchParams{Query: q, Cursor: cursor}, nil
}
