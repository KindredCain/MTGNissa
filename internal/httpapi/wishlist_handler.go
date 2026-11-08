package httpapi

import (
	"context"
	"net/http"
	"net/url"

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
	// Both lists use the same filter domains and scalar validation rules.
	params, err := parseCollectionParams(query)
	if err != nil {
		return model.WishlistParams{}, err
	}
	return model.WishlistParams{Query: params.Query, Sets: params.Sets, Languages: params.Languages,
		Colors: params.Colors, ManaValues: params.ManaValues, Rarities: params.Rarities, Sort: params.Sort, Cursor: params.Cursor}, nil
}
