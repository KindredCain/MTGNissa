package httpapi

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"mtgnissa/internal/apperror"
	"mtgnissa/internal/httpresponse"
	"mtgnissa/internal/model"
)

type cardService interface {
	List(context.Context, string, string, string) (model.CardList, error)
	Detail(context.Context, string) (model.CardDetail, error)
	Adjust(context.Context, string, string, int, string) (model.CollectionState, bool, error)
	SetWishlist(context.Context, string, bool, string) (model.CardSummary, bool, bool, error)
}

func getCards(service cardService) gin.HandlerFunc {
	return func(c *gin.Context) {
		code, language, query, err := parseCardList(c.Request.URL.Query())
		if err != nil {
			httpresponse.WriteAppError(c, err)
			return
		}
		list, err := service.List(c.Request.Context(), code, language, query)
		if err != nil {
			httpresponse.WriteAppError(c, err)
			return
		}
		httpresponse.WriteJSON(c, http.StatusOK, newCardList(list))
	}
}

func parseCardList(query url.Values) (string, string, string, error) {
	for _, field := range []string{"set_code", "language", "q"} {
		if len(query[field]) > 1 {
			return "", "", "", apperror.New(apperror.CodeInvalidRequest, field+" must not be repeated")
		}
	}
	for _, field := range []string{"set_code", "language", "q"} {
		value := strings.TrimSpace(query.Get(field))
		limit := 512
		if field == "set_code" {
			limit = 16
		}
		if field == "language" {
			limit = 8
		}
		if value == "" && field != "q" {
			return "", "", "", validationError(field, "required")
		}
		if containsNUL(value) || utf8.RuneCountInString(value) > limit {
			return "", "", "", validationError(field, "invalid_length_or_nul")
		}
	}
	return strings.TrimSpace(query.Get("set_code")), strings.TrimSpace(query.Get("language")), strings.TrimSpace(query.Get("q")), nil
}

func printingID(value string) (string, error) {
	if !model.ValidUUID(value) {
		return "", apperror.New(apperror.CodeCardNotFound, "card not found")
	}
	return strings.ToLower(value), nil
}

func getCard(service cardService) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := printingID(c.Param("printing_uuid"))
		if err != nil {
			httpresponse.WriteAppError(c, err)
			return
		}
		d, err := service.Detail(c.Request.Context(), id)
		if err != nil {
			httpresponse.WriteAppError(c, err)
			return
		}
		httpresponse.WriteJSON(c, http.StatusOK, newCardDetail(d))
	}
}

func decodeCardRequest(reader io.Reader, target any) error {
	decoder := jsonDecoder(reader)
	if err := decoder.Decode(target); err != nil {
		return requestDecodeError(err, "invalid JSON request body")
	}
	if err := ensureEOF(decoder); err != nil {
		return requestDecodeError(err, "invalid JSON request body")
	}
	return nil
}

func mutationKey(values []string) (string, error) {
	if len(values) != 1 {
		return "", apperror.New(apperror.CodeIdempotencyRequired, "one canonical UUID Idempotency-Key is required")
	}
	key := strings.ToLower(values[0])
	if !model.ValidUUID(key) || key[14] < '1' || key[14] > '5' || !strings.ContainsRune("89ab", rune(key[19])) {
		return "", apperror.New(apperror.CodeIdempotencyRequired, "Idempotency-Key must be an RFC 4122 UUID")
	}
	return key, nil
}

func adjustCard(service cardService) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := printingID(c.Param("printing_uuid"))
		if err != nil {
			httpresponse.WriteAppError(c, err)
			return
		}
		var request adjustmentRequest
		if err := decodeCardRequest(c.Request.Body, &request); err != nil {
			httpresponse.WriteAppError(c, err)
			return
		}
		if request.Finish == nil || (*request.Finish != "normal" && *request.Finish != "foil") {
			httpresponse.WriteAppError(c, validationError("finish", "must_be_normal_or_foil"))
			return
		}
		if request.Delta == nil || (*request.Delta != 1 && *request.Delta != -1) {
			httpresponse.WriteAppError(c, validationError("delta", "must_be_1_or_minus_1"))
			return
		}
		key, err := mutationKey(c.Request.Header.Values("Idempotency-Key"))
		if err != nil {
			httpresponse.WriteAppError(c, err)
			return
		}
		state, deduplicated, err := service.Adjust(c.Request.Context(), id, *request.Finish, *request.Delta, key)
		if err != nil {
			httpresponse.WriteAppError(c, err)
			return
		}
		if deduplicated {
			c.Header("Idempotency-Deduplicated", "true")
		}
		httpresponse.WriteJSON(c, http.StatusOK, newAdjustment(state))
	}
}

func setCardWishlist(service cardService) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := printingID(c.Param("printing_uuid"))
		if err != nil {
			httpresponse.WriteAppError(c, err)
			return
		}
		var request wishlistRequest
		if err := decodeCardRequest(c.Request.Body, &request); err != nil {
			httpresponse.WriteAppError(c, err)
			return
		}
		if request.Included == nil {
			httpresponse.WriteAppError(c, validationError("included", "required"))
			return
		}
		key, err := mutationKey(c.Request.Header.Values("Idempotency-Key"))
		if err != nil {
			httpresponse.WriteAppError(c, err)
			return
		}
		card, included, deduplicated, err := service.SetWishlist(c.Request.Context(), id, *request.Included, key)
		if err != nil {
			httpresponse.WriteAppError(c, err)
			return
		}
		if deduplicated {
			c.Header("Idempotency-Deduplicated", "true")
		}
		httpresponse.WriteJSON(c, http.StatusOK, wishlistResponse{PrintingUUID: card.PrintingUUID, OracleID: card.OracleID, Included: included})
	}
}
