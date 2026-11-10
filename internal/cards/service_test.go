package cards

import (
	"errors"
	"math"
	"testing"

	"mtgnissa/internal/apperror"
	"mtgnissa/internal/model"
)

func TestAdjustmentDoesNotBorrowOtherFinishAndChecksBounds(t *testing.T) {
	_, err := adjustQuantity(model.Quantity{Foil: 3}, "normal", -1)
	var appErr *apperror.Error
	if !errors.As(err, &appErr) || appErr.Code != apperror.CodeCollectionUnderflow {
		t.Fatalf("underflow error=%v", err)
	}
	q, err := adjustQuantity(model.Quantity{Normal: 1, Foil: 3}, "normal", -1)
	if err != nil || q.Normal != 0 || q.Foil != 3 {
		t.Fatalf("quantity=%#v error=%v", q, err)
	}
	q, err = adjustQuantity(model.Quantity{Normal: 1}, "normal", -1)
	if err != nil || q.Normal+q.Foil != 0 {
		t.Fatalf("zero state=%#v error=%v", q, err)
	}
	if _, err := adjustQuantity(model.Quantity{Foil: math.MaxUint32}, "foil", 1); err == nil {
		t.Fatal("quantity overflow accepted")
	}
	if _, err := adjustQuantity(model.Quantity{}, "etched", 1); err == nil {
		t.Fatal("unsupported finish accepted")
	}
}

func TestDetailGroupsVersionsAndOrdersLanguages(t *testing.T) {
	current := model.CardVersionCandidate{PrintingUUID: "current-en", Set: model.SetRef{Code: "KLD"}, CollectorNumber: "163", Language: model.Language{Code: "en", SortOrder: 1}}
	translated := current
	translated.PrintingUUID, translated.Language = "current-zhs", model.Language{Code: "zhs", SortOrder: 10}
	other := current
	other.PrintingUUID, other.Set.Code = "other-en", "KLR"
	detail := buildDetail(model.CardDetailData{
		Card:     model.CardSummary{Set: current.Set, CollectorNumber: current.CollectorNumber},
		Versions: []model.CardVersionCandidate{translated, other, current},
		Tags:     []model.OracleTagLabel{{ID: "tag", Label: `draw "card"\engine`}},
	})
	if len(detail.Languages) != 2 || detail.Languages[0].PrintingUUID != current.PrintingUUID || detail.Languages[1].PrintingUUID != translated.PrintingUUID {
		t.Fatalf("current languages=%#v", detail.Languages)
	}
	if len(detail.OtherVersions) != 1 || detail.OtherVersions[0].Set.Code != "KLR" || detail.OtherVersions[0].Languages[0].PrintingUUID != other.PrintingUUID {
		t.Fatalf("other versions=%#v", detail.OtherVersions)
	}
	if len(detail.Tags) != 1 || detail.Tags[0].Query != `otag:"draw \"card\"\\engine"` {
		t.Fatalf("tags=%#v", detail.Tags)
	}
}
