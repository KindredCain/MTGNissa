package store

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
