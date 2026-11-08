package httpapi

import (
	"encoding/json"
	"strconv"
	"time"

	"mtgnissa/internal/model"
)

type deckNameRequest struct {
	Name *string `json:"name"`
}

type deckSaveRequest struct {
	Name              *string               `json:"name"`
	CoverPrintingUUID json.RawMessage       `json:"cover_printing_uuid"`
	Sections          *[]deckSectionRequest `json:"sections"`
}

type deckSectionRequest struct {
	Type  *string                `json:"type"`
	Cards *[]deckSaveCardRequest `json:"cards"`
}

type deckSaveCardRequest struct {
	PrintingUUID *string `json:"printing_uuid"`
	Quantity     *int64  `json:"quantity"`
}

type deckCodeRequest struct {
	SourceText *string `json:"source_text"`
}

type deckContextsRequest struct {
	PrintingUUIDs *[]string `json:"printing_uuids"`
}

type deckIdentityResponse struct {
	DeckID string `json:"deck_id"`
	Name   string `json:"name"`
}

type deckDeleteResponse struct {
	DeckID  string `json:"deck_id"`
	Deleted bool   `json:"deleted"`
}

type deckStatisticsResponse struct {
	CMC            float64  `json:"cmc"`
	ManaCost       *string  `json:"mana_cost"`
	Colors         []string `json:"colors"`
	ProducedMana   []string `json:"produced_mana"`
	TypeGroups     []string `json:"type_groups"`
	OracleTags     []string `json:"oracle_tags"`
	IsLand         bool     `json:"is_land"`
	CanBeCommander bool     `json:"can_be_commander"`
}

type deckContextResponse struct {
	PrintingUUID         string                 `json:"printing_uuid"`
	OracleID             string                 `json:"oracle_id"`
	Card                 cardSummaryResponse    `json:"card"`
	TypeGroup            string                 `json:"type_group"`
	TranslatedTypeLine   string                 `json:"translated_type_line"`
	TranslatedOracleText *string                `json:"translated_oracle_text"`
	OwnedQuantity        uint64                 `json:"owned_quantity"`
	OtherDecksQuantity   uint64                 `json:"other_decks_quantity"`
	OtherDecksCount      uint64                 `json:"other_decks_count"`
	Statistics           deckStatisticsResponse `json:"statistics"`
}

type deckEditContextResponse struct {
	RequestedPrintingUUID string `json:"requested_printing_uuid"`
	deckContextResponse
}

type deckContextsResponse struct {
	Items []deckEditContextResponse `json:"items"`
}

type deckExceptionResponse struct {
	Type            string `json:"type"`
	MissingQuantity uint64 `json:"missing_quantity"`
}

type deckExceptionTotalsResponse struct {
	DistinctCards   uint64 `json:"distinct_cards"`
	MissingQuantity uint64 `json:"missing_quantity"`
}

type deckCardResponse struct {
	deckContextResponse
	Quantity         uint64                 `json:"quantity"`
	AllDecksQuantity uint64                 `json:"all_decks_quantity"`
	UsingDecks       uint64                 `json:"using_decks"`
	Exception        *deckExceptionResponse `json:"exception"`
}

type deckSectionResponse struct {
	Type     string             `json:"type"`
	Quantity uint64             `json:"quantity"`
	Cards    []deckCardResponse `json:"cards"`
}

type deckSummaryResponse struct {
	CountedTotal   uint64                      `json:"counted_total"`
	CollectionRate float64                     `json:"collection_rate"`
	Shortage       deckExceptionTotalsResponse `json:"shortage"`
	DeckUsage      deckExceptionTotalsResponse `json:"deck_usage"`
}

type deckDetailResponse struct {
	DeckID            string                `json:"deck_id"`
	Name              string                `json:"name"`
	SourceText        string                `json:"source_text"`
	CoverPrintingUUID *string               `json:"cover_printing_uuid"`
	CreatedAt         time.Time             `json:"created_at"`
	UpdatedAt         time.Time             `json:"updated_at"`
	Sections          []deckSectionResponse `json:"sections"`
	Summary           deckSummaryResponse   `json:"summary"`
}

type deckSectionCountsResponse struct {
	Commander  uint64 `json:"commander"`
	Mainboard  uint64 `json:"mainboard"`
	Sideboard  uint64 `json:"sideboard"`
	Maybeboard uint64 `json:"maybeboard"`
}

type deckColorsResponse struct {
	W uint64 `json:"W"`
	U uint64 `json:"U"`
	B uint64 `json:"B"`
	R uint64 `json:"R"`
	G uint64 `json:"G"`
	C uint64 `json:"C"`
}

type deckListExceptionResponse struct {
	Type string `json:"type"`
	deckExceptionTotalsResponse
}

type deckListItemResponse struct {
	DeckID            string                     `json:"deck_id"`
	Name              string                     `json:"name"`
	CreatedAt         time.Time                  `json:"created_at"`
	UpdatedAt         time.Time                  `json:"updated_at"`
	CoverCard         *cardSummaryResponse       `json:"cover_card"`
	SectionCounts     deckSectionCountsResponse  `json:"section_counts"`
	CountedTotal      uint64                     `json:"counted_total"`
	ColorDistribution deckColorsResponse         `json:"color_distribution"`
	Exception         *deckListExceptionResponse `json:"exception"`
}

type deckPageResponse struct {
	Items      []deckListItemResponse `json:"items"`
	Total      int64                  `json:"total"`
	NextCursor *string                `json:"next_cursor"`
}

type deckCandidateResponse struct {
	Card              cardSummaryResponse `json:"card"`
	DisplayTypeLine   string              `json:"display_type_line"`
	DisplayOracleText *string             `json:"display_oracle_text"`
}

type deckCandidatePageResponse struct {
	Items      []deckCandidateResponse `json:"items"`
	Total      int64                   `json:"total"`
	NextCursor *string                 `json:"next_cursor"`
}

type deckNameSuggestionResponse struct {
	PrintedName string `json:"printed_name"`
}

type deckNameSuggestionsResponse struct {
	Items []deckNameSuggestionResponse `json:"items"`
}

type parsedDeckCardResponse struct {
	PrintingUUID string              `json:"printing_uuid"`
	OracleID     string              `json:"oracle_id"`
	Quantity     uint64              `json:"quantity"`
	Card         cardSummaryResponse `json:"card"`
}

type parsedDeckSectionResponse struct {
	Type  string                   `json:"type"`
	Cards []parsedDeckCardResponse `json:"cards"`
}

type parsedDeckResponse struct {
	Name     *string                     `json:"name"`
	Sections []parsedDeckSectionResponse `json:"sections"`
}

func newDeckContext(item model.DeckEditCardContext) deckContextResponse {
	stats := item.Statistics
	oracle := ""
	if item.Card.OracleID != nil {
		oracle = *item.Card.OracleID
	}
	return deckContextResponse{PrintingUUID: item.Card.PrintingUUID, OracleID: oracle, Card: newCardSummary(item.Card), TypeGroup: item.TypeGroup,
		TranslatedTypeLine: item.TranslatedTypeLine, TranslatedOracleText: item.TranslatedOracleText, OwnedQuantity: item.OwnedQuantity,
		OtherDecksQuantity: item.OtherDecksQuantity, OtherDecksCount: item.OtherDecksCount,
		Statistics: deckStatisticsResponse{CMC: stats.CMC, ManaCost: stats.ManaCost, Colors: append([]string{}, stats.Colors...), ProducedMana: append([]string{}, stats.ProducedMana...), TypeGroups: append([]string{}, stats.TypeGroups...), OracleTags: append([]string{}, stats.OracleTags...), IsLand: stats.IsLand, CanBeCommander: stats.CanBeCommander}}
}

func newDeckTotals(totals model.DeckExceptionTotals) deckExceptionTotalsResponse {
	return deckExceptionTotalsResponse{DistinctCards: totals.DistinctCards, MissingQuantity: totals.MissingQuantity}
}

func newDeckDetail(detail model.DeckDetail) deckDetailResponse {
	result := deckDetailResponse{DeckID: strconv.FormatUint(detail.ID, 10), Name: detail.Name, SourceText: detail.SourceText, CoverPrintingUUID: detail.CoverPrintingUUID,
		CreatedAt: detail.CreatedAt.UTC(), UpdatedAt: detail.UpdatedAt.UTC(), Sections: []deckSectionResponse{},
		Summary: deckSummaryResponse{CountedTotal: detail.Summary.CountedTotal, CollectionRate: detail.Summary.CollectionRate, Shortage: newDeckTotals(detail.Summary.Shortage), DeckUsage: newDeckTotals(detail.Summary.DeckUsage)}}
	for _, section := range detail.Sections {
		row := deckSectionResponse{Type: section.Type, Quantity: section.Quantity, Cards: []deckCardResponse{}}
		for _, card := range section.Cards {
			item := deckCardResponse{deckContextResponse: newDeckContext(card.Context), Quantity: card.Quantity, AllDecksQuantity: card.AllDecksQuantity, UsingDecks: card.UsingDecks}
			if card.Exception != nil {
				item.Exception = &deckExceptionResponse{Type: card.Exception.Type, MissingQuantity: card.Exception.MissingQuantity}
			}
			row.Cards = append(row.Cards, item)
		}
		result.Sections = append(result.Sections, row)
	}
	return result
}

func newDeckPage(page model.DeckPage) deckPageResponse {
	result := deckPageResponse{Items: []deckListItemResponse{}, Total: page.Total, NextCursor: page.NextCursor}
	for _, deck := range page.Items {
		counts, colors := deck.SectionCounts, deck.ColorDistribution
		item := deckListItemResponse{DeckID: strconv.FormatUint(deck.ID, 10), Name: deck.Name, CreatedAt: deck.CreatedAt.UTC(), UpdatedAt: deck.UpdatedAt.UTC(), CountedTotal: deck.CountedTotal,
			SectionCounts:     deckSectionCountsResponse{Commander: counts["commander"], Mainboard: counts["mainboard"], Sideboard: counts["sideboard"], Maybeboard: counts["maybeboard"]},
			ColorDistribution: deckColorsResponse{W: colors["W"], U: colors["U"], B: colors["B"], R: colors["R"], G: colors["G"], C: colors["C"]}}
		if deck.CoverCard != nil {
			cover := newCardSummary(*deck.CoverCard)
			item.CoverCard = &cover
		}
		if deck.Exception != nil {
			item.Exception = &deckListExceptionResponse{Type: deck.Exception.Type, deckExceptionTotalsResponse: newDeckTotals(deck.Exception.DeckExceptionTotals)}
		}
		result.Items = append(result.Items, item)
	}
	return result
}

func newDeckCandidates(page model.DeckCandidatePage) deckCandidatePageResponse {
	result := deckCandidatePageResponse{Items: []deckCandidateResponse{}, Total: page.Total, NextCursor: page.NextCursor}
	for _, item := range page.Items {
		result.Items = append(result.Items, deckCandidateResponse{Card: newCardSummary(item.Card), DisplayTypeLine: item.DisplayTypeLine, DisplayOracleText: item.DisplayOracleText})
	}
	return result
}

func newParsedDeck(parsed model.ParsedDeck) parsedDeckResponse {
	result := parsedDeckResponse{Name: parsed.Name, Sections: []parsedDeckSectionResponse{}}
	for _, section := range parsed.Sections {
		row := parsedDeckSectionResponse{Type: section.Type, Cards: []parsedDeckCardResponse{}}
		for _, item := range section.Cards {
			oracle := ""
			if item.Card.OracleID != nil {
				oracle = *item.Card.OracleID
			}
			row.Cards = append(row.Cards, parsedDeckCardResponse{PrintingUUID: item.Card.PrintingUUID, OracleID: oracle, Quantity: item.Quantity, Card: newCardSummary(item.Card)})
		}
		result.Sections = append(result.Sections, row)
	}
	return result
}
