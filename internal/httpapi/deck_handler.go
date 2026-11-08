package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"mtgnissa/internal/apperror"
	"mtgnissa/internal/httpresponse"
	"mtgnissa/internal/model"
)

type deckService interface {
	List(context.Context, model.DeckListParams) (model.DeckPage, error)
	Detail(context.Context, uint64) (model.DeckDetail, error)
	Create(context.Context, string, string) (model.DeckIdentity, bool, error)
	Copy(context.Context, uint64, string) (model.DeckIdentity, bool, error)
	Save(context.Context, uint64, model.DeckSave) (model.DeckDetail, error)
	Delete(context.Context, uint64) error
	ParseCode(context.Context, uint64, string) (model.ParsedDeck, error)
	SearchCards(context.Context, uint64, model.DeckSearchParams) (model.DeckCandidatePage, error)
	NameSuggestions(context.Context, uint64, string) ([]string, error)
	CardContexts(context.Context, uint64, []string) ([]model.DeckEditCardContext, error)
}

func deckID(value string) (uint64, error) {
	if value == "" || len(value) > 20 {
		return 0, apperror.New(apperror.CodeDeckNotFound, "deck not found")
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return 0, apperror.New(apperror.CodeDeckNotFound, "deck not found")
		}
	}
	id, err := strconv.ParseUint(value, 10, 64)
	if err != nil || id == 0 {
		return 0, apperror.New(apperror.CodeDeckNotFound, "deck not found")
	}
	return id, nil
}

func parseDeckQuery(query url.Values) (model.DeckListParams, error) {
	params := model.DeckListParams{Query: strings.TrimSpace(query.Get("q")), Sort: "updated_desc", Cursor: query.Get("cursor")}
	if err := validateDeckQuery(query, "q", "sort", "cursor"); err != nil {
		return params, err
	}
	if value := query.Get("sort"); value != "" {
		params.Sort = value
	}
	if !oneOf(params.Sort, "updated_desc", "created_desc", "name_asc", "card_count_desc") {
		return params, apperror.New(apperror.CodeInvalidRequest, "sort is invalid")
	}
	return params, nil
}

func validateDeckQuery(query url.Values, fields ...string) error {
	for _, field := range fields {
		if len(query[field]) > 1 {
			return apperror.New(apperror.CodeInvalidRequest, field+" must not be repeated")
		}
	}
	q := strings.TrimSpace(query.Get("q"))
	if containsNUL(q) || utf8.RuneCountInString(q) > 512 {
		return validationError("q", "must_not_contain_nul_and_must_be_at_most_512_characters")
	}
	if containsNUL(query.Get("cursor")) {
		return validationError("cursor", "must_not_contain_nul")
	}
	return nil
}

func getDecks(service deckService) gin.HandlerFunc {
	return func(c *gin.Context) {
		params, err := parseDeckQuery(c.Request.URL.Query())
		if err != nil {
			httpresponse.WriteAppError(c, err)
			return
		}
		page, err := service.List(c.Request.Context(), params)
		if err != nil {
			httpresponse.WriteAppError(c, err)
			return
		}
		httpresponse.WriteJSON(c, http.StatusOK, newDeckPage(page))
	}
}

func getDeck(service deckService) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := deckID(c.Param("deck_id"))
		if err != nil {
			httpresponse.WriteAppError(c, err)
			return
		}
		detail, err := service.Detail(c.Request.Context(), id)
		if err != nil {
			httpresponse.WriteAppError(c, err)
			return
		}
		httpresponse.WriteJSON(c, http.StatusOK, newDeckDetail(detail))
	}
}

func createDeck(service deckService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var request deckNameRequest
		if err := decodeCardRequest(c.Request.Body, &request); err != nil {
			httpresponse.WriteAppError(c, err)
			return
		}
		if request.Name == nil {
			httpresponse.WriteAppError(c, validationError("name", "required"))
			return
		}
		key, err := mutationKey(c.Request.Header.Values("Idempotency-Key"))
		if err != nil {
			httpresponse.WriteAppError(c, err)
			return
		}
		identity, duplicate, err := service.Create(c.Request.Context(), *request.Name, key)
		writeDeckIdentity(c, identity, duplicate, err)
	}
}

func copyDeck(service deckService) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := deckID(c.Param("deck_id"))
		if err != nil {
			httpresponse.WriteAppError(c, err)
			return
		}
		if err := noDeckBody(c.Request.Body); err != nil {
			httpresponse.WriteAppError(c, err)
			return
		}
		key, err := mutationKey(c.Request.Header.Values("Idempotency-Key"))
		if err != nil {
			httpresponse.WriteAppError(c, err)
			return
		}
		identity, duplicate, err := service.Copy(c.Request.Context(), id, key)
		writeDeckIdentity(c, identity, duplicate, err)
	}
}

func writeDeckIdentity(c *gin.Context, identity model.DeckIdentity, duplicate bool, err error) {
	if duplicate {
		c.Header("Idempotency-Deduplicated", "true")
	}
	if err != nil {
		httpresponse.WriteAppError(c, err)
		return
	}
	status := http.StatusCreated
	if duplicate {
		status = http.StatusOK
	}
	id := strconv.FormatUint(identity.ID, 10)
	c.Header("Location", "/api/v1/decks/"+id)
	httpresponse.WriteJSON(c, status, deckIdentityResponse{DeckID: id, Name: identity.Name})
}

func deleteDeck(service deckService) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := deckID(c.Param("deck_id"))
		if err != nil {
			httpresponse.WriteAppError(c, err)
			return
		}
		if err := noDeckBody(c.Request.Body); err != nil {
			httpresponse.WriteAppError(c, err)
			return
		}
		if err := service.Delete(c.Request.Context(), id); err != nil {
			httpresponse.WriteAppError(c, err)
			return
		}
		httpresponse.WriteJSON(c, http.StatusOK, deckDeleteResponse{DeckID: strconv.FormatUint(id, 10), Deleted: true})
	}
}

func noDeckBody(reader io.Reader) error {
	raw, err := io.ReadAll(reader)
	if err != nil {
		return requestDecodeError(err, "could not read request body")
	}
	if strings.TrimSpace(string(raw)) != "" {
		return apperror.New(apperror.CodeInvalidRequest, "this operation does not accept a request body")
	}
	return nil
}

func saveDeck(service deckService) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := deckID(c.Param("deck_id"))
		if err != nil {
			httpresponse.WriteAppError(c, err)
			return
		}
		var request deckSaveRequest
		if err := decodeCardRequest(c.Request.Body, &request); err != nil {
			httpresponse.WriteAppError(c, err)
			return
		}
		input, err := parseDeckSave(request)
		if err != nil {
			httpresponse.WriteAppError(c, err)
			return
		}
		detail, err := service.Save(c.Request.Context(), id, input)
		if err != nil {
			httpresponse.WriteAppError(c, err)
			return
		}
		httpresponse.WriteJSON(c, http.StatusOK, newDeckDetail(detail))
	}
}

func parseDeckSave(request deckSaveRequest) (model.DeckSave, error) {
	result := model.DeckSave{Sections: []model.DeckSaveSection{}}
	if request.Name == nil {
		return result, validationError("name", "required")
	}
	result.Name = *request.Name
	if len(request.CoverPrintingUUID) == 0 {
		return result, validationError("cover_printing_uuid", "required")
	}
	if err := json.Unmarshal(request.CoverPrintingUUID, &result.CoverPrintingUUID); err != nil {
		return result, requestDecodeError(err, "cover_printing_uuid must be a string or null")
	}
	if result.CoverPrintingUUID != nil {
		if !model.ValidUUID(*result.CoverPrintingUUID) {
			return result, validationError("cover_printing_uuid", "must_be_a_canonical_uuid")
		}
		value := strings.ToLower(*result.CoverPrintingUUID)
		result.CoverPrintingUUID = &value
	}
	if request.Sections == nil {
		return result, validationError("sections", "required")
	}
	seen := map[string]bool{}
	for i, section := range *request.Sections {
		field := fmt.Sprintf("sections[%d]", i)
		if section.Type == nil {
			return result, validationError(field+".type", "required")
		}
		if !oneOf(*section.Type, "commander", "mainboard", "sideboard", "maybeboard") || seen[*section.Type] {
			return result, apperror.New(apperror.CodeInvalidRequest, "section types must be valid and unique")
		}
		seen[*section.Type] = true
		if section.Cards == nil {
			return result, validationError(field+".cards", "required")
		}
		row := model.DeckSaveSection{Type: *section.Type, Cards: []model.DeckSaveCard{}}
		for j, card := range *section.Cards {
			cardField := fmt.Sprintf("%s.cards[%d]", field, j)
			if card.PrintingUUID == nil || !model.ValidUUID(*card.PrintingUUID) {
				return result, validationError(cardField+".printing_uuid", "must_be_a_canonical_uuid")
			}
			if card.Quantity == nil || *card.Quantity <= 0 || *card.Quantity > math.MaxUint32 {
				return result, validationError(cardField+".quantity", "must_be_1_to_4294967295")
			}
			row.Cards = append(row.Cards, model.DeckSaveCard{PrintingUUID: strings.ToLower(*card.PrintingUUID), Quantity: uint64(*card.Quantity)})
		}
		result.Sections = append(result.Sections, row)
	}
	return result, nil
}

func parseDeckCode(service deckService) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := deckID(c.Param("deck_id"))
		if err != nil {
			httpresponse.WriteAppError(c, err)
			return
		}
		var request deckCodeRequest
		if err := decodeCardRequest(c.Request.Body, &request); err != nil {
			httpresponse.WriteAppError(c, err)
			return
		}
		if request.SourceText == nil {
			httpresponse.WriteAppError(c, validationError("source_text", "required"))
			return
		}
		parsed, err := service.ParseCode(c.Request.Context(), id, *request.SourceText)
		if err != nil {
			httpresponse.WriteAppError(c, err)
			return
		}
		httpresponse.WriteJSON(c, http.StatusOK, newParsedDeck(parsed))
	}
}

func searchDeckCards(service deckService) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := deckID(c.Param("deck_id"))
		if err != nil {
			httpresponse.WriteAppError(c, err)
			return
		}
		query := c.Request.URL.Query()
		if err := validateDeckQuery(query, "q", "cursor"); err != nil {
			httpresponse.WriteAppError(c, err)
			return
		}
		page, err := service.SearchCards(c.Request.Context(), id, model.DeckSearchParams{Query: strings.TrimSpace(query.Get("q")), Cursor: query.Get("cursor")})
		if err != nil {
			httpresponse.WriteAppError(c, err)
			return
		}
		httpresponse.WriteJSON(c, http.StatusOK, newDeckCandidates(page))
	}
}

func deckNameSuggestions(service deckService) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := deckID(c.Param("deck_id"))
		if err != nil {
			httpresponse.WriteAppError(c, err)
			return
		}
		query := c.Request.URL.Query()
		if err := validateDeckQuery(query, "q"); err != nil {
			httpresponse.WriteAppError(c, err)
			return
		}
		names, err := service.NameSuggestions(c.Request.Context(), id, query.Get("q"))
		if err != nil {
			httpresponse.WriteAppError(c, err)
			return
		}
		result := deckNameSuggestionsResponse{Items: []deckNameSuggestionResponse{}}
		for _, name := range names {
			result.Items = append(result.Items, deckNameSuggestionResponse{PrintedName: name})
		}
		httpresponse.WriteJSON(c, http.StatusOK, result)
	}
}

func deckCardContexts(service deckService) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := deckID(c.Param("deck_id"))
		if err != nil {
			httpresponse.WriteAppError(c, err)
			return
		}
		var request deckContextsRequest
		if err := decodeCardRequest(c.Request.Body, &request); err != nil {
			httpresponse.WriteAppError(c, err)
			return
		}
		ids, err := parseDeckContextIDs(request)
		if err != nil {
			httpresponse.WriteAppError(c, err)
			return
		}
		items, err := service.CardContexts(c.Request.Context(), id, ids)
		if err != nil {
			httpresponse.WriteAppError(c, err)
			return
		}
		result := deckContextsResponse{Items: []deckEditContextResponse{}}
		for _, item := range items {
			result.Items = append(result.Items, deckEditContextResponse{RequestedPrintingUUID: item.RequestedPrintingUUID, deckContextResponse: newDeckContext(item)})
		}
		httpresponse.WriteJSON(c, http.StatusOK, result)
	}
}

func parseDeckContextIDs(request deckContextsRequest) ([]string, error) {
	if request.PrintingUUIDs == nil || len(*request.PrintingUUIDs) == 0 || len(*request.PrintingUUIDs) > 200 {
		return nil, validationError("printing_uuids", "must_contain_1_to_200_unique_printing_uuids")
	}
	seen := map[string]bool{}
	for i, id := range *request.PrintingUUIDs {
		if !model.ValidUUID(id) || seen[strings.ToLower(id)] {
			return nil, validationError(fmt.Sprintf("printing_uuids[%d]", i), "must_be_a_unique_canonical_uuid")
		}
		seen[strings.ToLower(id)] = true
	}
	return *request.PrintingUUIDs, nil
}
