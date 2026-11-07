// Package collection provides owned-printing filters, grouping, and cursor lists.
package collection

import "mtgnissa/internal/cards"

// Params contains normalized collection filters and effective grouping switches.
type Params struct {
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

// Options contains owned series and languages plus the fixed filter domains.
type Options struct {
	Sets       []cards.Set
	Languages  []cards.Language
	Colors     []string
	ManaValues []string
	Rarities   []string
}

// Item represents one filtered ownership group and its authoritative series header.
type Item struct {
	Card            cards.Summary
	GroupSet        *cards.Set
	GroupSummary    *cards.Totals
	Quantity        cards.Quantity
	MergedSets      bool
	MergedLanguages bool
}

// Page contains a collection batch and totals independent of the active filters.
type Page struct {
	Items      []Item
	Summary    cards.Totals
	Total      int64
	NextCursor *string
}

type record struct {
	Card     cards.Summary
	Quantity cards.Quantity
}

type groupKey struct {
	Kind     string
	Identity string
	Set      string
	Number   string
	Language string
}
