package search

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"mtgnissa/internal/apperror"
	"mtgnissa/internal/model"
	"mtgnissa/internal/store"
)

type searchStore interface {
	Search(context.Context, model.SearchQuery, *store.SearchPosition, int) ([]model.CardSummary, int64, bool, error)
}

// Service parses complete global queries and coordinates stable representative-card pages.
type Service struct {
	store    searchStore
	pageSize int
}

type cursorPayload struct {
	Version     int    `json:"v"`
	Fingerprint string `json:"f"`
	Name        string `json:"name"`
	OracleID    string `json:"oracle_id"`
}

// NewService creates a global search service using the shared Card DB store.
func NewService(cards *store.CardStore, pageSize int) *Service {
	return &Service{store: cards, pageSize: pageSize}
}

// List falls back to literal name matching whenever any syntax cannot be fully recognized.
func (s *Service) List(ctx context.Context, params model.SearchParams) (model.SearchPage, error) {
	query := parseQuery(params.Query)
	fingerprint := queryFingerprint(query)
	var position *store.SearchPosition
	if params.Cursor != "" {
		decoded, err := decodeCursor(params.Cursor, fingerprint)
		if err != nil {
			return model.SearchPage{}, err
		}
		position = &decoded
	}
	items, total, more, err := s.store.Search(ctx, query, position, s.pageSize)
	if err != nil {
		return model.SearchPage{}, apperror.Wrap(apperror.CodeDependencyUnavailable, "card search is temporarily unavailable", err)
	}
	result := model.SearchPage{Input: query.Input, Mode: query.Mode, ExecutedQuery: query.ExecutedQuery, Items: append([]model.CardSummary{}, items...), Total: total}
	if more {
		if len(items) == 0 || items[len(items)-1].OracleID == nil {
			return model.SearchPage{}, apperror.New(apperror.CodeInternalError, "search pagination boundary is unavailable")
		}
		last := items[len(items)-1]
		raw, err := json.Marshal(cursorPayload{Version: 1, Fingerprint: fingerprint, Name: strings.ToLower(last.Name), OracleID: *last.OracleID})
		if err != nil {
			return model.SearchPage{}, apperror.Wrap(apperror.CodeInternalError, "could not create search cursor", err)
		}
		cursor := base64.RawURLEncoding.EncodeToString(raw)
		result.NextCursor = &cursor
	}
	return result, nil
}

func queryFingerprint(query model.SearchQuery) string {
	raw, _ := json.Marshal([]string{"global_search", query.Mode, query.ExecutedQuery})
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func decodeCursor(value, fingerprint string) (store.SearchPosition, error) {
	invalid := apperror.New(apperror.CodeInvalidCursor, "cursor is invalid or does not match this query")
	if len(value) > 4096 {
		return store.SearchPosition{}, invalid
	}
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return store.SearchPosition{}, invalid
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	var payload cursorPayload
	if decoder.Decode(&payload) != nil {
		return store.SearchPosition{}, invalid
	}
	var extra any
	if !errors.Is(decoder.Decode(&extra), io.EOF) || payload.Version != 1 || payload.Fingerprint != fingerprint || !model.ValidUUID(payload.OracleID) || strings.ContainsRune(payload.Name, 0) {
		return store.SearchPosition{}, invalid
	}
	return store.SearchPosition{Name: payload.Name, OracleID: payload.OracleID}, nil
}
