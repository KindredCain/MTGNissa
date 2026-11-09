package model

// CollectionParams contains normalized collection filters and effective grouping switches.
type CollectionParams struct {
	Query          string
	Sets           []string
	Languages      []string
	Colors         []string
	ManaValues     []string
	Rarities       []string
	Sort           string
	MergeSets      bool
	MergeLanguages bool
	Cursor         string
}

// CollectionOptions contains owned series and languages plus the fixed filter domains.
type CollectionOptions struct {
	Sets       []SetRef
	Languages  []Language
	Colors     []string
	ManaValues []string
	Rarities   []string
}

// CollectionItem represents one filtered ownership group and its authoritative series header.
type CollectionItem struct {
	Card            CardSummary
	GroupSet        *SetRef
	GroupSummary    *OwnershipTotals
	Quantity        Quantity
	MergedSets      bool
	MergedLanguages bool
}

// CollectionPage contains a collection batch and totals independent of the active filters.
type CollectionPage struct {
	Items      []CollectionItem
	Summary    OwnershipTotals
	Total      int64
	NextCursor *string
}
