package store

import (
	"context"

	"github.com/jmoiron/sqlx"

	"mtgnissa/internal/appdata"
	"mtgnissa/internal/model"
)

// OperationLogTx writes historical snapshots in the same transaction as their mutations.
type OperationLogTx struct {
	tx *sqlx.Tx
}

// AppendCard writes a card operation and its display snapshot without committing.
func (s *OperationLogTx) AppendCard(ctx context.Context, change model.CardChange) error {
	action := appdata.ActionAddCard
	var finish *appdata.FinishKind
	var delta *int
	if change.Included != nil {
		action = appdata.ActionRemoveWishlist
		if *change.Included {
			action = appdata.ActionAddWishlist
		}
	} else {
		if change.Delta < 0 {
			action = appdata.ActionRemoveCard
		}
		kind := appdata.FinishNormal
		if change.Finish == "foil" {
			kind = appdata.FinishFoil
		}
		finish, delta = &kind, &change.Delta
	}
	result, err := s.tx.ExecContext(ctx, "INSERT INTO operation_log (action_type,occurred_at) VALUES (?,?)", action, change.OccurredAt)
	if err != nil {
		return err
	}
	operationID, err := result.LastInsertId()
	if err != nil {
		return err
	}
	card := change.Card
	_, err = s.tx.ExecContext(ctx, `INSERT INTO operation_card_detail
 (operation_id,printing_uuid,oracle_id,card_name,set_code,collector_number,lang,lang_display_name,front_url,back_url,has_distinct_back,finish_kind,quantity_delta)
 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`, operationID, card.PrintingUUID, card.OracleID, card.DisplayName, card.Set.Code,
		card.CollectorNumber, card.Language.Code, card.Language.DisplayName, card.Images.FrontURL, card.Images.BackURL,
		card.Images.BackURL != nil, finish, delta)
	return err
}
