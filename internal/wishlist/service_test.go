package wishlist

import (
	"context"
	"encoding/base64"
	"errors"
	"mtgnissa/internal/apperror"
	"mtgnissa/internal/model"
	"mtgnissa/internal/store"
	"reflect"
	"testing"
)

type storeStub struct {
	page      model.WishlistPage
	next      *store.PersonalPosition
	cards     map[string]model.CardSummary
	sets      []model.SetRef
	languages []model.Language
	params    model.WishlistParams
	position  *store.PersonalPosition
	pageSize  int
	calls     int
	resolved  []string
	err       error
}

func (s *storeStub) Options(context.Context) ([]model.SetRef, []model.Language, error) {
	return s.sets, s.languages, s.err
}
func (s *storeStub) List(_ context.Context, params model.WishlistParams, position *store.PersonalPosition, pageSize int) (model.WishlistPage, *store.PersonalPosition, error) {
	s.calls++
	s.params, s.position, s.pageSize = params, position, pageSize
	return s.page, s.next, s.err
}
func (s *storeStub) Summaries(_ context.Context, ids []string) (map[string]model.CardSummary, error) {
	s.resolved = append(s.resolved, ids...)
	return s.cards, s.err
}
func TestPageKeepsSQLBoundaryAndUsesCardDBDisplayMetadata(t *testing.T) {
	id := "00000000-0000-4000-8000-000000000001"
	boundary := &store.PersonalPosition{UUID: id, Name: "saved name", SetCode: "OLD", SetDate: "2020-01-01", Number: "107a", Mana: 2.5, Color: 1}
	card := model.CardSummary{PrintingUUID: id, Name: "Current Card DB Name", DisplayName: "Current Translation", CMC: 7, Set: model.SetRef{Code: "OLD", DisplayName: "Current Set"}, Language: model.Language{Code: "en", DisplayName: "Current Language"}}
	for _, order := range []string{"set", "name", "color", "mana_value"} {
		stub := &storeStub{page: model.WishlistPage{Total: 12, Items: []model.WishlistItem{{Card: model.CardSummary{PrintingUUID: id, Name: "must not display"}}}}, next: boundary, cards: map[string]model.CardSummary{id: card}}
		service := Service{store: stub, pageSize: 2}
		stub.page.Items[0].Included = true
		params := model.WishlistParams{Sort: order, Query: " NAME ", Languages: []string{"JA", "en", "en"}}
		page, err := service.List(context.Background(), params)
		if err != nil || page.Total != 12 || len(page.Items) != 1 || !reflect.DeepEqual(page.Items[0].Card, card) || !reflect.DeepEqual(stub.resolved, []string{id}) || stub.pageSize != 2 || stub.params.Query != "name" || !reflect.DeepEqual(stub.params.Languages, []string{"en", "ja"}) {
			t.Fatalf("sort=%s page=%+v params=%+v error=%v", order, page, stub.params, err)
		}
		if order == "set" && (page.Items[0].GroupSet == nil || *page.Items[0].GroupSet != card.Set) {
			t.Fatalf("header used saved metadata: %+v", page.Items[0])
		}
		if !page.Items[0].Included {
			t.Fatal("Card DB hydration changed wishlist state")
		}
		if page.NextCursor == nil {
			t.Fatal("lost SQL continuation")
		}
		decoded, err := decodeCursor(*page.NextCursor, order, queryFingerprint(normalizeParams(params)))
		if err != nil || !reflect.DeepEqual(store.PersonalPosition(decoded), *boundary) {
			t.Fatalf("cursor used display metadata: %+v %v", decoded, err)
		}
		params.Cursor = *page.NextCursor
		params.Languages = []string{"en", "ja"}
		stub.next = nil
		stub.page = model.WishlistPage{Total: 12}
		empty, err := service.List(context.Background(), params)
		if err != nil || empty.Items == nil || len(empty.Items) != 0 || empty.Total != 12 || !reflect.DeepEqual(stub.position, boundary) || empty.NextCursor != nil || len(stub.resolved) != 1 {
			t.Fatalf("continuation=%+v boundary=%+v error=%v", empty, stub.position, err)
		}
		before := stub.calls
		params.Query = "changed"
		_, err = service.List(context.Background(), params)
		var appErr *apperror.Error
		if !errors.As(err, &appErr) || appErr.Code != apperror.CodeInvalidCursor || stub.calls != before {
			t.Fatalf("changed query reached store: %v", err)
		}
	}
}
func TestOptionsKeepCardDBMetadataAndFixedDomains(t *testing.T) {
	stub := &storeStub{sets: []model.SetRef{{Code: "SET", DisplayName: "Current Set"}}, languages: []model.Language{{Code: "ja", DisplayName: "Current Language", SortOrder: 7}}}
	service := Service{store: stub}
	options, err := service.Options(context.Background())
	if err != nil || !reflect.DeepEqual(options.Sets, stub.sets) || !reflect.DeepEqual(options.Languages, stub.languages) || len(options.Colors) != 7 || len(options.ManaValues) != 9 || len(options.Rarities) != 6 || stub.calls != 0 {
		t.Fatalf("options=%+v error=%v", options, err)
	}
	stub.sets, stub.languages = nil, nil
	options, err = service.Options(context.Background())
	if err != nil || options.Sets == nil || options.Languages == nil {
		t.Fatalf("empty options=%+v error=%v", options, err)
	}
}
func TestMalformedCursorMissingCardAndDependency(t *testing.T) {
	stub := &storeStub{}
	service := Service{store: stub, pageSize: 2}
	for _, cursor := range []string{"!", base64.RawURLEncoding.EncodeToString([]byte(`{"v":1,"unknown":true}`))} {
		if _, err := service.List(context.Background(), model.WishlistParams{Cursor: cursor}); err == nil || stub.calls != 0 {
			t.Fatalf("malformed cursor reached store: %v", err)
		}
	}
	stub.page = model.WishlistPage{Items: []model.WishlistItem{{Card: model.CardSummary{PrintingUUID: "missing"}}}}
	if _, err := service.List(context.Background(), model.WishlistParams{}); err == nil {
		t.Fatal("missing Card DB printing was silently returned")
	}
	stub.err = errors.New("private database error")
	_, err := service.Options(context.Background())
	var appErr *apperror.Error
	if !errors.As(err, &appErr) || appErr.Code != apperror.CodeDependencyUnavailable {
		t.Fatalf("dependency error=%v", err)
	}
}
