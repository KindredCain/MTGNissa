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

const cursorVersion = 2

type cursorPayload struct {
	Version     int    `json:"v"`
	Fingerprint string `json:"f"`
	Sort        Sort   `json:"s"`
	Value       string `json:"p"`
	Code        string `json:"code"`
	MissingDate bool   `json:"n,omitempty"`
}

type cursorPosition struct {
	value  string
	code   string
	isNull bool
}

type setStore interface {
	List(context.Context, Params, *cursorPosition, int) (Page, bool, error)
}

// Service validates opaque cursors and coordinates Set list database queries.
type Service struct {
	store    setStore
	pageSize int
}

// NewService creates a Set list service backed by the Card DB store.
func NewService(store *Store, pageSize int) *Service {
	return &Service{store: store, pageSize: pageSize}
}

// List returns one stable page and a cursor bound to the current query.
func (s *Service) List(ctx context.Context, params Params) (Page, error) {
	params.Query = strings.TrimSpace(params.Query)
	if params.Sort == "" {
		params.Sort = SortReleasedDesc
	}
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
	if hasMore {
		var last *Set
		for i := len(page.Items) - 1; i >= 0; i-- {
			if page.Items[i].ParentCode == nil {
				last = &page.Items[i]
				break
			}
		}
		if last == nil {
			return Page{}, apperror.New(apperror.CodeInternalError, "could not locate the pagination root")
		}
		cursor, err := encodeCursor(cursorPayload{
			Version:     cursorVersion,
			Fingerprint: fingerprint,
			Sort:        params.Sort,
			Value:       cursorValue(params.Sort, *last),
			Code:        last.Code,
			MissingDate: isDateSort(params.Sort) && last.ReleasedAt == nil,
		})
		if err != nil {
			return Page{}, apperror.Wrap(apperror.CodeInternalError, "could not create pagination cursor", err)
		}
		page.NextCursor = &cursor
	}
	return page, nil
}

func queryFingerprint(params Params) string {
	languages := cursorFilterValues(params.Languages)
	setTypes := cursorFilterValues(params.SetTypes)
	payload, _ := json.Marshal(struct {
		Query     string   `json:"q"`
		Languages []string `json:"languages"`
		SetTypes  []string `json:"set_types"`
	}{strings.ToLower(strings.TrimSpace(params.Query)), languages, setTypes})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func cursorFilterValues(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		value = strings.ToLower(value)
		if !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result
}

func cursorValue(sort Sort, item Set) string {
	switch sort {
	case SortReleasedDesc, SortReleasedAsc:
		if item.ReleasedAt == nil {
			return ""
		}
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
	if payload.Version != cursorVersion || payload.Sort != sort || payload.Fingerprint != fingerprint || payload.Code == "" || payload.Value == "" && !payload.MissingDate {
		return cursorPosition{}, errors.New("cursor metadata mismatch")
	}
	if payload.MissingDate && (!isDateSort(sort) || payload.Value != "") {
		return cursorPosition{}, errors.New("cursor has an invalid null date")
	}
	if sort == SortCardCountAsc || sort == SortCardCountDesc {
		if _, err := strconv.ParseUint(payload.Value, 10, 64); err != nil {
			return cursorPosition{}, fmt.Errorf("invalid card count: %w", err)
		}
	}
	if isDateSort(sort) && !payload.MissingDate {
		if _, err := time.Parse("2006-01-02", payload.Value); err != nil {
			return cursorPosition{}, fmt.Errorf("invalid release date: %w", err)
		}
	}
	if (sort == SortCodeAsc || sort == SortCodeDesc) && payload.Value != payload.Code {
		return cursorPosition{}, errors.New("cursor code does not match its sort position")
	}
	return cursorPosition{value: payload.Value, code: payload.Code, isNull: payload.MissingDate}, nil
}
