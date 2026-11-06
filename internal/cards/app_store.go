package cards

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"time"

	"github.com/jmoiron/sqlx"

	"mtgnissa/internal/appdata"
	"mtgnissa/internal/apperror"
)

// AppStore manages personal printing references, quantities, wishlist, and log snapshots.
type AppStore struct {
	db *sqlx.DB
}

type ownedRow struct {
	UUID string `db:"printing_uuid"`
	Quantity
}

type deckRow struct {
	ID       uint64 `db:"id"`
	Name     string `db:"name"`
	Quantity uint64 `db:"quantity"`
	Section  string `db:"section_type"`
}

type personalStateRow struct {
	Normal        uint64 `db:"normal_quantity"`
	Foil          uint64 `db:"foil_quantity"`
	LogicalNormal uint64 `db:"logical_normal"`
	LogicalFoil   uint64 `db:"logical_foil"`
	Included      bool   `db:"included"`
}

// NewAppStore creates a personal card store backed by App DB.
func NewAppStore(db *sqlx.DB) *AppStore {
	return &AppStore{db: db}
}

func (s *AppStore) mutate(ctx context.Context, card Summary, finish string, delta int, included *bool) error {
	tx, err := s.db.BeginTxx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().UTC()
	// Upserting the stable reference locks both existing and initially absent state.
	_, err = tx.ExecContext(ctx, `INSERT INTO card_print_ref (printing_uuid,oracle_id,set_code,collector_number,lang,created_at,last_verified_at)
 VALUES (?,?,?,?,?,?,?) ON DUPLICATE KEY UPDATE last_verified_at=VALUES(last_verified_at)`, card.PrintingUUID, card.OracleID, card.Set.Code, card.CollectorNumber, card.Language.Code, now, now)
	if err != nil {
		return err
	}
	var ref appdata.CardPrintRef
	if err := tx.GetContext(ctx, &ref, "SELECT * FROM card_print_ref WHERE printing_uuid=? FOR UPDATE", card.PrintingUUID); err != nil {
		return err
	}
	if !sameOptional(ref.OracleID, card.OracleID) || ref.SetCode != card.Set.Code || ref.CollectorNumber != card.CollectorNumber || ref.Lang != card.Language.Code {
		return apperror.New(apperror.CodeInternalError, "stored printing identity does not match card data")
	}
	var action appdata.ActionType
	var finishKind *appdata.FinishKind
	var quantityDelta *int
	if included != nil {
		var count int
		if err := tx.GetContext(ctx, &count, "SELECT COUNT(*) FROM wishlist_item WHERE printing_uuid=?", card.PrintingUUID); err != nil {
			return err
		}
		if (count > 0) == *included {
			return tx.Commit()
		}
		if *included {
			action = appdata.ActionAddWishlist
			_, err = tx.ExecContext(ctx, "INSERT INTO wishlist_item (printing_uuid,created_at) VALUES (?,?)", card.PrintingUUID, now)
		} else {
			action = appdata.ActionRemoveWishlist
			_, err = tx.ExecContext(ctx, "DELETE FROM wishlist_item WHERE printing_uuid=?", card.PrintingUUID)
		}
	} else {
		var q Quantity
		err = tx.GetContext(ctx, &q, "SELECT normal_quantity,foil_quantity FROM collection_item WHERE printing_uuid=? FOR UPDATE", card.PrintingUUID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		q, err = adjustQuantity(q, finish, delta)
		if err != nil {
			return err
		}
		if q.Normal+q.Foil == 0 {
			_, err = tx.ExecContext(ctx, "DELETE FROM collection_item WHERE printing_uuid=?", card.PrintingUUID)
		} else {
			_, err = tx.ExecContext(ctx, `INSERT INTO collection_item (printing_uuid,normal_quantity,foil_quantity,created_at,updated_at)
 VALUES (?,?,?,?,?) ON DUPLICATE KEY UPDATE normal_quantity=VALUES(normal_quantity),foil_quantity=VALUES(foil_quantity),updated_at=VALUES(updated_at)`, card.PrintingUUID, q.Normal, q.Foil, now, now)
		}
		action = appdata.ActionAddCard
		if delta < 0 {
			action = appdata.ActionRemoveCard
		}
		kind := appdata.FinishNormal
		if finish == "foil" {
			kind = appdata.FinishFoil
		}
		finishKind = &kind
		quantityDelta = &delta
	}
	if err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, "INSERT INTO operation_log (action_type,occurred_at) VALUES (?,?)", action, now)
	if err != nil {
		return err
	}
	operationID, err := result.LastInsertId()
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO operation_card_detail
 (operation_id,printing_uuid,oracle_id,card_name,set_code,collector_number,lang,lang_display_name,front_url,back_url,has_distinct_back,finish_kind,quantity_delta)
 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`, operationID, card.PrintingUUID, card.OracleID, card.DisplayName, card.Set.Code, card.CollectorNumber, card.Language.Code, card.Language.DisplayName, card.Images.FrontURL, card.Images.BackURL, card.Images.BackURL != nil, finishKind, quantityDelta)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func adjustQuantity(q Quantity, finish string, delta int) (Quantity, error) {
	if (finish != "normal" && finish != "foil") || (delta != 1 && delta != -1) {
		return Quantity{}, apperror.New(apperror.CodeValidationFailed, "invalid collection adjustment")
	}
	value := &q.Normal
	if finish == "foil" {
		value = &q.Foil
	}
	if delta < 0 {
		if *value == 0 {
			return Quantity{}, apperror.New(apperror.CodeCollectionUnderflow, "collection quantity cannot be negative")
		}
		*value--
	} else {
		if *value >= math.MaxUint32 {
			return Quantity{}, apperror.New(apperror.CodeValidationFailed, "collection quantity is at its maximum")
		}
		*value++
	}
	return q, nil
}

func sameOptional(a, b *string) bool {
	return a == nil && b == nil || a != nil && b != nil && *a == *b
}

func readQuantity(ctx context.Context, tx *sqlx.Tx, id string) (Quantity, error) {
	var q Quantity
	err := tx.GetContext(ctx, &q, "SELECT normal_quantity,foil_quantity FROM collection_item WHERE printing_uuid=?", id)
	if errors.Is(err, sql.ErrNoRows) {
		err = nil
	}
	return q, err
}

func (s *AppStore) collectionState(ctx context.Context, card Summary) (CollectionState, error) {
	tx, err := s.db.BeginTxx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return CollectionState{}, err
	}
	defer tx.Rollback()
	state := CollectionState{Card: card}
	state.Group, err = readQuantity(ctx, tx, card.PrintingUUID)
	if err != nil {
		return CollectionState{}, err
	}
	const totalsQuery = `SELECT COALESCE(SUM(c.normal_quantity+c.foil_quantity),0) AS total_quantity,
 COUNT(DISTINCT CASE WHEN r.oracle_id IS NULL OR r.oracle_id='' THEN CONCAT('print:',r.printing_uuid) ELSE CONCAT('oracle:',r.oracle_id) END) AS distinct_cards
 FROM collection_item c JOIN card_print_ref r ON r.printing_uuid=c.printing_uuid`
	if err := tx.GetContext(ctx, &state.Collection, totalsQuery); err != nil {
		return CollectionState{}, err
	}
	if err := tx.GetContext(ctx, &state.SetSummary, totalsQuery+" WHERE r.set_code=?", card.Set.Code); err != nil {
		return CollectionState{}, err
	}
	if err := tx.Commit(); err != nil {
		return CollectionState{}, err
	}
	return state, nil
}

func (s *AppStore) wishlist(ctx context.Context, id string) (bool, error) {
	var count int
	err := s.db.GetContext(ctx, &count, "SELECT COUNT(*) FROM wishlist_item WHERE printing_uuid=?", id)
	return count > 0, err
}

func (s *AppStore) fillDetail(ctx context.Context, d *Detail) error {
	tx, err := s.db.BeginTxx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var state personalStateRow
	if err := tx.GetContext(ctx, &state, `
 SELECT COALESCE(c.normal_quantity,0) AS normal_quantity,COALESCE(c.foil_quantity,0) AS foil_quantity,
 CASE WHEN ? IS NULL THEN COALESCE(c.normal_quantity,0) ELSE COALESCE(logical.normal_quantity,0) END AS logical_normal,
 CASE WHEN ? IS NULL THEN COALESCE(c.foil_quantity,0) ELSE COALESCE(logical.foil_quantity,0) END AS logical_foil,
 EXISTS(SELECT 1 FROM wishlist_item w WHERE w.printing_uuid=requested.printing_uuid) AS included
 FROM (SELECT ? AS printing_uuid) requested
 LEFT JOIN collection_item c ON c.printing_uuid=requested.printing_uuid
 LEFT JOIN (
  SELECT SUM(c.normal_quantity) AS normal_quantity,SUM(c.foil_quantity) AS foil_quantity
  FROM card_print_ref r JOIN collection_item c ON c.printing_uuid=r.printing_uuid WHERE r.oracle_id=?
 ) logical ON 1=1`, d.Card.OracleID, d.Card.OracleID, d.Card.PrintingUUID, d.Card.OracleID); err != nil {
		return err
	}
	d.Collection = Quantity{Normal: state.Normal, Foil: state.Foil}
	d.LogicalCollection = Quantity{Normal: state.LogicalNormal, Foil: state.LogicalFoil}
	d.Included = state.Included
	ids := make([]string, 0)
	for _, option := range d.Languages {
		ids = append(ids, option.PrintingUUID)
	}
	for _, version := range d.OtherVersions {
		for _, option := range version.Languages {
			ids = append(ids, option.PrintingUUID)
		}
	}
	owned := make(map[string]Quantity)
	for start := 0; start < len(ids); start += 500 {
		end := start + 500
		if end > len(ids) {
			end = len(ids)
		}
		query, args, err := sqlx.In("SELECT printing_uuid,normal_quantity,foil_quantity FROM collection_item WHERE printing_uuid IN (?)", ids[start:end])
		if err != nil {
			return err
		}
		var rows []ownedRow
		if err := tx.SelectContext(ctx, &rows, query, args...); err != nil {
			return err
		}
		for _, row := range rows {
			owned[row.UUID] = row.Quantity
		}
	}
	for i := range d.Languages {
		d.Languages[i].Quantity = owned[d.Languages[i].PrintingUUID]
	}
	for i := range d.OtherVersions {
		v := &d.OtherVersions[i]
		for j := range v.Languages {
			q := owned[v.Languages[j].PrintingUUID]
			v.Languages[j].Quantity = q
			v.Quantity.Normal += q.Normal
			v.Quantity.Foil += q.Foil
		}
	}
	if d.Card.OracleID != nil {
		var rows []deckRow
		if err := tx.SelectContext(ctx, &rows, `SELECT d.id,d.name,c.quantity,c.section_type FROM deck_card c JOIN deck d ON d.id=c.deck_id
 WHERE c.oracle_id=? AND c.section_type IN ('commander','mainboard','sideboard')
 ORDER BY d.updated_at DESC,d.id ASC,FIELD(c.section_type,'commander','mainboard','sideboard')`, *d.Card.OracleID); err != nil {
			return err
		}
		positions := make(map[uint64]int)
		for _, row := range rows {
			index, ok := positions[row.ID]
			if !ok {
				index = len(d.Decks)
				positions[row.ID] = index
				d.Decks = append(d.Decks, DeckUsage{ID: row.ID, Name: row.Name, Sections: []string{}})
			}
			d.Decks[index].Quantity += row.Quantity
			d.Decks[index].Sections = append(d.Decks[index].Sections, row.Section)
		}
	}
	err = tx.Commit()
	return err
}
