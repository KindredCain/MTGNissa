package httpapi

import "mtgnissa/internal/model"

type searchPageResponse struct {
	Input         string                `json:"input"`
	Mode          string                `json:"mode"`
	ExecutedQuery string                `json:"executed_query"`
	Items         []cardSummaryResponse `json:"items"`
	Total         int64                 `json:"total"`
	NextCursor    *string               `json:"next_cursor"`
}

func newSearchPage(page model.SearchPage) searchPageResponse {
	result := searchPageResponse{Input: page.Input, Mode: page.Mode, ExecutedQuery: page.ExecutedQuery, Items: []cardSummaryResponse{}, Total: page.Total, NextCursor: page.NextCursor}
	for _, card := range page.Items {
		result.Items = append(result.Items, newCardSummary(card))
	}
	return result
}
