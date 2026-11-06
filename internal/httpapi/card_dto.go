package httpapi

import (
	"strconv"
	"time"

	"mtgnissa/internal/cards"
)

type cardSetResponse struct {
	Code        string  `json:"code"`
	Name        string  `json:"name"`
	DisplayName string  `json:"display_name"`
	ReleasedAt  *string `json:"released_at"`
}

type cardLanguageResponse struct {
	Code        string `json:"code"`
	DisplayName string `json:"display_name"`
	SortOrder   int    `json:"sort_order"`
}

type cardImagesResponse struct {
	FrontURL        *string `json:"front_url"`
	BackURL         *string `json:"back_url"`
	HasDistinctBack bool    `json:"has_distinct_back"`
}

type cardSummaryResponse struct {
	PrintingUUID    string               `json:"printing_uuid"`
	OracleID        *string              `json:"oracle_id"`
	Name            string               `json:"name"`
	DisplayName     string               `json:"display_name"`
	Set             cardSetResponse      `json:"set"`
	CollectorNumber string               `json:"collector_number"`
	Language        cardLanguageResponse `json:"language"`
	ReleasedAt      *string              `json:"released_at"`
	CMC             float64              `json:"cmc"`
	ManaCost        *string              `json:"mana_cost"`
	Colors          []string             `json:"colors"`
	Rarity          string               `json:"rarity"`
	Layout          string               `json:"layout"`
	Images          cardImagesResponse   `json:"images"`
}

type cardListResponse struct {
	Set      cardSetResponse       `json:"set"`
	Language cardLanguageResponse  `json:"language"`
	Items    []cardSummaryResponse `json:"items"`
	Total    int                   `json:"total"`
}

type cardFaceResponse struct {
	FaceIndex         int     `json:"face_index"`
	Name              string  `json:"name"`
	DisplayName       string  `json:"display_name"`
	ManaCost          *string `json:"mana_cost"`
	TypeLine          *string `json:"type_line"`
	DisplayTypeLine   *string `json:"display_type_line"`
	OracleText        *string `json:"oracle_text"`
	DisplayOracleText *string `json:"display_oracle_text"`
	FlavorText        *string `json:"flavor_text"`
	DisplayFlavorText *string `json:"display_flavor_text"`
	Power             *string `json:"power"`
	Toughness         *string `json:"toughness"`
	Loyalty           *string `json:"loyalty"`
	ImageURL          *string `json:"image_url"`
}

type quantityResponse struct {
	Normal uint64 `json:"normal_quantity"`
	Foil   uint64 `json:"foil_quantity"`
}

type totalQuantityResponse struct {
	quantityResponse
	Total uint64 `json:"total_quantity"`
}

type ownershipTotalsResponse struct {
	Total    uint64 `json:"total_quantity"`
	Distinct uint64 `json:"distinct_cards"`
}

type setOwnershipResponse struct {
	SetCode string `json:"set_code"`
	ownershipTotalsResponse
}

type adjustmentResponse struct {
	PrintingUUID string                  `json:"printing_uuid"`
	OracleID     *string                 `json:"oracle_id"`
	Group        totalQuantityResponse   `json:"group"`
	Collection   ownershipTotalsResponse `json:"collection"`
	SetSummary   *setOwnershipResponse   `json:"set_summary"`
}

type wishlistResponse struct {
	PrintingUUID string  `json:"printing_uuid"`
	OracleID     *string `json:"oracle_id"`
	Included     bool    `json:"included"`
}

type wishlistStateResponse struct {
	Included bool `json:"included"`
}

type cardLanguageOptionResponse struct {
	PrintingUUID string               `json:"printing_uuid"`
	OracleID     *string              `json:"oracle_id"`
	Language     cardLanguageResponse `json:"language"`
	quantityResponse
}

type cardVersionResponse struct {
	Set             cardSetResponse `json:"set"`
	CollectorNumber string          `json:"collector_number"`
	ReleasedAt      *string         `json:"released_at"`
	quantityResponse
	Languages []cardLanguageOptionResponse `json:"languages"`
}

type rulingResponse struct {
	PublishedAt string `json:"published_at"`
	Text        string `json:"text"`
}

type oracleTagResponse struct {
	ID    string `json:"tag_id"`
	Query string `json:"query"`
}

type cardDeckResponse struct {
	ID       string   `json:"deck_id"`
	Name     string   `json:"name"`
	Quantity uint64   `json:"quantity"`
	Sections []string `json:"sections"`
}

type cardDeckUsageResponse struct {
	Total int                `json:"total_decks"`
	Decks []cardDeckResponse `json:"decks"`
}

type cardDetailResponse struct {
	Card              cardSummaryResponse          `json:"card"`
	Faces             []cardFaceResponse           `json:"faces"`
	Collection        quantityResponse             `json:"collection"`
	LogicalCollection totalQuantityResponse        `json:"logical_collection"`
	Wishlist          wishlistStateResponse        `json:"wishlist"`
	Rulings           []rulingResponse             `json:"rulings"`
	Tags              []oracleTagResponse          `json:"oracle_tags"`
	Languages         []cardLanguageOptionResponse `json:"languages"`
	OtherVersions     []cardVersionResponse        `json:"other_versions"`
	DeckUsage         cardDeckUsageResponse        `json:"deck_usage"`
}

type adjustmentRequest struct {
	Finish *string `json:"finish"`
	Delta  *int    `json:"delta"`
}

type wishlistRequest struct {
	Included *bool `json:"included"`
}

func cardDate(value *time.Time) *string {
	if value == nil {
		return nil
	}
	date := value.Format("2006-01-02")
	return &date
}

func newCardSet(s cards.Set) cardSetResponse {
	return cardSetResponse{Code: s.Code, Name: s.Name, DisplayName: s.DisplayName, ReleasedAt: cardDate(s.ReleasedAt)}
}

func newCardLanguage(l cards.Language) cardLanguageResponse {
	return cardLanguageResponse{Code: l.Code, DisplayName: l.DisplayName, SortOrder: l.SortOrder}
}

func newQuantity(q cards.Quantity) quantityResponse {
	return quantityResponse{Normal: q.Normal, Foil: q.Foil}
}

func newTotalQuantity(q cards.Quantity) totalQuantityResponse {
	return totalQuantityResponse{quantityResponse: newQuantity(q), Total: q.Normal + q.Foil}
}

func newCardSummary(c cards.Summary) cardSummaryResponse {
	return cardSummaryResponse{PrintingUUID: c.PrintingUUID, OracleID: c.OracleID, Name: c.Name, DisplayName: c.DisplayName,
		Set: newCardSet(c.Set), CollectorNumber: c.CollectorNumber, Language: newCardLanguage(c.Language), ReleasedAt: cardDate(c.ReleasedAt),
		CMC: c.CMC, ManaCost: c.ManaCost, Colors: append([]string{}, c.Colors...), Rarity: c.Rarity, Layout: c.Layout,
		Images: cardImagesResponse{FrontURL: c.Images.FrontURL, BackURL: c.Images.BackURL, HasDistinctBack: c.Images.BackURL != nil}}
}

func newCardList(list cards.List) cardListResponse {
	r := cardListResponse{Set: newCardSet(list.Set), Language: newCardLanguage(list.Language), Items: make([]cardSummaryResponse, 0, len(list.Items)), Total: len(list.Items)}
	for _, item := range list.Items {
		r.Items = append(r.Items, newCardSummary(item))
	}
	return r
}

func newCardLanguages(options []cards.LanguageOption) []cardLanguageOptionResponse {
	r := make([]cardLanguageOptionResponse, 0, len(options))
	for _, option := range options {
		r = append(r, cardLanguageOptionResponse{PrintingUUID: option.PrintingUUID, OracleID: option.OracleID, Language: newCardLanguage(option.Language), quantityResponse: newQuantity(option.Quantity)})
	}
	return r
}

func newCardDetail(d cards.Detail) cardDetailResponse {
	r := cardDetailResponse{Card: newCardSummary(d.Card), Collection: newQuantity(d.Collection), LogicalCollection: newTotalQuantity(d.LogicalCollection),
		Wishlist: wishlistStateResponse{Included: d.Included}, Faces: []cardFaceResponse{}, Rulings: []rulingResponse{}, Tags: []oracleTagResponse{},
		Languages: newCardLanguages(d.Languages), OtherVersions: []cardVersionResponse{}, DeckUsage: cardDeckUsageResponse{Total: len(d.Decks), Decks: []cardDeckResponse{}}}
	for _, f := range d.Faces {
		r.Faces = append(r.Faces, cardFaceResponse{FaceIndex: f.FaceIndex, Name: f.Name, DisplayName: f.DisplayName, ManaCost: f.ManaCost,
			TypeLine: f.TypeLine, DisplayTypeLine: f.DisplayTypeLine, OracleText: f.OracleText, DisplayOracleText: f.DisplayOracleText,
			FlavorText: f.FlavorText, DisplayFlavorText: f.DisplayFlavorText, Power: f.Power, Toughness: f.Toughness, Loyalty: f.Loyalty, ImageURL: f.ImageURL})
	}
	for _, v := range d.OtherVersions {
		r.OtherVersions = append(r.OtherVersions, cardVersionResponse{Set: newCardSet(v.Set), CollectorNumber: v.CollectorNumber, ReleasedAt: cardDate(v.ReleasedAt), quantityResponse: newQuantity(v.Quantity), Languages: newCardLanguages(v.Languages)})
	}
	for _, ruling := range d.Rulings {
		r.Rulings = append(r.Rulings, rulingResponse{PublishedAt: ruling.PublishedAt.Format("2006-01-02"), Text: ruling.Text})
	}
	for _, tag := range d.Tags {
		r.Tags = append(r.Tags, oracleTagResponse{ID: tag.ID, Query: tag.Query})
	}
	for _, deck := range d.Decks {
		r.DeckUsage.Decks = append(r.DeckUsage.Decks, cardDeckResponse{ID: strconv.FormatUint(deck.ID, 10), Name: deck.Name, Quantity: deck.Quantity, Sections: append([]string{}, deck.Sections...)})
	}
	return r
}

func newAdjustment(state cards.CollectionState) adjustmentResponse {
	r := adjustmentResponse{PrintingUUID: state.Card.PrintingUUID, OracleID: state.Card.OracleID, Group: newTotalQuantity(state.Group),
		Collection: ownershipTotalsResponse{Total: state.Collection.TotalQuantity, Distinct: state.Collection.DistinctCards}}
	if state.Card.Set.Code != "" {
		r.SetSummary = &setOwnershipResponse{SetCode: state.Card.Set.Code, ownershipTotalsResponse: ownershipTotalsResponse{Total: state.SetSummary.TotalQuantity, Distinct: state.SetSummary.DistinctCards}}
	}
	return r
}
