package store

import (
	"context"
	"time"

	"github.com/jmoiron/sqlx"

	"mtgnissa/internal/appdata"
	"mtgnissa/internal/apperror"
	"mtgnissa/internal/model"
)

// lockPrintingRef maintains the shared App DB identity within the caller's transaction.
func lockPrintingRef(ctx context.Context, tx *sqlx.Tx, card model.CardSummary, now time.Time) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO card_print_ref (printing_uuid,oracle_id,set_code,collector_number,lang,created_at,last_verified_at)
 VALUES (?,?,?,?,?,?,?) ON DUPLICATE KEY UPDATE last_verified_at=VALUES(last_verified_at)`, card.PrintingUUID, card.OracleID,
		card.Set.Code, card.CollectorNumber, card.Language.Code, now, now)
	if err != nil {
		return err
	}
	var ref appdata.CardPrintRef
	if err := tx.GetContext(ctx, &ref, "SELECT * FROM card_print_ref WHERE printing_uuid=? FOR UPDATE", card.PrintingUUID); err != nil {
		return err
	}
	sameOracle := ref.OracleID == nil && card.OracleID == nil || ref.OracleID != nil && card.OracleID != nil && *ref.OracleID == *card.OracleID
	if !sameOracle || ref.SetCode != card.Set.Code || ref.CollectorNumber != card.CollectorNumber || ref.Lang != card.Language.Code {
		return apperror.New(apperror.CodeInternalError, "stored printing identity does not match card data")
	}
	return nil
}
