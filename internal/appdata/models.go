package appdata

import "time"

// DeckSection identifies the functional section containing a card in a deck.
type DeckSection string

const (
	DeckSectionMainboard  DeckSection = "mainboard"
	DeckSectionSideboard  DeckSection = "sideboard"
	DeckSectionCommander  DeckSection = "commander"
	DeckSectionMaybeboard DeckSection = "maybeboard"
)

func (s DeckSection) Valid() bool {
	switch s {
	case DeckSectionMainboard, DeckSectionSideboard, DeckSectionCommander, DeckSectionMaybeboard:
		return true
	default:
		return false
	}
}

// ActionType identifies the collection, wishlist, or deck operation being recorded.
type ActionType uint8

const (
	ActionAddCard ActionType = iota + 1
	ActionRemoveCard
	ActionAddWishlist
	ActionRemoveWishlist
	ActionCreateDeck
	ActionDeleteDeck
	ActionCopyDeck
	ActionModifyDeck
)

func (a ActionType) Valid() bool {
	return a >= ActionAddCard && a <= ActionModifyDeck
}

func (a ActionType) IsCardAction() bool {
	return a >= ActionAddCard && a <= ActionRemoveWishlist
}

func (a ActionType) IsDeckAction() bool {
	return a >= ActionCreateDeck && a <= ActionModifyDeck
}

// FinishKind identifies the physical finish of a card involved in an operation.
type FinishKind uint8

const (
	FinishNormal FinishKind = iota + 1
	FinishFoil
)

func (f FinishKind) Valid() bool {
	return f == FinishNormal || f == FinishFoil
}

// CardPrintRef stores the stable card-print identity referenced by application data.
type CardPrintRef struct {
	ScryfallID      string    `db:"scryfall_id"`
	OracleID        *string   `db:"oracle_id"`
	SetID           string    `db:"set_id"`
	SetCode         string    `db:"set_code"`
	CollectorNumber string    `db:"collector_number"`
	Lang            string    `db:"lang"`
	CreatedAt       time.Time `db:"created_at"`
	LastVerifiedAt  time.Time `db:"last_verified_at"`
}

// CollectionItem stores owned normal and foil quantities for one card print.
type CollectionItem struct {
	ScryfallID     string    `db:"scryfall_id"`
	NormalQuantity uint32    `db:"normal_quantity"`
	FoilQuantity   uint32    `db:"foil_quantity"`
	CreatedAt      time.Time `db:"created_at"`
	UpdatedAt      time.Time `db:"updated_at"`
}

// WishlistItem records when a card print was added to the wishlist.
type WishlistItem struct {
	ScryfallID string    `db:"scryfall_id"`
	CreatedAt  time.Time `db:"created_at"`
}

// Deck stores a deck definition, its source text, and optimistic-lock version.
type Deck struct {
	ID         uint64    `db:"id"`
	Name       string    `db:"name"`
	SourceText string    `db:"source_text"`
	Version    uint64    `db:"version"`
	CreatedAt  time.Time `db:"created_at"`
	UpdatedAt  time.Time `db:"updated_at"`
}

// DeckCard stores a card quantity and name snapshot within a deck section.
type DeckCard struct {
	DeckID       uint64      `db:"deck_id"`
	SectionType  DeckSection `db:"section_type"`
	OracleID     string      `db:"oracle_id"`
	Quantity     uint32      `db:"quantity"`
	NameSnapshot string      `db:"name_snapshot"`
}

// OperationLog records the type and time of a collection, wishlist, or deck change.
type OperationLog struct {
	ID            uint64     `db:"id"`
	ActionType    ActionType `db:"action_type"`
	OperationDate time.Time  `db:"operation_date"`
	OccurredAt    time.Time  `db:"occurred_at"`
}

// OperationCardDetail stores the card-specific snapshot and quantity change for an operation.
type OperationCardDetail struct {
	OperationID     uint64      `db:"operation_id"`
	ScryfallID      string      `db:"scryfall_id"`
	OracleID        *string     `db:"oracle_id"`
	CardName        string      `db:"card_name"`
	SetCode         string      `db:"set_code"`
	CollectorNumber string      `db:"collector_number"`
	Lang            string      `db:"lang"`
	FinishKind      *FinishKind `db:"finish_kind"`
	QuantityDelta   *int64      `db:"quantity_delta"`
}

// OperationDeckDetail stores the deck identity and name snapshot for an operation.
type OperationDeckDetail struct {
	OperationID uint64 `db:"operation_id"`
	DeckID      uint64 `db:"deck_id"`
	DeckName    string `db:"deck_name"`
}
