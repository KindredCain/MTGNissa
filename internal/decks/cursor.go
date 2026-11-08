package decks

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"strings"
	"time"

	"mtgnissa/internal/apperror"
	"mtgnissa/internal/model"
)

type cursorPosition struct {
	ID       uint64    `json:"id"`
	Name     string    `json:"name"`
	Time     time.Time `json:"time"`
	Count    uint64    `json:"count"`
	OracleID string    `json:"oracle_id"`
}

type cursorPayload struct {
	Version     int            `json:"v"`
	Kind        string         `json:"kind"`
	Fingerprint string         `json:"f"`
	Position    cursorPosition `json:"p"`
}

func cursorFingerprint(kind string, id uint64, query, order string) string {
	raw, _ := json.Marshal([]any{kind, id, query, order})
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func encodeCursor(kind, fingerprint string, position cursorPosition) (string, error) {
	raw, err := json.Marshal(cursorPayload{Version: 1, Kind: kind, Fingerprint: fingerprint, Position: position})
	return base64.RawURLEncoding.EncodeToString(raw), err
}

func decodeCursor(value, fingerprint, kind string) (cursorPosition, error) {
	invalid := apperror.New(apperror.CodeInvalidCursor, "cursor is invalid or does not match this query")
	if len(value) > 4096 {
		return cursorPosition{}, invalid
	}
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return cursorPosition{}, invalid
	}
	var payload cursorPayload
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		return cursorPosition{}, invalid
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF || payload.Version != 1 || payload.Kind != kind || payload.Fingerprint != fingerprint {
		return cursorPosition{}, invalid
	}
	if kind == "decks" && payload.Position.ID == 0 || kind == "deck_cards" && !model.ValidUUID(payload.Position.OracleID) {
		return cursorPosition{}, invalid
	}
	return payload.Position, nil
}

func deckPosition(deck model.SavedDeck, order string) cursorPosition {
	position := cursorPosition{ID: deck.ID, Name: strings.ToLower(deck.Name), Time: deck.UpdatedAt}
	if order == "created_desc" {
		position.Time = deck.CreatedAt
	}
	for _, quantity := range countedCards(deck) {
		position.Count += quantity
	}
	return position
}

func compareDeckPositions(a, b cursorPosition, order string) int {
	switch order {
	case "name_asc":
		if c := strings.Compare(a.Name, b.Name); c != 0 {
			return c
		}
	case "card_count_desc":
		if a.Count > b.Count {
			return -1
		}
		if a.Count < b.Count {
			return 1
		}
	default:
		if a.Time.After(b.Time) {
			return -1
		}
		if a.Time.Before(b.Time) {
			return 1
		}
	}
	if a.ID < b.ID {
		return -1
	}
	if a.ID > b.ID {
		return 1
	}
	return 0
}
