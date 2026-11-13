package httpapi

import (
	"log/slog"

	"github.com/gin-gonic/gin"

	"mtgnissa/internal/apperror"
	"mtgnissa/internal/config"
	"mtgnissa/internal/health"
	"mtgnissa/internal/httpresponse"
)

// Dependencies contains runtime settings and services used by the HTTP router.
type Dependencies struct {
	Logger        *slog.Logger
	Health        health.Handler
	Loader        cardDataLoader
	LoadEnabled   bool
	App           AppConfig
	Sets          setLister
	Tokens        tokenLister
	Cards         cardService
	Collection    collectionLister
	Wishlist      wishlistLister
	Decks         deckService
	Search        searchLister
	OperationLogs operationLogService
}

// New assembles the HTTP router, global middleware, health checks, and API modules.
func New(deps Dependencies) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.HandleMethodNotAllowed = true
	router.Use(requestID(), accessLog(deps.Logger), recovery(), requestTimeout(config.HTTPRequestTimeout), bodyLimit(1<<20))

	router.GET("/health/live", deps.Health.Live)
	router.GET("/health/ready", deps.Health.Ready)

	api := router.Group("/api/v1")
	api.GET("/app", rejectUnknownQueryParameters(), getApp(deps.App))
	api.GET("/search", rejectUnknownQueryParameters("q", "cursor"), getSearch(deps.Search))
	api.GET("/operation_logs/summary", rejectUnknownQueryParameters("start_date", "end_date", "page"), getOperationSummary(deps.OperationLogs))
	api.GET("/operation_logs", rejectUnknownQueryParameters("operation_date", "action_type", "page"), getOperationLogs(deps.OperationLogs))
	api.POST("/card_data/load", requireJSONContentType(), rejectUnknownQueryParameters(), startLoad(deps.Loader, deps.LoadEnabled))
	api.GET("/card_data/load/:task_id", rejectUnknownQueryParameters(), getLoad(deps.Loader, deps.LoadEnabled))
	api.GET("/sets", rejectUnknownQueryParameters("q", "language", "set_type", "sort", "cursor"), getSets(deps.Sets))
	api.GET("/sets/options", rejectUnknownQueryParameters(), getSetOptions(deps.Sets))
	api.GET("/tokens", rejectUnknownQueryParameters("set_code"), getTokens(deps.Tokens))
	api.GET("/cards", rejectUnknownQueryParameters("set_code", "language", "q"), getCards(deps.Cards))
	api.GET("/cards/:printing_uuid", rejectUnknownQueryParameters(), getCard(deps.Cards))
	api.POST("/cards/:printing_uuid/collection_adjustments", requireJSONContentType(), rejectUnknownQueryParameters(), adjustCard(deps.Cards))
	api.POST("/cards/:printing_uuid/wishlist_state", requireJSONContentType(), rejectUnknownQueryParameters(), setCardWishlist(deps.Cards))
	api.GET("/collection/options", rejectUnknownQueryParameters(), getCollectionOptions(deps.Collection))
	api.GET("/collection", rejectUnknownQueryParameters("q", "set_code", "language", "color", "mana_value", "rarity", "sort", "merge_sets", "merge_languages", "cursor"), getCollection(deps.Collection))
	api.GET("/wishlist/options", rejectUnknownQueryParameters(), getWishlistOptions(deps.Wishlist))
	api.GET("/wishlist", rejectUnknownQueryParameters("q", "set_code", "language", "color", "mana_value", "rarity", "sort", "cursor"), getWishlist(deps.Wishlist))
	api.GET("/decks", rejectUnknownQueryParameters("q", "sort", "cursor"), getDecks(deps.Decks))
	api.POST("/decks", requireJSONContentType(), rejectUnknownQueryParameters(), createDeck(deps.Decks))
	api.GET("/decks/:deck_id", rejectUnknownQueryParameters(), getDeck(deps.Decks))
	api.POST("/decks/:deck_id/save", requireJSONContentType(), rejectUnknownQueryParameters(), saveDeck(deps.Decks))
	api.POST("/decks/:deck_id/delete", rejectUnknownQueryParameters(), deleteDeck(deps.Decks))
	api.POST("/decks/:deck_id/copies", rejectUnknownQueryParameters(), copyDeck(deps.Decks))
	api.POST("/decks/:deck_id/parse_code", requireJSONContentType(), rejectUnknownQueryParameters(), parseDeckCode(deps.Decks))
	api.GET("/decks/:deck_id/cards/search", rejectUnknownQueryParameters("q", "cursor"), searchDeckCards(deps.Decks))
	api.GET("/decks/:deck_id/cards/name_suggestions", rejectUnknownQueryParameters("q"), deckNameSuggestions(deps.Decks))
	api.POST("/decks/:deck_id/card_contexts", requireJSONContentType(), rejectUnknownQueryParameters(), deckCardContexts(deps.Decks))

	registerFallbackRoutes(router)
	return router
}

func registerFallbackRoutes(router *gin.Engine) {
	router.NoMethod(func(c *gin.Context) {
		httpresponse.WriteAppError(c, apperror.New(apperror.CodeMethodNotAllowed, "method not allowed"))
	})
	router.NoRoute(func(c *gin.Context) {
		httpresponse.WriteAppError(c, apperror.New(apperror.CodeNotFound, "route not found"))
	})
}
