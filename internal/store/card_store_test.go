package store

import (
	"reflect"
	"testing"
	"time"

	"mtgnissa/internal/model"
)

func TestPrintingIdentityIsIndependentOfImageFallback(t *testing.T) {
	id := "01234567-1234-1234-1234-123456789abc"
	rows := []faceRow{
		{UUID: "back-print", SourceID: id, ImageID: id, Index: 1, Name: "Front // Back", FaceName: "Back", DisplayName: "背", Layout: "transform", FaceCategory: "double_sided", LanguageCode: "ja"},
		{UUID: "front-print", SourceID: id, ImageID: id, Index: 0, Name: "Front // Back", FaceName: "Front", DisplayName: "正", Layout: "transform", FaceCategory: "double_sided", LanguageCode: "ja", Colors: 18},
	}
	card, faces := summarize(rows)
	if card.PrintingUUID != "front-print" || card.Language.Code != "ja" || len(faces) != 2 || faces[0].FaceIndex != 0 || faces[1].FaceIndex != 1 {
		t.Fatalf("identity or face order changed: %#v %#v", card, faces)
	}
	if card.Images.FrontURL == nil || *card.Images.FrontURL != "https://cards.scryfall.io/normal/front/0/1/"+id+".jpg" || card.Images.BackURL == nil || faces[1].ImageURL == nil {
		t.Fatalf("double-faced image resolution failed: %#v", card.Images)
	}
	if !reflect.DeepEqual(card.Colors, []string{"U", "G"}) {
		t.Fatalf("colors=%v", card.Colors)
	}
	rows[0].Layout, rows[0].FaceCategory = "split", "single_sided_multi_face"
	rows[1].Layout, rows[1].FaceCategory = "split", "single_sided_multi_face"
	card, faces = summarize(rows)
	if card.Images.BackURL != nil || faces[1].ImageURL != nil {
		t.Fatal("split card invented an independent back")
	}
}

func TestGroupingKeepsLanguagesAndPrintingsSeparate(t *testing.T) {
	rows := []faceRow{{SourceID: "one", LanguageCode: "en", Index: 0}, {SourceID: "one", LanguageCode: "en", Index: 1}, {SourceID: "one", LanguageCode: "ja"}, {SourceID: "two", LanguageCode: "en"}}
	if len(groupFaces(rows)) != 3 {
		t.Fatal("different languages or printings merged")
	}
	items := []model.CardSummary{{PrintingUUID: "z", CollectorNumber: "10"}, {PrintingUUID: "b", CollectorNumber: "2"}, {PrintingUUID: "a", CollectorNumber: "2"}, {PrintingUUID: "c", CollectorNumber: "2a"}}
	sortSummaries(items)
	var ids []string
	for _, item := range items {
		ids = append(ids, item.PrintingUUID)
	}
	if !reflect.DeepEqual(ids, []string{"a", "b", "c", "z"}) {
		t.Fatalf("order=%v", ids)
	}
}

func TestDeckPrintingPreferenceUsesTierThenDateThenUUID(t *testing.T) {
	oldDate := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	newDate := oldDate.AddDate(20, 0, 0)
	owned := deckPrintingRow{UUID: "owned", ReleasedAt: &oldDate}
	wishlist := deckPrintingRow{UUID: "wish", ReleasedAt: &newDate}
	latest := deckPrintingRow{UUID: "latest", ReleasedAt: &newDate}
	preferred := map[string]int{"owned": 1, "wish": 2}
	if !preferredDeckPrinting(owned, wishlist, preferred) || !preferredDeckPrinting(wishlist, latest, preferred) || preferredDeckPrinting(latest, owned, preferred) {
		t.Fatal("printing preference ignored ownership or wishlist")
	}
	if !preferredDeckPrinting(latest, deckPrintingRow{UUID: "a", ReleasedAt: &oldDate}, nil) || !preferredDeckPrinting(deckPrintingRow{UUID: "a", ReleasedAt: &newDate}, latest, nil) {
		t.Fatal("printing date or UUID tie ordering changed")
	}
}
