package sets

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
	"mtgnissa/internal/model"
	"mtgnissa/internal/store"
)

const cursorVersion = 2

type cursorPayload struct {
	Version     int           `json:"v"`
	Fingerprint string        `json:"f"`
	Sort        model.SetSort `json:"s"`
	Value       string        `json:"p"`
	Code        string        `json:"code"`
	MissingDate bool          `json:"n,omitempty"`
}

type setStore interface {
	List(context.Context, model.SetParams, *store.SetPosition, int) (model.SetPage, bool, error)
	Options(context.Context) (model.SetOptions, error)
}

// Service validates opaque cursors and coordinates Set list database queries.
type Service struct {
	store    setStore
	pageSize int
}

// NewService creates a Set list service backed by the Card DB store.
func NewService(store *store.SetStore, pageSize int) *Service {
	return &Service{store: store, pageSize: pageSize}
}

// Options returns the dynamic filters available for the Set list.
func (s *Service) Options(ctx context.Context) (model.SetOptions, error) {
	options, err := s.store.Options(ctx)
	if err != nil {
		return model.SetOptions{}, apperror.Wrap(apperror.CodeDependencyUnavailable, "card data is temporarily unavailable", err)
	}
	return options, nil
}

// List returns one stable page and a cursor bound to the current query.
func (s *Service) List(ctx context.Context, params model.SetParams) (model.SetPage, error) {
	params.Query = strings.TrimSpace(params.Query)
	if params.Sort == "" {
		params.Sort = model.SortReleasedDesc
	}
	fingerprint := queryFingerprint(params)
	var position *store.SetPosition
	if params.Cursor != "" {
		decoded, err := decodeCursor(params.Cursor, params.Sort, fingerprint)
		if err != nil {
			return model.SetPage{}, apperror.Wrap(apperror.CodeInvalidCursor, "cursor is invalid or does not match this query", err)
		}
		position = &decoded
	}

	page, hasMore, err := s.store.List(ctx, params, position, s.pageSize)
	if err != nil {
		return model.SetPage{}, apperror.Wrap(apperror.CodeDependencyUnavailable, "card data is temporarily unavailable", err)
	}
	if hasMore {
		var last *model.SetListItem
		for i := len(page.Items) - 1; i >= 0; i-- {
			if page.Items[i].ParentCode == nil {
				last = &page.Items[i]
				break
			}
		}
		if last == nil {
			return model.SetPage{}, apperror.New(apperror.CodeInternalError, "could not locate the pagination root")
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
			return model.SetPage{}, apperror.Wrap(apperror.CodeInternalError, "could not create pagination cursor", err)
		}
		page.NextCursor = &cursor
	}
	return page, nil
}

func queryFingerprint(params model.SetParams) string {
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

func cursorValue(sort model.SetSort, item model.SetListItem) string {
	switch sort {
	case model.SortReleasedDesc, model.SortReleasedAsc:
		if item.ReleasedAt == nil {
			return ""
		}
		return item.ReleasedAt.Format("2006-01-02")
	case model.SortNameAsc, model.SortNameDesc:
		return item.Name
	case model.SortCodeAsc, model.SortCodeDesc:
		return item.Code
	case model.SortCardCountDesc, model.SortCardCountAsc:
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

func decodeCursor(value string, sort model.SetSort, fingerprint string) (store.SetPosition, error) {
	if len(value) > 4096 {
		return store.SetPosition{}, errors.New("cursor is too long")
	}
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return store.SetPosition{}, err
	}
	var payload cursorPayload
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		return store.SetPosition{}, err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return store.SetPosition{}, errors.New("cursor contains trailing data")
	}
	if payload.Version != cursorVersion || payload.Sort != sort || payload.Fingerprint != fingerprint || payload.Code == "" || payload.Value == "" && !payload.MissingDate {
		return store.SetPosition{}, errors.New("cursor metadata mismatch")
	}
	if payload.MissingDate && (!isDateSort(sort) || payload.Value != "") {
		return store.SetPosition{}, errors.New("cursor has an invalid null date")
	}
	if sort == model.SortCardCountAsc || sort == model.SortCardCountDesc {
		if _, err := strconv.ParseUint(payload.Value, 10, 64); err != nil {
			return store.SetPosition{}, fmt.Errorf("invalid card count: %w", err)
		}
	}
	if isDateSort(sort) && !payload.MissingDate {
		if _, err := time.Parse("2006-01-02", payload.Value); err != nil {
			return store.SetPosition{}, fmt.Errorf("invalid release date: %w", err)
		}
	}
	if (sort == model.SortCodeAsc || sort == model.SortCodeDesc) && payload.Value != payload.Code {
		return store.SetPosition{}, errors.New("cursor code does not match its sort position")
	}
	return store.SetPosition{Value: payload.Value, Code: payload.Code, IsNull: payload.MissingDate}, nil
}

func isDateSort(order model.SetSort) bool {
	return order == model.SortReleasedAsc || order == model.SortReleasedDesc
}
