package appdata

import "time"

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

func (a ActionType) Valid() bool { return a >= ActionAddCard && a <= ActionModifyDeck }

func (a ActionType) IsCardAction() bool {
	return a >= ActionAddCard && a <= ActionRemoveWishlist
}

func (a ActionType) IsDeckAction() bool {
	return a >= ActionCreateDeck && a <= ActionModifyDeck
}

type FinishKind uint8

const (
	FinishNormal FinishKind = iota + 1
	FinishFoil
)

func (f FinishKind) Valid() bool { return f == FinishNormal || f == FinishFoil }

type CardPrintRef struct {
	ScryfallID      string    `db:"scryfall_id" json:"scryfall_id"`
	OracleID        *string   `db:"oracle_id" json:"oracle_id,omitempty"`
	SetID           string    `db:"set_id" json:"set_id"`
	SetCode         string    `db:"set_code" json:"set_code"`
	CollectorNumber string    `db:"collector_number" json:"collector_number"`
	Lang            string    `db:"lang" json:"lang"`
	CreatedAt       time.Time `db:"created_at" json:"created_at"`
	LastVerifiedAt  time.Time `db:"last_verified_at" json:"last_verified_at"`
}

type CollectionItem struct {
	ScryfallID     string    `db:"scryfall_id" json:"scryfall_id"`
	NormalQuantity uint32    `db:"normal_quantity" json:"normal_quantity"`
	FoilQuantity   uint32    `db:"foil_quantity" json:"foil_quantity"`
	CreatedAt      time.Time `db:"created_at" json:"created_at"`
	UpdatedAt      time.Time `db:"updated_at" json:"updated_at"`
}

type WishlistItem struct {
	ScryfallID string    `db:"scryfall_id" json:"scryfall_id"`
	CreatedAt  time.Time `db:"created_at" json:"created_at"`
}

type Deck struct {
	ID         uint64    `db:"id" json:"id"`
	Name       string    `db:"name" json:"name"`
	SourceText string    `db:"source_text" json:"source_text"`
	Version    uint64    `db:"version" json:"version"`
	CreatedAt  time.Time `db:"created_at" json:"created_at"`
	UpdatedAt  time.Time `db:"updated_at" json:"updated_at"`
}

type DeckCard struct {
	DeckID       uint64      `db:"deck_id" json:"deck_id"`
	SectionType  DeckSection `db:"section_type" json:"section_type"`
	OracleID     string      `db:"oracle_id" json:"oracle_id"`
	Quantity     uint32      `db:"quantity" json:"quantity"`
	NameSnapshot string      `db:"name_snapshot" json:"name_snapshot"`
}

type OperationLog struct {
	ID            uint64     `db:"id" json:"id"`
	ActionType    ActionType `db:"action_type" json:"action_type"`
	OperationDate time.Time  `db:"operation_date" json:"operation_date"`
	OccurredAt    time.Time  `db:"occurred_at" json:"occurred_at"`
}

type OperationCardDetail struct {
	OperationID     uint64      `db:"operation_id" json:"operation_id"`
	ScryfallID      string      `db:"scryfall_id" json:"scryfall_id"`
	OracleID        *string     `db:"oracle_id" json:"oracle_id,omitempty"`
	CardName        string      `db:"card_name" json:"card_name"`
	SetCode         string      `db:"set_code" json:"set_code"`
	CollectorNumber string      `db:"collector_number" json:"collector_number"`
	Lang            string      `db:"lang" json:"lang"`
	FinishKind      *FinishKind `db:"finish_kind" json:"finish_kind,omitempty"`
	QuantityDelta   *int64      `db:"quantity_delta" json:"quantity_delta,omitempty"`
}

type OperationDeckDetail struct {
	OperationID uint64 `db:"operation_id" json:"operation_id"`
	DeckID      uint64 `db:"deck_id" json:"deck_id"`
	DeckName    string `db:"deck_name" json:"deck_name"`
}
