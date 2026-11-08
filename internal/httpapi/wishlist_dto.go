package httpapi

import "mtgnissa/internal/model"

type wishlistItemResponse struct {
	Card     cardSummaryResponse `json:"card"`
	GroupSet *cardSetResponse    `json:"group_set"`
	Included bool                `json:"included"`
}

type wishlistPageResponse struct {
	Items      []wishlistItemResponse `json:"items"`
	Total      int64                  `json:"total"`
	NextCursor *string                `json:"next_cursor"`
}

func newWishlistOptions(options model.WishlistOptions) collectionOptionsResponse {
	return newCollectionOptions(model.CollectionOptions{Sets: options.Sets, Languages: options.Languages,
		Colors: options.Colors, ManaValues: options.ManaValues, Rarities: options.Rarities})
}

func newWishlistPage(page model.WishlistPage) wishlistPageResponse {
	result := wishlistPageResponse{Items: make([]wishlistItemResponse, 0, len(page.Items)), Total: page.Total, NextCursor: page.NextCursor}
	for _, item := range page.Items {
		row := wishlistItemResponse{Card: newCardSummary(item.Card), Included: item.Included}
		if item.GroupSet != nil {
			set := newCardSet(*item.GroupSet)
			row.GroupSet = &set
		}
		result.Items = append(result.Items, row)
	}
	return result
}
