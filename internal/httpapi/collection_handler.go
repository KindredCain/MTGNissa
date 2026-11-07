package httpapi

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"mtgnissa/internal/apperror"
	"mtgnissa/internal/collection"
	"mtgnissa/internal/httpresponse"
)

type collectionLister interface {
	Options(context.Context) (collection.Options, error)
	List(context.Context, collection.Params) (collection.Page, error)
}

func getCollectionOptions(service collectionLister) gin.HandlerFunc {
	return func(c *gin.Context) {
		options, err := service.Options(c.Request.Context())
		if err != nil {
			httpresponse.WriteAppError(c, err)
			return
		}
		httpresponse.WriteJSON(c, http.StatusOK, newCollectionOptions(options))
	}
}

func getCollection(service collectionLister) gin.HandlerFunc {
	return func(c *gin.Context) {
		params, err := parseCollectionParams(c.Request.URL.Query())
		if err != nil {
			httpresponse.WriteAppError(c, err)
			return
		}
		page, err := service.List(c.Request.Context(), params)
		if err != nil {
			httpresponse.WriteAppError(c, err)
			return
		}
		httpresponse.WriteJSON(c, http.StatusOK, newCollectionPage(page))
	}
}

func parseCollectionParams(query url.Values) (collection.Params, error) {
	params := collection.Params{Query: strings.TrimSpace(query.Get("q")), Sort: "set", MergeLanguages: true, Cursor: query.Get("cursor")}
	for _, field := range []string{"q", "sort", "merge_sets", "merge_languages", "cursor"} {
		if len(query[field]) > 1 {
			return collection.Params{}, apperror.New(apperror.CodeInvalidRequest, field+" must not be repeated")
		}
	}
	if containsNUL(params.Query) || utf8.RuneCountInString(params.Query) > 512 {
		return collection.Params{}, validationError("q", "must_not_contain_nul_and_must_be_at_most_512_characters")
	}
	if containsNUL(params.Cursor) {
		return collection.Params{}, validationError("cursor", "must_not_contain_nul")
	}
	if value := query.Get("sort"); value != "" {
		params.Sort = value
	}
	if !oneOf(params.Sort, "set", "color", "mana_value", "name") {
		return collection.Params{}, apperror.New(apperror.CodeInvalidRequest, "sort is invalid")
	}
	for field, target := range map[string]*bool{"merge_sets": &params.MergeSets, "merge_languages": &params.MergeLanguages} {
		if _, present := query[field]; present {
			value := query.Get(field)
			if value != "true" && value != "false" {
				return collection.Params{}, apperror.New(apperror.CodeInvalidRequest, field+" must be true or false")
			}
			*target = value == "true"
		}
	}
	if params.Sort == "set" && params.MergeSets {
		return collection.Params{}, apperror.New(apperror.CodeInvalidRequest, "merge_sets must be false when sort is set")
	}
	for _, filter := range []struct {
		field  string
		target *[]string
	}{
		{"set_code", &params.Sets}, {"language", &params.Languages}, {"color", &params.Colors},
		{"mana_value", &params.ManaValues}, {"rarity", &params.Rarities},
	} {
		values, err := normalizedMultiValue(query[filter.field], filter.field)
		if err != nil {
			return collection.Params{}, err
		}
		for _, value := range values {
			switch filter.field {
			case "set_code", "language":
				if strings.TrimSpace(value) == "" || strings.Contains(value, ",") || utf8.RuneCountInString(value) > 16 {
					return collection.Params{}, validationError(filter.field, "must_be_one_nonblank_code_at_most_16_characters")
				}
			case "color":
				if !oneOf(value, "W", "U", "B", "R", "G", "multicolor", "colorless") {
					return collection.Params{}, apperror.New(apperror.CodeInvalidRequest, "color is invalid")
				}
			case "mana_value":
				if !oneOf(value, "0", "1", "2", "3", "4", "5", "6", "7", "8_plus") {
					return collection.Params{}, apperror.New(apperror.CodeInvalidRequest, "mana_value is invalid")
				}
			case "rarity":
				if !oneOf(value, "common", "uncommon", "rare", "mythic", "special", "bonus") {
					return collection.Params{}, apperror.New(apperror.CodeInvalidRequest, "rarity is invalid")
				}
			}
		}
		*filter.target = values
	}
	return params, nil
}

func oneOf(value string, choices ...string) bool {
	for _, choice := range choices {
		if value == choice {
			return true
		}
	}
	return false
}
