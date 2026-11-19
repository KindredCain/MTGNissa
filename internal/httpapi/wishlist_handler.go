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

type wishlistLister interface {
	Options(context.Context) (model.WishlistOptions, error)
	List(context.Context, model.WishlistParams) (model.WishlistPage, error)
}

func getWishlistOptions(service wishlistLister) gin.HandlerFunc {
	return func(c *gin.Context) {
		options, err := service.Options(c.Request.Context())
		if err != nil {
			httpresponse.WriteAppError(c, err)
			return
		}
		httpresponse.WriteJSON(c, http.StatusOK, newWishlistOptions(options))
	}
}

func getWishlist(service wishlistLister) gin.HandlerFunc {
	return func(c *gin.Context) {
		params, err := parseWishlistParams(c.Request.URL.Query())
		if err != nil {
			httpresponse.WriteAppError(c, err)
			return
		}
		page, err := service.List(c.Request.Context(), params)
		if err != nil {
			httpresponse.WriteAppError(c, err)
			return
		}
		httpresponse.WriteJSON(c, http.StatusOK, newWishlistPage(page))
	}
}

func parseWishlistParams(query url.Values) (model.WishlistParams, error) {
	for _, field := range []string{"merge_sets", "merge_languages"} {
		if _, present := query[field]; present {
			return model.WishlistParams{}, apperror.New(apperror.CodeInvalidRequest, "wishlist does not accept "+field)
		}
	}
	params := model.WishlistParams{Query: strings.TrimSpace(query.Get("q")), Sort: "set_desc", Cursor: query.Get("cursor")}
	for _, field := range []string{"q", "sort", "cursor"} {
		if len(query[field]) > 1 {
			return model.WishlistParams{}, apperror.New(apperror.CodeInvalidRequest, field+" must not be repeated")
		}
	}
	if containsNUL(params.Query) || utf8.RuneCountInString(params.Query) > 512 {
		return model.WishlistParams{}, validationError("q", "must_not_contain_nul_and_must_be_at_most_512_characters")
	}
	if containsNUL(params.Cursor) {
		return model.WishlistParams{}, validationError("cursor", "must_not_contain_nul")
	}
	if value := query.Get("sort"); value != "" {
		params.Sort = value
	}
	if !oneOf(params.Sort, "set_desc", "color", "mana_value_asc", "name_asc") {
		return model.WishlistParams{}, validationError("sort", "must_be_a_valid_wishlist_sort")
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
			return model.WishlistParams{}, err
		}
		for _, value := range values {
			switch filter.field {
			case "set_code", "language":
				if strings.TrimSpace(value) == "" || strings.Contains(value, ",") || utf8.RuneCountInString(value) > 16 {
					return model.WishlistParams{}, validationError(filter.field, "must_be_one_nonblank_code_at_most_16_characters")
				}
			case "color":
				if !oneOf(value, "W", "U", "B", "R", "G", "multicolor", "colorless") {
					return model.WishlistParams{}, validationError("color", "must_be_a_valid_color_group")
				}
			case "mana_value":
				if !oneOf(value, "0", "1", "2", "3", "4", "5", "6", "7", "8_plus") {
					return model.WishlistParams{}, validationError("mana_value", "must_be_a_valid_mana_value_group")
				}
			case "rarity":
				if !oneOf(value, "common", "uncommon", "rare", "mythic", "special", "bonus") {
					return model.WishlistParams{}, validationError("rarity", "must_be_a_valid_rarity")
				}
			}
		}
		*filter.target = values
	}
	return params, nil
}
