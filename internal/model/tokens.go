package model

import (
	"time"
)

// TokenSet identifies the Token Set whose printings are being browsed.
type TokenSet struct {
	Code        string     `db:"code"`
	Name        string     `db:"name"`
	DisplayName string     `db:"display_name"`
	ReleasedAt  *time.Time `db:"released_at"`
	SetType     string     `db:"set_type"`
}

// TokenFace contains the source identity and display fields of one Token face.
type TokenFace struct {
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

// TokenList contains the complete lightweight printings of a Token Set.
type TokenList struct {
	Set   TokenSet
	Items []Token
}
