package httpapi

import (
	"mtgnissa/internal/model"
)

type tokenListResponse struct {
	Set   tokenSetResponse `json:"set"`
	Items []tokenResponse  `json:"items"`
	Total int              `json:"total"`
}

type tokenSetResponse struct {
	Code        string  `json:"code"`
	Name        string  `json:"name"`
	DisplayName string  `json:"display_name"`
	ReleasedAt  *string `json:"released_at"`
}

type tokenResponse struct {
	UUID            string             `json:"token_uuid"`
	Name            string             `json:"name"`
	CollectorNumber string             `json:"collector_number"`
	Layout          string             `json:"layout"`
	Images          tokenImageResponse `json:"images"`
}

type tokenImageResponse struct {
	FrontURL        *string `json:"front_url"`
	BackURL         *string `json:"back_url"`
	HasDistinctBack bool    `json:"has_distinct_back"`
}

func newTokenListResponse(list model.TokenList) tokenListResponse {
	response := tokenListResponse{
		Set:   tokenSetResponse{Code: list.Set.Code, Name: list.Set.Name, DisplayName: list.Set.DisplayName},
		Items: make([]tokenResponse, 0, len(list.Items)), Total: len(list.Items),
	}
	if list.Set.ReleasedAt != nil {
		date := list.Set.ReleasedAt.Format("2006-01-02")
		response.Set.ReleasedAt = &date
	}
	for _, item := range list.Items {
		response.Items = append(response.Items, tokenResponse{
			UUID: item.UUID, Name: item.Name, CollectorNumber: item.CollectorNumber, Layout: item.Layout,
			Images: tokenImageResponse{FrontURL: item.FrontURL, BackURL: item.BackURL, HasDistinctBack: item.BackURL != nil},
		})
	}
	return response
}
