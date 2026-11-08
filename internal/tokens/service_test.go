package tokens

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"mtgnissa/internal/apperror"
	"mtgnissa/internal/model"
)

type storeStub struct {
	err error
}

func (s storeStub) Read(context.Context, string) (model.TokenSet, []model.TokenFace, error) {
	return model.TokenSet{Code: "TKLD"}, nil, s.err
}

func TestOrganizeGroupsFacesAndPreservesMissingImages(t *testing.T) {
	id := "01234567-1234-1234-1234-123456789abc"
	blank := ""
	faces := []model.TokenFace{
		{UUID: "back", ScryfallID: &id, FaceIndex: 1, CollectorNumber: "20", Layout: "double_faced_token"},
		{UUID: "front", ScryfallID: &id, FaceIndex: 0, Name: "Beast", CollectorNumber: "10", Layout: "double_faced_token"},
		{UUID: "missing-b", ScryfallID: &blank, FaceIndex: -1, CollectorNumber: "2", Layout: "token"},
		{UUID: "missing-a", FaceIndex: -1, CollectorNumber: "2", Layout: "token"},
		{UUID: "suffix", FaceIndex: -1, CollectorNumber: "2a", Layout: "token"},
	}
	got := organize(faces)
	var ids []string
	for _, item := range got {
		ids = append(ids, item.UUID)
	}
	if !reflect.DeepEqual(ids, []string{"missing-a", "missing-b", "suffix", "front"}) {
		t.Fatalf("printing order = %v", ids)
	}
	front := got[3]
	if front.Name != "Beast" || front.FrontURL == nil || *front.FrontURL != "https://cards.scryfall.io/normal/front/0/1/"+id+".jpg" || front.BackURL == nil || *front.BackURL != "https://cards.scryfall.io/normal/back/0/1/"+id+".jpg" {
		t.Fatalf("front summary = %#v", front)
	}
	for _, item := range got[:3] {
		if item.FrontURL != nil || item.BackURL != nil {
			t.Fatalf("missing ID produced images: %#v", item)
		}
	}
	// Query row ordering must not affect printing identity or output order.
	for i, j := 0, len(faces)-1; i < j; i, j = i+1, j-1 {
		faces[i], faces[j] = faces[j], faces[i]
	}
	if !reflect.DeepEqual(got, organize(faces)) {
		t.Fatal("output depends on database row order")
	}
}

func TestListReturnsEmptyItemsAndMapsErrors(t *testing.T) {
	service := &Service{store: storeStub{}}
	list, err := service.List(context.Background(), "TKLD")
	if err != nil || list.Items == nil || len(list.Items) != 0 || list.Set.Code != "TKLD" {
		t.Fatalf("empty list = %#v, error = %v", list, err)
	}
	for _, code := range []apperror.Code{apperror.CodeSetNotFound, apperror.CodeInvalidRequest} {
		original := apperror.New(code, "safe error")
		service.store = storeStub{err: original}
		if _, err := service.List(context.Background(), "TKLD"); err != original {
			t.Fatalf("business error was changed: %v", err)
		}
	}
	service.store = storeStub{err: errors.New("private database failure")}
	_, err = service.List(context.Background(), "TKLD")
	var appErr *apperror.Error
	if !errors.As(err, &appErr) || appErr.Code != apperror.CodeDependencyUnavailable || appErr.Message != "card data is temporarily unavailable" {
		t.Fatalf("database error was not mapped safely: %v", err)
	}
}
