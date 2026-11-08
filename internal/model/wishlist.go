package model

// WishlistParams contains the filters and ordering for precise wishlist printings.
type WishlistParams struct {
	Query      string
	Sets       []string
	Languages  []string
	Colors     []string
	ManaValues []string
	Rarities   []string
	Sort       string
	Cursor     string
}

// WishlistOptions contains wishlist series, languages, and fixed filter domains.
type WishlistOptions struct {
	Sets       []SetRef
	Languages  []Language
	Colors     []string
	ManaValues []string
	Rarities   []string
}

// WishlistItem represents one precise printing and its optional series header.
type WishlistItem struct {
	Card     CardSummary
	GroupSet *SetRef
	Included bool
}

// WishlistPage contains a filtered wishlist batch and its continuation cursor.
type WishlistPage struct {
	Items      []WishlistItem
	Total      int64
	NextCursor *string
}
