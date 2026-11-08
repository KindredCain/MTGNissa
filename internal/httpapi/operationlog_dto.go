package httpapi

import (
	"strconv"
	"time"

	"mtgnissa/internal/model"
)

type operationSummaryResponse struct {
	Date   string `json:"operation_date"`
	Action string `json:"action_type"`
	Count  int64  `json:"count"`
}

type operationSummaryPageResponse struct {
	Items      []operationSummaryResponse `json:"items"`
	Page       int                        `json:"page"`
	PageSize   int                        `json:"page_size"`
	TotalItems int64                      `json:"total_items"`
	TotalPages int64                      `json:"total_pages"`
}

type operationCardResponse struct {
	PrintingUUID        string             `json:"printing_uuid"`
	Name                string             `json:"name"`
	SetCode             string             `json:"set_code"`
	CollectorNumber     string             `json:"collector_number"`
	Language            string             `json:"language"`
	LanguageDisplayName string             `json:"language_display_name"`
	Finish              *string            `json:"finish"`
	QuantityDelta       *int64             `json:"quantity_delta"`
	Images              cardImagesResponse `json:"images"`
}

type operationDeckResponse struct {
	ID   string `json:"deck_id"`
	Name string `json:"name"`
}

type operationItemResponse struct {
	ID         string                 `json:"operation_id"`
	Kind       string                 `json:"kind"`
	Action     string                 `json:"action_type"`
	OccurredAt string                 `json:"occurred_at"`
	Card       *operationCardResponse `json:"card,omitempty"`
	Deck       *operationDeckResponse `json:"deck,omitempty"`
}

type operationLogPageResponse struct {
	Items      []operationItemResponse `json:"items"`
	Page       int                     `json:"page"`
	PageSize   int                     `json:"page_size"`
	TotalItems int64                   `json:"total_items"`
	TotalPages int64                   `json:"total_pages"`
}

func operationTotalPages(total int64) int64 {
	pages := total / 10
	if total%10 != 0 {
		pages++
	}
	return pages
}

func newOperationSummary(page model.OperationSummaryPage) operationSummaryPageResponse {
	result := operationSummaryPageResponse{Items: []operationSummaryResponse{}, Page: page.Page, PageSize: 10, TotalItems: page.Total, TotalPages: operationTotalPages(page.Total)}
	for _, item := range page.Items {
		result.Items = append(result.Items, operationSummaryResponse{Date: item.Date, Action: item.Action, Count: item.Count})
	}
	return result
}

func newOperationLogPage(page model.OperationLogPage) operationLogPageResponse {
	result := operationLogPageResponse{Items: []operationItemResponse{}, Page: page.Page, PageSize: 10, TotalItems: page.Total, TotalPages: operationTotalPages(page.Total)}
	for _, item := range page.Items {
		row := operationItemResponse{ID: strconv.FormatUint(item.ID, 10), Action: item.Action, OccurredAt: item.OccurredAt.UTC().Format(time.RFC3339Nano)}
		if item.Card != nil {
			card := item.Card
			row.Kind = "card"
			row.Card = &operationCardResponse{PrintingUUID: card.PrintingUUID, Name: card.Name, SetCode: card.SetCode, CollectorNumber: card.CollectorNumber,
				Language: card.Language, LanguageDisplayName: card.LanguageDisplayName, Finish: card.Finish, QuantityDelta: card.QuantityDelta,
				Images: cardImagesResponse{FrontURL: card.Images.FrontURL, BackURL: card.Images.BackURL, HasDistinctBack: card.HasDistinctBack}}
		} else if item.Deck != nil {
			row.Kind = "deck"
			row.Deck = &operationDeckResponse{ID: strconv.FormatUint(item.Deck.ID, 10), Name: item.Deck.Name}
		}
		result.Items = append(result.Items, row)
	}
	return result
}
