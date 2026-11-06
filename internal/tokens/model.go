// Package tokens provides read-only Token Set browsing.
package tokens

import "time"

// Set identifies the Token Set whose printings are being browsed.
type Set struct {
	Code        string     `db:"code"`
	Name        string     `db:"name"`
	DisplayName string     `db:"display_name"`
	ReleasedAt  *time.Time `db:"released_at"`
	SetType     string     `db:"set_type"`
}

// Face contains the source identity and display fields of one Token face.
type Face struct {
	UUID            string  `db:"uuid"`
	ScryfallID      *string `db:"scryfall_id"`
	FaceIndex       int     `db:"face_index"`
	Name            string  `db:"name"`
	CollectorNumber string  `db:"collector_number"`
	Layout          string  `db:"layout"`
}

// Token represents one printing after its related faces have been grouped.
type Token struct {
	UUID            string
	Name            string
	CollectorNumber string
	Layout          string
	FrontURL        *string
	BackURL         *string
}

// List contains the complete lightweight printings of a Token Set.
type List struct {
	Set   Set
	Items []Token
}
