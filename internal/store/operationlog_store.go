package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"

	"mtgnissa/internal/appdata"
	"mtgnissa/internal/model"
)

// OperationLogTx writes historical snapshots in the same transaction as their mutations.
type OperationLogTx struct {
	tx *sqlx.Tx
}

// OperationLogStore reads event metadata and historical snapshots exclusively from App DB.
type OperationLogStore struct {
	db *sqlx.DB
}

type operationCardRow struct {
	appdata.OperationLog
	appdata.OperationCardDetail
}

type operationDeckRow struct {
	appdata.OperationLog
	appdata.OperationDeckDetail
}

type operationSummaryRow struct {
	Date   string `db:"operation_date"`
	Action uint8  `db:"action_type"`
	Count  int64  `db:"operation_count"`
}

// NewOperationLogStore creates an application log reader without Card DB dependencies.
func NewOperationLogStore(db *sqlx.DB) *OperationLogStore {
	return &OperationLogStore{db: db}
}

// Summary aggregates and pages daily action groups using one fixed UTC offset.
func (s *OperationLogStore) Summary(ctx context.Context, bounds model.OperationRange, offsetSeconds, page int) ([]model.OperationSummary, int64, error) {
	tx, err := s.db.BeginTxx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, 0, err
	}
	defer tx.Rollback()
	grouped, args := operationSummarySQL(bounds, offsetSeconds)
	var total int64
	if err := tx.GetContext(ctx, &total, "SELECT COUNT(*) FROM ("+grouped+") AS grouped", args...); err != nil {
		return nil, 0, err
	}
	items := []model.OperationSummary{}
	if int64(page-1)*10 < total {
		statement := grouped + " ORDER BY operation_date DESC,l.action_type ASC LIMIT ? OFFSET ?"
		args = append(args, 10, int64(page-1)*10)
		var rows []operationSummaryRow
		if err := tx.SelectContext(ctx, &rows, statement, args...); err != nil {
			return nil, 0, err
		}
		for _, row := range rows {
			items = append(items, model.OperationSummary{Date: row.Date, Action: model.OperationActionName(row.Action), Count: row.Count})
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func operationSummarySQL(bounds model.OperationRange, offsetSeconds int) (string, []any) {
	where, args := operationRangeSQL(bounds)
	statement := `SELECT DATE_FORMAT(DATE_ADD(l.occurred_at, INTERVAL ? SECOND), '%Y-%m-%d') AS operation_date,
 l.action_type,COUNT(*) AS operation_count FROM operation_log l WHERE ` + where + " GROUP BY operation_date,l.action_type"
	return statement, append([]any{offsetSeconds}, args...)
}

// List reads the count and one snapshot page from the same repeatable-read snapshot.
func (s *OperationLogStore) List(ctx context.Context, bounds model.OperationRange, action uint8, page int) ([]model.OperationItem, int64, error) {
	if !appdata.ActionType(action).Valid() {
		return nil, 0, fmt.Errorf("unknown operation action %d", action)
	}
	tx, err := s.db.BeginTxx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, 0, err
	}
	defer tx.Rollback()
	where, args := operationRangeSQL(bounds)
	where += " AND l.action_type=?"
	args = append(args, action)
	var total int64
	if err := tx.GetContext(ctx, &total, "SELECT COUNT(*) FROM operation_log l WHERE "+where, args...); err != nil {
		return nil, 0, err
	}
	items := []model.OperationItem{}
	if int64(page-1)*10 < total {
		args = append(args, 10, int64(page-1)*10)
		suffix := " WHERE " + where + " ORDER BY l.occurred_at DESC,l.id DESC LIMIT ? OFFSET ?"
		if appdata.ActionType(action).IsCardAction() {
			var rows []operationCardRow
			err = tx.SelectContext(ctx, &rows, `SELECT l.id,l.action_type,l.occurred_at,
 d.operation_id,d.printing_uuid,d.oracle_id,d.card_name,d.set_code,d.collector_number,
 d.lang,d.lang_display_name,d.front_url,d.back_url,d.has_distinct_back,d.finish_kind,d.quantity_delta
 FROM operation_log l LEFT JOIN operation_card_detail d ON d.operation_id=l.id`+suffix, args...)
			if err != nil {
				return nil, 0, err
			}
			for _, row := range rows {
				var finish *string
				if row.FinishKind != nil {
					if !row.FinishKind.Valid() {
						return nil, 0, fmt.Errorf("invalid finish snapshot for operation %d", row.ID)
					}
					value := "normal"
					if *row.FinishKind == appdata.FinishFoil {
						value = "foil"
					}
					finish = &value
				}
				items = append(items, model.OperationItem{ID: row.ID, Action: model.OperationActionName(uint8(row.ActionType)), OccurredAt: row.OccurredAt,
					Card: &model.OperationCardSnapshot{PrintingUUID: row.PrintingUUID, Name: row.CardName, SetCode: row.SetCode,
						CollectorNumber: row.CollectorNumber, Language: row.Lang, LanguageDisplayName: row.LangDisplayName,
						Finish: finish, QuantityDelta: row.QuantityDelta, Images: model.CardImages{FrontURL: row.FrontURL, BackURL: row.BackURL}, HasDistinctBack: row.HasDistinctBack}})
			}
		} else {
			var rows []operationDeckRow
			err = tx.SelectContext(ctx, &rows, `SELECT l.id,l.action_type,l.occurred_at,d.operation_id,d.deck_id,d.deck_name
 FROM operation_log l LEFT JOIN operation_deck_detail d ON d.operation_id=l.id`+suffix, args...)
			if err != nil {
				return nil, 0, err
			}
			for _, row := range rows {
				items = append(items, model.OperationItem{ID: row.ID, Action: model.OperationActionName(uint8(row.ActionType)), OccurredAt: row.OccurredAt,
					Deck: &model.OperationDeckSnapshot{ID: row.DeckID, Name: row.DeckName}})
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func operationRangeSQL(bounds model.OperationRange) (string, []any) {
	where := "1=1"
	args := []any{}
	if bounds.Start != nil {
		where += " AND l.occurred_at>=?"
		args = append(args, *bounds.Start)
	}
	if bounds.End != nil {
		where += " AND l.occurred_at<?"
		args = append(args, *bounds.End)
	}
	return where, args
}

// AppendDeck records the deck name snapshot in the same transaction as its mutation.
func (s *OperationLogTx) AppendDeck(ctx context.Context, action string, id uint64, name string) error {
	actions := map[string]appdata.ActionType{"create": appdata.ActionCreateDeck, "copy": appdata.ActionCopyDeck, "save": appdata.ActionModifyDeck, "delete": appdata.ActionDeleteDeck}
	kind, ok := actions[action]
	if !ok {
		return fmt.Errorf("unknown deck operation %q", action)
	}
	result, err := s.tx.ExecContext(ctx, "INSERT INTO operation_log (action_type,occurred_at) VALUES (?,?)", kind, time.Now().UTC())
	if err != nil {
		return err
	}
	operationID, err := result.LastInsertId()
	if err != nil {
		return err
	}
	_, err = s.tx.ExecContext(ctx, "INSERT INTO operation_deck_detail (operation_id,deck_id,deck_name) VALUES (?,?,?)", operationID, id, name)
	return err
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
