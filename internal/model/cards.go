package model

import (
	"encoding/hex"
	"strings"
	"time"
)

// SetRef identifies the series containing a printing.
type SetRef struct {
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

// CardImages contains resolved picture sources without changing printing identity.
type CardImages struct {
	FrontURL *string
	BackURL  *string
}

// CardSummary describes one language printing using its canonical main-face UUID.
type CardSummary struct {
	PrintingUUID    string
	OracleID        *string
	Name            string
	DisplayName     string
	Set             SetRef
	CollectorNumber string
	Language        Language
	ReleasedAt      *time.Time
	CMC             float64
	ManaCost        *string
	Colors          []string
	Rarity          string
	Layout          string
	Images          CardImages
}

// CardFace contains the original and translated text of a printing face.
type CardFace struct {
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

// CardList is the complete lightweight result for one ordinary series and language.
type CardList struct {
	Set      SetRef
	Language Language
	Items    []CardSummary
}

// Quantity holds the two independent collection finish counters.
type Quantity struct {
	Normal uint64 `db:"normal_quantity"`
	Foil   uint64 `db:"foil_quantity"`
}

// OwnershipTotals contains authoritative ownership totals independent of list filters.
type OwnershipTotals struct {
	TotalQuantity uint64 `db:"total_quantity"`
	DistinctCards uint64 `db:"distinct_cards"`
}

// CollectionState is returned after adjusting one precise printing.
type CollectionState struct {
	Card       CardSummary
	Group      Quantity
	Collection OwnershipTotals
	SetSummary OwnershipTotals
}

// CardLanguageOption identifies a navigable language printing and its owned quantities.
type CardLanguageOption struct {
	PrintingUUID string
	OracleID     *string
	Language     Language
	Quantity     Quantity
}

// CardVersion groups all language options for one series and collector number.
type CardVersion struct {
	Set             SetRef
	CollectorNumber string
	ReleasedAt      *time.Time
	Quantity        Quantity
	Languages       []CardLanguageOption
}

// Ruling contains a dated Oracle clarification with translated text when available.
type Ruling struct {
	PublishedAt time.Time `db:"published_at"`
	Text        string    `db:"text"`
}

// OracleTag contains a canonical Oracle tag query for display and search navigation.
type OracleTag struct {
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

// CardDetail combines a complete printing with its Oracle and personal context.
type CardDetail struct {
	Card              CardSummary
	Faces             []CardFace
	Collection        Quantity
	LogicalCollection Quantity
	Included          bool
	Rulings           []Ruling
	Tags              []OracleTag
	Languages         []CardLanguageOption
	OtherVersions     []CardVersion
	Decks             []DeckUsage
}

// CardChange describes a card mutation to log within its originating transaction.
type CardChange struct {
	Card       CardSummary
	Finish     string
	Delta      int
	Included   *bool
	OccurredAt time.Time
}

// ValidUUID checks canonical UUID text without accepting alternate encodings.
func ValidUUID(value string) bool {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' {
		return false
	}
	_, err := hex.DecodeString(strings.ReplaceAll(value, "-", ""))
	return err == nil
}

// CompareCollectorNumbers compares numeric and textual collector-number segments.
func CompareCollectorNumbers(a, b string) int {
	for len(a) > 0 && len(b) > 0 {
		if a[0] >= '0' && a[0] <= '9' && b[0] >= '0' && b[0] <= '9' {
			i, j := 0, 0
			for i < len(a) && a[i] >= '0' && a[i] <= '9' {
				i++
			}
			for j < len(b) && b[j] >= '0' && b[j] <= '9' {
				j++
			}
			x, y := strings.TrimLeft(a[:i], "0"), strings.TrimLeft(b[:j], "0")
			if len(x) < len(y) {
				return -1
			}
			if len(x) > len(y) {
				return 1
			}
			if order := strings.Compare(x, y); order != 0 {
				return order
			}
			a, b = a[i:], b[j:]
			continue
		}
		if a[0] != b[0] {
			return strings.Compare(a[:1], b[:1])
		}
		a, b = a[1:], b[1:]
	}
	return strings.Compare(a, b)
}
