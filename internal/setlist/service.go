package setlist

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"mtgnissa/internal/apperror"
)

const cursorVersion = 1

type cursorPayload struct {
	Version     int    `json:"v"`
	Fingerprint string `json:"f"`
	Sort        Sort   `json:"s"`
	Value       string `json:"p"`
	SetID       string `json:"id"`
}

type cursorPosition struct {
	value string
	setID string
}

// Service validates opaque cursors and coordinates Set list database queries.
type Service struct {
	store    *Store
	pageSize int
}

// NewService creates a Set list service backed by the Card DB store.
func NewService(store *Store, pageSize int) *Service {
	return &Service{store: store, pageSize: pageSize}
}

// List returns one stable page and a cursor bound to the current query.
func (s *Service) List(ctx context.Context, params Params) (Page, error) {
	fingerprint := queryFingerprint(params)
	var position *cursorPosition
	if params.Cursor != "" {
		decoded, err := decodeCursor(params.Cursor, params.Sort, fingerprint)
		if err != nil {
			return Page{}, apperror.Wrap(apperror.CodeInvalidCursor, "cursor is invalid or does not match this query", err)
		}
		position = &decoded
	}

	page, hasMore, err := s.store.List(ctx, params, position, s.pageSize)
	if err != nil {
		return Page{}, apperror.Wrap(apperror.CodeDependencyUnavailable, "card data is temporarily unavailable", err)
	}
	if hasMore && len(page.Items) != 0 {
		last := page.Items[len(page.Items)-1]
		cursor, err := encodeCursor(cursorPayload{
			Version:     cursorVersion,
			Fingerprint: fingerprint,
			Sort:        params.Sort,
			Value:       cursorValue(params.Sort, last),
			SetID:       last.SetID,
		})
		if err != nil {
			return Page{}, apperror.Wrap(apperror.CodeInternalError, "could not create pagination cursor", err)
		}
		page.NextCursor = &cursor
	}
	return page, nil
}

func queryFingerprint(params Params) string {
	languages := append([]string(nil), params.Languages...)
	setTypes := append([]string(nil), params.SetTypes...)
	stringsToLower(languages)
	stringsToLower(setTypes)
	sort.Strings(languages)
	sort.Strings(setTypes)
	payload, _ := json.Marshal(struct {
		Query     string   `json:"q"`
		Languages []string `json:"languages"`
		SetTypes  []string `json:"set_types"`
	}{strings.ToLower(params.Query), languages, setTypes})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func stringsToLower(values []string) {
	for i := range values {
		values[i] = strings.ToLower(values[i])
	}
}

func cursorValue(sort Sort, item Set) string {
	switch sort {
	case SortReleasedDesc, SortReleasedAsc:
		return item.ReleasedAt.Format("2006-01-02")
	case SortNameAsc, SortNameDesc:
		return item.Name
	case SortCodeAsc, SortCodeDesc:
		return item.Code
	case SortCardCountDesc, SortCardCountAsc:
		return strconv.FormatUint(item.CardCount, 10)
	default:
		return ""
	}
}

func encodeCursor(payload cursorPayload) (string, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(encoded), nil
}

func decodeCursor(value string, sort Sort, fingerprint string) (cursorPosition, error) {
	if len(value) > 4096 {
		return cursorPosition{}, errors.New("cursor is too long")
	}
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return cursorPosition{}, err
	}
	var payload cursorPayload
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		return cursorPosition{}, err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return cursorPosition{}, errors.New("cursor contains trailing data")
	}
	if payload.Version != cursorVersion || payload.Sort != sort || payload.Fingerprint != fingerprint || payload.SetID == "" || payload.Value == "" {
		return cursorPosition{}, errors.New("cursor metadata mismatch")
	}
	if sort == SortCardCountAsc || sort == SortCardCountDesc {
		if _, err := strconv.ParseUint(payload.Value, 10, 64); err != nil {
			return cursorPosition{}, fmt.Errorf("invalid card count: %w", err)
		}
	}
	if sort == SortReleasedAsc || sort == SortReleasedDesc {
		if _, err := time.Parse("2006-01-02", payload.Value); err != nil {
			return cursorPosition{}, fmt.Errorf("invalid release date: %w", err)
		}
	}
	return cursorPosition{value: payload.Value, setID: payload.SetID}, nil
}
