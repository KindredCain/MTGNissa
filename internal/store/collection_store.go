package store

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"time"

	"github.com/doug-martin/goqu/v9"
	"github.com/jmoiron/sqlx"

	"mtgnissa/internal/apperror"
	"mtgnissa/internal/model"
)

// CollectionStore reads personal ownership and batches matching metadata from the Card DB.
type CollectionStore struct {
	app   *sqlx.DB
	faces *CardStore
}

type collectionSetTotal struct {
	Code string `db:"set_code"`
	model.OwnershipTotals
}

// CollectionTx reads and changes collection quantities in the caller's transaction.
type CollectionTx struct {
	tx *sqlx.Tx
}

type versionQuantityRow struct {
	UUID string `db:"printing_uuid"`
	model.Quantity
}

// NewCollectionStore creates a collection reader using separate application and card databases.
func NewCollectionStore(app, card *sqlx.DB) *CollectionStore {
	return &CollectionStore{app: app, faces: NewCardStore(card)}
}

// Options resolves only the owned series and language codes through Card DB.
func (s *CollectionStore) Options(ctx context.Context) ([]model.SetRef, []model.Language, error) {
	return personalOptions(ctx, s.app, s.faces, "collection_item")
}

// List groups and pages query metadata in one App DB snapshot without hydrating other pages.
func (s *CollectionStore) List(ctx context.Context, params model.CollectionParams, position *PersonalPosition, pageSize int) (model.CollectionPage, *PersonalPosition, error) {
	page := model.CollectionPage{Items: []model.CollectionItem{}}
	tx, err := s.app.BeginTxx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return page, nil, err
	}
	defer tx.Rollback()
	base := personalBase("collection_item")
	statement, args, err := collectionTotals(base).ToSQL()
	if err != nil {
		return page, nil, err
	}
	if err := tx.GetContext(ctx, &page.Summary, statement, args...); err != nil {
		return page, nil, err
	}
	groups := collectionGroups(personalFilters(base, params), params.MergeSets, params.MergeLanguages)
	page.Total, err = personalCount(ctx, tx, mysqlDialect.From(groups.As("groups")).Prepared(true))
	if err != nil {
		return page, nil, err
	}
	dataset := mysqlDialect.From(groups.As("g")).Join(goqu.T("card_print_ref").As("r"), goqu.On(goqu.I("r.printing_uuid").Eq(goqu.I("g.printing_uuid")))).Prepared(true)
	dataset = personalColumns(dataset).SelectAppend(goqu.I("g.normal_quantity"), goqu.I("g.foil_quantity"))
	rows, next, err := readPersonalPage(ctx, tx, personalPage(dataset, params.Sort, position, pageSize), pageSize)
	if err != nil {
		return page, nil, err
	}
	totals := map[string]model.OwnershipTotals{}
	if params.Sort == "set" && len(rows) > 0 {
		codes := make([]string, 0, len(rows))
		for _, row := range rows {
			codes = append(codes, row.SetCode)
		}
		statement, args, err := collectionTotals(base.Where(goqu.I("r.set_code").In(codes))).SelectAppend(goqu.I("r.set_code")).GroupBy(goqu.I("r.set_code")).ToSQL()
		if err != nil {
			return page, nil, err
		}
		var setTotals []collectionSetTotal
		if err := tx.SelectContext(ctx, &setTotals, statement, args...); err != nil {
			return page, nil, err
		}
		for _, row := range setTotals {
			totals[row.Code] = row.OwnershipTotals
		}
	}
	for _, row := range rows {
		item := model.CollectionItem{Card: model.CardSummary{PrintingUUID: row.UUID}, Quantity: row.Quantity, MergedSets: params.MergeSets, MergedLanguages: params.MergeLanguages}
		if params.Sort == "set" {
			total := totals[row.SetCode]
			item.GroupSummary = &total
		}
		page.Items = append(page.Items, item)
	}
	if err := tx.Commit(); err != nil {
		return page, nil, err
	}
	return page, next, nil
}

func collectionGroups(base *goqu.SelectDataset, mergeSets, mergeLanguages bool) *goqu.SelectDataset {
	identity := goqu.L("CAST(CONCAT('print:',r.printing_uuid) AS BINARY)")
	if mergeSets || mergeLanguages {
		identity = goqu.L("CAST(CASE WHEN NULLIF(r.oracle_id,'') IS NULL THEN CONCAT('print:',r.printing_uuid) ELSE CONCAT('oracle:',r.oracle_id) END AS BINARY)")
	}
	keys := []any{identity}
	if !mergeSets && mergeLanguages {
		keys = append(keys, goqu.L("CAST(r.set_code AS BINARY)"), goqu.L("CAST(r.collector_number AS BINARY)"))
	}
	if mergeSets && !mergeLanguages {
		keys = append(keys, goqu.L("CAST(r.lang AS BINARY)"))
	}
	// An inverse YYYYMMDD prefix selects the newest non-NULL date, then the smallest UUID.
	representative := goqu.L("SUBSTRING(MIN(CONCAT(LPAD(99999999-COALESCE(CAST(DATE_FORMAT(r.released_at, '%Y%m%d') AS UNSIGNED),0),8,'0'),r.printing_uuid)),9)").As("printing_uuid")
	return base.Select(representative, goqu.SUM(goqu.I("c.normal_quantity")).As("normal_quantity"), goqu.SUM(goqu.I("c.foil_quantity")).As("foil_quantity")).GroupBy(keys...)
}

func collectionTotals(base *goqu.SelectDataset) *goqu.SelectDataset {
	return base.Select(goqu.L("COALESCE(SUM(c.normal_quantity+c.foil_quantity),0)").As("total_quantity"),
		goqu.L("COUNT(DISTINCT CAST(CASE WHEN NULLIF(r.oracle_id,'') IS NULL THEN CONCAT('print:',r.printing_uuid) ELSE CONCAT('oracle:',r.oracle_id) END AS BINARY))").As("distinct_cards"))
}

// Summaries resolves complete card summaries only for the current page.
func (s *CollectionStore) Summaries(ctx context.Context, ids []string) (map[string]model.CardSummary, error) {
	return s.faces.Summaries(ctx, ids)
}

// Adjust changes one precise printing and returns the event for transactional logging.
func (s *CollectionTx) Adjust(ctx context.Context, card model.CardSummary, finish string, delta int) (model.CardChange, error) {
	now := time.Now().UTC()
	if err := lockPrintingRef(ctx, s.tx, card, now); err != nil {
		return model.CardChange{}, err
	}
	var quantity model.Quantity
	err := s.tx.GetContext(ctx, &quantity, "SELECT normal_quantity,foil_quantity FROM collection_item WHERE printing_uuid=? FOR UPDATE", card.PrintingUUID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return model.CardChange{}, err
	}
	quantity, err = adjustQuantity(quantity, finish, delta)
	if err != nil {
		return model.CardChange{}, err
	}
	if quantity.Normal+quantity.Foil == 0 {
		_, err = s.tx.ExecContext(ctx, "DELETE FROM collection_item WHERE printing_uuid=?", card.PrintingUUID)
	} else {
		_, err = s.tx.ExecContext(ctx, `INSERT INTO collection_item (printing_uuid,normal_quantity,foil_quantity,created_at,updated_at)
 VALUES (?,?,?,?,?) ON DUPLICATE KEY UPDATE normal_quantity=VALUES(normal_quantity),foil_quantity=VALUES(foil_quantity),updated_at=VALUES(updated_at)`,
			card.PrintingUUID, quantity.Normal, quantity.Foil, now, now)
	}
	if err != nil {
		return model.CardChange{}, err
	}
	return model.CardChange{Card: card, Finish: finish, Delta: delta, OccurredAt: now}, nil
}

// State reads precise quantities and unfiltered collection and series totals.
func (s *CollectionTx) State(ctx context.Context, card model.CardSummary) (model.CollectionState, error) {
	state := model.CollectionState{Card: card}
	err := s.tx.GetContext(ctx, &state.Group, "SELECT normal_quantity,foil_quantity FROM collection_item WHERE printing_uuid=?", card.PrintingUUID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return model.CollectionState{}, err
	}
	const totalsQuery = `SELECT COALESCE(SUM(c.normal_quantity+c.foil_quantity),0) AS total_quantity,
 COUNT(DISTINCT CASE WHEN r.oracle_id IS NULL OR r.oracle_id='' THEN CONCAT('print:',r.printing_uuid) ELSE CONCAT('oracle:',r.oracle_id) END) AS distinct_cards
 FROM collection_item c JOIN card_print_ref r ON r.printing_uuid=c.printing_uuid`
	if err := s.tx.GetContext(ctx, &state.Collection, totalsQuery); err != nil {
		return model.CollectionState{}, err
	}
	if err := s.tx.GetContext(ctx, &state.SetSummary, totalsQuery+" WHERE r.set_code=?", card.Set.Code); err != nil {
		return model.CollectionState{}, err
	}
	return state, nil
}

func (s *CollectionTx) quantities(ctx context.Context, ids []string) (map[string]model.Quantity, error) {
	owned := make(map[string]model.Quantity)
	for start := 0; start < len(ids); start += 500 {
		query, args, err := sqlx.In("SELECT printing_uuid,normal_quantity,foil_quantity FROM collection_item WHERE printing_uuid IN (?)", ids[start:min(start+500, len(ids))])
		if err != nil {
			return nil, err
		}
		var rows []versionQuantityRow
		if err := s.tx.SelectContext(ctx, &rows, query, args...); err != nil {
			return nil, err
		}
		for _, row := range rows {
			owned[row.UUID] = row.Quantity
		}
	}
	return owned, nil
}

func adjustQuantity(quantity model.Quantity, finish string, delta int) (model.Quantity, error) {
	if (finish != "normal" && finish != "foil") || (delta != 1 && delta != -1) {
		return model.Quantity{}, apperror.New(apperror.CodeValidationFailed, "invalid collection adjustment")
	}
	value := &quantity.Normal
	if finish == "foil" {
		value = &quantity.Foil
	}
	if delta < 0 {
		if *value == 0 {
			return model.Quantity{}, apperror.New(apperror.CodeCollectionUnderflow, "collection quantity cannot be negative")
		}
		*value--
	} else {
		if *value >= math.MaxUint32 {
			return model.Quantity{}, apperror.New(apperror.CodeValidationFailed, "collection quantity is at its maximum")
		}
		*value++
	}
	return quantity, nil
}
