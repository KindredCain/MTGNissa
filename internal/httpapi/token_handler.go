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
	"mtgnissa/internal/tokens"
)

type tokenLister interface {
	List(context.Context, string) (tokens.List, error)
}

func getTokens(service tokenLister) gin.HandlerFunc {
	return func(c *gin.Context) {
		code, err := parseTokenSetCode(c.Request.URL.Query())
		if err != nil {
			httpresponse.WriteAppError(c, err)
			return
		}
		list, err := service.List(c.Request.Context(), code)
		if err != nil {
			httpresponse.WriteAppError(c, err)
			return
		}
		httpresponse.WriteJSON(c, http.StatusOK, newTokenListResponse(list))
	}
}

func parseTokenSetCode(query url.Values) (string, error) {
	if len(query["set_code"]) > 1 {
		return "", apperror.New(apperror.CodeInvalidRequest, "set_code must not be repeated")
	}
	code := strings.TrimSpace(query.Get("set_code"))
	if code == "" {
		return "", validationError("set_code", "required")
	}
	if containsNUL(code) || utf8.RuneCountInString(code) > 16 {
		return "", validationError("set_code", "must_not_contain_nul_and_must_be_at_most_16_characters")
	}
	return code, nil
}
