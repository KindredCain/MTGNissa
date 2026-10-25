// Package setlist provides the query model and Card DB access for the Set list.
package setlist

import "time"

// Sort identifies the primary ordering applied to root Set groups.
type Sort string

const (
	SortReleasedDesc  Sort = "released_desc"
	SortReleasedAsc   Sort = "released_asc"
	SortNameAsc       Sort = "name_asc"
	SortNameDesc      Sort = "name_desc"
	SortCodeAsc       Sort = "code_asc"
	SortCodeDesc      Sort = "code_desc"
	SortCardCountDesc Sort = "card_count_desc"
	SortCardCountAsc  Sort = "card_count_asc"
)

// Params contains the normalized filters and cursor for a Set list query.
type Params struct {
	Query     string
	Languages []string
	SetTypes  []string
	Sort      Sort
	Cursor    string
}

// Language describes one of the languages shown on every Set list row.
type Language struct {
	Code        string
	DisplayName string
	SortOrder   int
	Available   bool
}

// Set contains one root or descendant Set and its language availability.
type Set struct {
	Code            string
	Name            string
	DisplayName     string
	SetType         string
	CardCount       uint64
	ReleasedAt      *time.Time
	Languages       []Language
	DefaultLanguage *string
	ParentCode      *string
	Depth           int
}

// Page contains complete root groups in preorder, with a total counting roots.
type Page struct {
	Items      []Set
	Total      int64
	NextCursor *string
}
