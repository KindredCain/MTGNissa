package cards

import (
	"errors"
	"math"
	"reflect"
	"testing"

	"mtgnissa/internal/apperror"
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
	items := []Summary{{PrintingUUID: "z", CollectorNumber: "10"}, {PrintingUUID: "b", CollectorNumber: "2"}, {PrintingUUID: "a", CollectorNumber: "2"}, {PrintingUUID: "c", CollectorNumber: "2a"}}
	sortSummaries(items)
	var ids []string
	for _, item := range items {
		ids = append(ids, item.PrintingUUID)
	}
	if !reflect.DeepEqual(ids, []string{"a", "b", "c", "z"}) {
		t.Fatalf("order=%v", ids)
	}
}

func TestAdjustmentDoesNotBorrowOtherFinishAndChecksBounds(t *testing.T) {
	_, err := adjustQuantity(Quantity{Foil: 3}, "normal", -1)
	var appErr *apperror.Error
	if !errors.As(err, &appErr) || appErr.Code != apperror.CodeCollectionUnderflow {
		t.Fatalf("underflow error=%v", err)
	}
	q, err := adjustQuantity(Quantity{Normal: 1, Foil: 3}, "normal", -1)
	if err != nil || q.Normal != 0 || q.Foil != 3 {
		t.Fatalf("quantity=%#v error=%v", q, err)
	}
	q, err = adjustQuantity(Quantity{Normal: 1}, "normal", -1)
	if err != nil || q.Normal+q.Foil != 0 {
		t.Fatalf("zero state=%#v error=%v", q, err)
	}
	if _, err := adjustQuantity(Quantity{Foil: math.MaxUint32}, "foil", 1); err == nil {
		t.Fatal("quantity overflow accepted")
	}
	if _, err := adjustQuantity(Quantity{}, "etched", 1); err == nil {
		t.Fatal("unsupported finish accepted")
	}
}
