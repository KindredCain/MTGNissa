// Package cards provides ordinary printing queries and personal card state changes.
package cards

import "time"

// Set identifies the series containing a printing.
type Set struct {
	Code        string
	Name        string
	DisplayName string
	ReleasedAt  *time.Time
}

// Language describes an actual printing language from the Card DB dictionary.
type Language struct {
	Code        string
	DisplayName string
	SortOrder   int
}

// Images contains resolved picture sources without changing printing identity.
type Images struct {
	FrontURL *string
	BackURL  *string
}

// Summary describes one language printing using its canonical main-face UUID.
type Summary struct {
	PrintingUUID    string
	OracleID        *string
	Name            string
	DisplayName     string
	Set             Set
	CollectorNumber string
	Language        Language
	ReleasedAt      *time.Time
	CMC             float64
	ManaCost        *string
	Colors          []string
	Rarity          string
	Layout          string
	Images          Images
}

// Face contains the original and translated text of a printing face.
type Face struct {
	FaceIndex         int
	Name              string
	DisplayName       string
	ManaCost          *string
	TypeLine          *string
	DisplayTypeLine   *string
	OracleText        *string
	DisplayOracleText *string
	FlavorText        *string
	DisplayFlavorText *string
	Power             *string
	Toughness         *string
	Loyalty           *string
	ImageURL          *string
}

// List is the complete lightweight result for one ordinary series and language.
type List struct {
	Set      Set
	Language Language
	Items    []Summary
}

// Quantity holds the two independent collection finish counters.
type Quantity struct {
	Normal uint64 `db:"normal_quantity"`
	Foil   uint64 `db:"foil_quantity"`
}

// Totals contains authoritative ownership totals independent of list filters.
type Totals struct {
	TotalQuantity uint64 `db:"total_quantity"`
	DistinctCards uint64 `db:"distinct_cards"`
}

// CollectionState is returned after adjusting one precise printing.
type CollectionState struct {
	Card       Summary
	Group      Quantity
	Collection Totals
	SetSummary Totals
}

// LanguageOption identifies a navigable language printing and its owned quantities.
type LanguageOption struct {
	PrintingUUID string
	OracleID     *string
	Language     Language
	Quantity     Quantity
}

// Version groups all language options for one series and collector number.
type Version struct {
	Set             Set
	CollectorNumber string
	ReleasedAt      *time.Time
	Quantity        Quantity
	Languages       []LanguageOption
}

// Ruling contains a dated Oracle clarification with translated text when available.
type Ruling struct {
	PublishedAt time.Time `db:"published_at"`
	Text        string    `db:"text"`
}

// Tag contains a canonical Oracle tag query for display and search navigation.
type Tag struct {
	ID    string
	Query string
}

// DeckUsage records how a logical card is used by one saved deck.
type DeckUsage struct {
	ID       uint64
	Name     string
	Quantity uint64
	Sections []string
}

// Detail combines a complete printing with its Oracle and personal context.
type Detail struct {
	Card              Summary
	Faces             []Face
	Collection        Quantity
	LogicalCollection Quantity
	Included          bool
	Rulings           []Ruling
	Tags              []Tag
	Languages         []LanguageOption
	OtherVersions     []Version
	Decks             []DeckUsage
}
