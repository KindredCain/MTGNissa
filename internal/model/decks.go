package model

import "time"

// SavedDeck contains the persisted deck definition and its logical card entries.
type SavedDeck struct {
	ID            uint64
	Name          string
	SourceText    string
	CoverOracleID *string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	Cards         []SavedDeckCard
}

// SavedDeckCard describes one logical card in one real deck section.
type SavedDeckCard struct {
	Section  string
	OracleID string
	Quantity uint64
	Name     string
}

// DeckSave contains the complete editor state submitted using printing identities.
type DeckSave struct {
	Name              string
	CoverPrintingUUID *string
	Sections          []DeckSaveSection
}

// DeckSaveSection contains the printing quantities submitted for one section.
type DeckSaveSection struct {
	Type  string
	Cards []DeckSaveCard
}

// DeckSaveCard identifies one submitted printing and its positive quantity.
type DeckSaveCard struct {
	PrintingUUID string
	Quantity     uint64
}

// DeckPersonal contains snapshot ownership, saved usage, and printing preferences.
type DeckPersonal struct {
	Owned       map[string]uint64
	AllQuantity map[string]uint64
	UsingDecks  map[string]uint64
	Preferred   map[string]int
}

// DeckCardData combines a selected printing with the raw attributes used by decks.
type DeckCardData struct {
	Card         CardSummary
	Faces        []CardFace
	ProducedMana []string
	OracleTags   []string
}

// DeckCardStatistics contains authoritative, unweighted properties for editor statistics.
type DeckCardStatistics struct {
	CMC            float64
	ManaCost       *string
	Colors         []string
	ProducedMana   []string
	TypeGroups     []string
	OracleTags     []string
	IsLand         bool
	CanBeCommander bool
}

// DeckCardException describes the priority shortage or shared usage of a logical card.
type DeckCardException struct {
	Type            string
	MissingQuantity uint64
}

// DeckExceptionTotals counts distinct affected cards and their aggregate missing quantities.
type DeckExceptionTotals struct {
	DistinctCards   uint64
	MissingQuantity uint64
}

// DeckEditCardContext supplies one resolved printing and stable personal editing baselines.
type DeckEditCardContext struct {
	RequestedPrintingUUID string
	Card                  CardSummary
	TypeGroup             string
	TranslatedTypeLine    string
	TranslatedOracleText  *string
	OwnedQuantity         uint64
	OtherDecksQuantity    uint64
	OtherDecksCount       uint64
	Statistics            DeckCardStatistics
}

// DeckCard is one saved section entry with its display and personal card context.
type DeckCard struct {
	Context          DeckEditCardContext
	Quantity         uint64
	AllDecksQuantity uint64
	UsingDecks       uint64
	Exception        *DeckCardException
}

// DeckSection contains one of the four fixed saved deck sections.
type DeckSection struct {
	Type     string
	Quantity uint64
	Cards    []DeckCard
}

// DeckSummary reports counted ownership and independent exception totals.
type DeckSummary struct {
	CountedTotal   uint64
	CollectionRate float64
	Shortage       DeckExceptionTotals
	DeckUsage      DeckExceptionTotals
}

// DeckDetail contains the saved deck, its selected cover, and all four sections.
type DeckDetail struct {
	ID                uint64
	Name              string
	SourceText        string
	CoverPrintingUUID *string
	CreatedAt         time.Time
	UpdatedAt         time.Time
	Sections          []DeckSection
	Summary           DeckSummary
}

// DeckListParams contains normalized deck name filters, ordering, and cursor.
type DeckListParams struct {
	Query  string
	Sort   string
	Cursor string
}

// DeckListItem contains a saved deck's cover and weighted list statistics.
type DeckListItem struct {
	ID                uint64
	Name              string
	CreatedAt         time.Time
	UpdatedAt         time.Time
	CoverCard         *CardSummary
	SectionCounts     map[string]uint64
	CountedTotal      uint64
	ColorDistribution map[string]uint64
	Exception         *DeckListException
}

// DeckListException contains the highest-priority exception class in a saved deck.
type DeckListException struct {
	Type string
	DeckExceptionTotals
}

// DeckPage is a cursor batch of saved deck list items.
type DeckPage struct {
	Items      []DeckListItem
	Total      int64
	NextCursor *string
}

// DeckIdentity identifies the authoritative resource returned by create or copy.
type DeckIdentity struct {
	ID   uint64
	Name string
}

// DeckSearchParams contains the printed name query and its continuation cursor.
type DeckSearchParams struct {
	Query  string
	Cursor string
}

// DeckCandidate is a lightweight display printing for one logical card search hit.
type DeckCandidate struct {
	Card              CardSummary
	DisplayTypeLine   string
	DisplayOracleText *string
}

// DeckCandidatePage contains Oracle-deduplicated search results.
type DeckCandidatePage struct {
	Items      []DeckCandidate
	Total      int64
	NextCursor *string
}

// DeckName identifies a canonical English card name and its optional Oracle identity.
type DeckName struct {
	Name     string
	OracleID *string
}

// ParsedDeckCard contains the selected display printing and parsed section quantity.
type ParsedDeckCard struct {
	Card     CardSummary
	Quantity uint64
}

// ParsedDeckSection contains parsed entries for one fixed section.
type ParsedDeckSection struct {
	Type  string
	Cards []ParsedDeckCard
}

// ParsedDeck contains an optional name override and complete parsed sections.
type ParsedDeck struct {
	Name     *string
	Sections []ParsedDeckSection
}
