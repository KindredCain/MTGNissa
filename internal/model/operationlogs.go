package model

import "time"

// OperationSummaryParams selects local calendar dates and a fixed-size summary page.
type OperationSummaryParams struct {
	StartDate string
	EndDate   string
	Page      int
}

// OperationLogParams selects one local date, action type and detail page.
type OperationLogParams struct {
	Date   string
	Action string
	Page   int
}

// OperationRange contains optional inclusive and exclusive UTC timestamp bounds.
type OperationRange struct {
	Start *time.Time
	End   *time.Time
}

// OperationSummary groups actual events by application date and action type.
type OperationSummary struct {
	Date   string
	Action string
	Count  int64
}

// OperationSummaryPage contains a fixed-size page of daily action groups.
type OperationSummaryPage struct {
	Items []OperationSummary
	Page  int
	Total int64
}

// OperationCardSnapshot preserves the card display state at mutation time.
type OperationCardSnapshot struct {
	PrintingUUID        string
	Name                string
	SetCode             string
	CollectorNumber     string
	Language            string
	LanguageDisplayName string
	Finish              *string
	QuantityDelta       *int64
	Images              CardImages
	HasDistinctBack     bool
}

// OperationDeckSnapshot preserves a deck identity and name even after deletion.
type OperationDeckSnapshot struct {
	ID   uint64
	Name string
}

// OperationItem combines event metadata with exactly one historical snapshot.
type OperationItem struct {
	ID         uint64
	Action     string
	OccurredAt time.Time
	Card       *OperationCardSnapshot
	Deck       *OperationDeckSnapshot
}

// OperationLogPage contains a fixed-size page of historical events.
type OperationLogPage struct {
	Items []OperationItem
	Page  int
	Total int64
}

// OperationActionCode returns the persisted enum order, or zero for an unknown action.
func OperationActionCode(name string) uint8 {
	for code := uint8(1); code <= 8; code++ {
		if OperationActionName(code) == name {
			return code
		}
	}
	return 0
}

// OperationActionName resolves the eight persisted action codes to public names.
func OperationActionName(code uint8) string {
	names := [...]string{"", "add_card", "remove_card", "add_wishlist", "remove_wishlist", "create_deck", "delete_deck", "copy_deck", "modify_deck"}
	if int(code) >= len(names) {
		return ""
	}
	return names[code]
}
