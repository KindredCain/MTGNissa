package tokens

import (
	"context"
	"encoding/hex"
	"errors"
	"sort"
	"strings"

	"mtgnissa/internal/apperror"
	"mtgnissa/internal/model"
	"mtgnissa/internal/store"
)

type tokenStore interface {
	Read(context.Context, string) (model.TokenSet, []model.TokenFace, error)
}

// Service organizes Token faces into naturally ordered printing summaries.
type Service struct {
	store tokenStore
}

// NewService creates a Token list service backed by the Card DB store.
func NewService(store *store.TokenStore) *Service {
	return &Service{store: store}
}

// List returns all printings in the requested Token Set without pagination.
func (s *Service) List(ctx context.Context, code string) (model.TokenList, error) {
	set, faces, err := s.store.Read(ctx, code)
	if err != nil {
		var appErr *apperror.Error
		if errors.As(err, &appErr) {
			return model.TokenList{}, err
		}
		return model.TokenList{}, apperror.Wrap(apperror.CodeDependencyUnavailable, "card data is temporarily unavailable", err)
	}
	return model.TokenList{Set: set, Items: organize(faces)}, nil
}

func organize(faces []model.TokenFace) []model.Token {
	groups := make(map[string][]model.TokenFace)
	for _, face := range faces {
		key := "uuid:" + face.UUID
		if face.ScryfallID != nil && strings.TrimSpace(*face.ScryfallID) != "" {
			key = "scryfall:" + strings.ToLower(strings.TrimSpace(*face.ScryfallID))
		}
		groups[key] = append(groups[key], face)
	}
	items := make([]model.Token, 0, len(groups))
	for _, group := range groups {
		sort.Slice(group, func(i, j int) bool {
			if group[i].FaceIndex != group[j].FaceIndex {
				return group[i].FaceIndex < group[j].FaceIndex
			}
			return group[i].UUID < group[j].UUID
		})
		front := group[0]
		item := model.Token{UUID: front.UUID, Name: front.Name, CollectorNumber: front.CollectorNumber, Layout: front.Layout}
		item.FrontURL = imageURL(front.ScryfallID, "front")
		// The public image source exposes a separate back for double-faced Tokens.
		if front.Layout == "double_faced_token" || len(group) > 1 && group[len(group)-1].FaceIndex > front.FaceIndex {
			item.BackURL = imageURL(front.ScryfallID, "back")
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		if order := model.CompareCollectorNumbers(items[i].CollectorNumber, items[j].CollectorNumber); order != 0 {
			return order < 0
		}
		return items[i].UUID < items[j].UUID
	})
	return items
}

func imageURL(id *string, side string) *string {
	if id == nil {
		return nil
	}
	value := strings.ToLower(strings.TrimSpace(*id))
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' {
		return nil
	}
	if _, err := hex.DecodeString(strings.ReplaceAll(value, "-", "")); err != nil {
		return nil
	}
	url := "https://cards.scryfall.io/normal/" + side + "/" + value[:1] + "/" + value[1:2] + "/" + value + ".jpg"
	return &url
}
