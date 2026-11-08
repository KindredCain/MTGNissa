package model

import (
	"time"
)

// SetOptions contains the available language and root Set type filters.
type SetOptions struct {
	Languages []SetLanguage
	SetTypes  []string
}

// SetSort identifies the primary ordering applied to root Set groups.
type SetSort string

const (
	SortReleasedDesc  SetSort = "released_desc"
	SortReleasedAsc   SetSort = "released_asc"
	SortNameAsc       SetSort = "name_asc"
	SortNameDesc      SetSort = "name_desc"
	SortCodeAsc       SetSort = "code_asc"
	SortCodeDesc      SetSort = "code_desc"
	SortCardCountDesc SetSort = "card_count_desc"
	SortCardCountAsc  SetSort = "card_count_asc"
)

// SetParams contains the normalized filters and cursor for a Set list query.
type SetParams struct {
	Query     string
	Languages []string
	SetTypes  []string
	Sort      SetSort
	Cursor    string
}

// SetLanguage describes one of the languages shown on every Set list row.
type SetLanguage struct {
	Code        string
	DisplayName string
	SortOrder   int
	Available   bool
}

// SetListItem contains one root or descendant Set and its language availability.
type SetListItem struct {
	Code            string
	Name            string
	DisplayName     string
	SetType         string
	CardCount       uint64
	ReleasedAt      *time.Time
	Languages       []SetLanguage
	DefaultLanguage *string
	ParentCode      *string
	Depth           int
}

// SetPage contains complete root groups in preorder, with a total counting roots.
type SetPage struct {
	Items      []SetListItem
	Total      int64
	NextCursor *string
}
