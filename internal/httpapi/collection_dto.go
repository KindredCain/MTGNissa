package httpapi

import (
	"mtgnissa/internal/model"
)

type collectionSetOptionResponse struct {
	Code        string `json:"code"`
	DisplayName string `json:"display_name"`
}

type collectionOptionsResponse struct {
	Sets       []collectionSetOptionResponse `json:"sets"`
	Languages  []cardLanguageResponse        `json:"languages"`
	Colors     []string                      `json:"colors"`
	ManaValues []string                      `json:"mana_values"`
	Rarities   []string                      `json:"rarities"`
}

type collectionItemResponse struct {
	Card            cardSummaryResponse      `json:"card"`
	GroupSet        *cardSetResponse         `json:"group_set"`
	GroupSummary    *ownershipTotalsResponse `json:"group_summary"`
	NormalQuantity  uint64                   `json:"normal_quantity"`
	FoilQuantity    uint64                   `json:"foil_quantity"`
	MergedSets      bool                     `json:"merged_sets"`
	MergedLanguages bool                     `json:"merged_languages"`
}

type collectionPageResponse struct {
	Items      []collectionItemResponse `json:"items"`
	Summary    ownershipTotalsResponse  `json:"summary"`
	Total      int64                    `json:"total"`
	NextCursor *string                  `json:"next_cursor"`
}

func newCollectionOptions(options model.CollectionOptions) collectionOptionsResponse {
	result := collectionOptionsResponse{Sets: []collectionSetOptionResponse{}, Languages: []cardLanguageResponse{},
		Colors: append([]string{}, options.Colors...), ManaValues: append([]string{}, options.ManaValues...), Rarities: append([]string{}, options.Rarities...)}
	for _, set := range options.Sets {
		result.Sets = append(result.Sets, collectionSetOptionResponse{Code: set.Code, DisplayName: set.DisplayName})
	}
	for _, language := range options.Languages {
		result.Languages = append(result.Languages, newCardLanguage(language))
	}
	return result
}

func newCollectionPage(page model.CollectionPage) collectionPageResponse {
	result := collectionPageResponse{Items: make([]collectionItemResponse, 0, len(page.Items)),
		Summary: ownershipTotalsResponse{Total: page.Summary.TotalQuantity, Distinct: page.Summary.DistinctCards}, Total: page.Total, NextCursor: page.NextCursor}
	for _, item := range page.Items {
		row := collectionItemResponse{Card: newCardSummary(item.Card), NormalQuantity: item.Quantity.Normal, FoilQuantity: item.Quantity.Foil,
			MergedSets: item.MergedSets, MergedLanguages: item.MergedLanguages}
		if item.GroupSet != nil {
			set := newCardSet(*item.GroupSet)
			row.GroupSet = &set
		}
		if item.GroupSummary != nil {
			row.GroupSummary = &ownershipTotalsResponse{Total: item.GroupSummary.TotalQuantity, Distinct: item.GroupSummary.DistinctCards}
		}
		result.Items = append(result.Items, row)
	}
	return result
}
