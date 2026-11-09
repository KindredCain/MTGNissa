package store

import (
	"context"
	"database/sql"
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/exp"
	"github.com/jmoiron/sqlx"

	"mtgnissa/internal/appdata"
	"mtgnissa/internal/model"
)

// PersonalPosition identifies a page boundary using the saved query metadata.
type PersonalPosition struct {
	SetDate string
	SetCode string
	Number  string
	Color   int
	Mana    float64
	Name    string
	UUID    string
}

type personalPageRow struct {
	UUID    string  `db:"printing_uuid"`
	SetCode string  `db:"set_code"`
	SetDate string  `db:"set_date"`
	Number  string  `db:"collector_number"`
	Color   int     `db:"color_order"`
	Mana    float64 `db:"cmc"`
	Name    string  `db:"sort_name"`
	model.Quantity
}

type personalOrderTerm struct {
	expression exp.LiteralExpression
	value      any
	descending bool
}

// PrintingRefStore refreshes pending personal query metadata from Card DB.
type PrintingRefStore struct {
	app   *sqlx.DB
	cards *CardStore
}

const personalColorOrder = "CASE r.colors_mask WHEN 1 THEN 0 WHEN 2 THEN 1 WHEN 4 THEN 2 WHEN 8 THEN 3 WHEN 16 THEN 4 ELSE CASE WHEN BIT_COUNT(r.colors_mask)>1 THEN 5 ELSE 6 END END"

const refreshPrintingRefSQL = `UPDATE card_print_ref SET
 oracle_id=:oracle_id,set_code=:set_code,collector_number=:collector_number,collector_sort_key=:collector_sort_key,
 lang=:lang,name=:name,colors_mask=:colors_mask,cmc=:cmc,rarity=:rarity,released_at=:released_at,
 set_released_at=:set_released_at,last_verified_at=:last_verified_at,is_real=TRUE
 WHERE printing_uuid=:printing_uuid AND is_real=FALSE`

// NewPrintingRefStore creates a pending-reference reader and updater.
func NewPrintingRefStore(app *sqlx.DB, cards *CardStore) *PrintingRefStore {
	return &PrintingRefStore{app: app, cards: cards}
}

// CountPending counts all pending references, including those no longer owned or wished for.
func (s *PrintingRefStore) CountPending(ctx context.Context) (int64, error) {
	var total int64
	err := s.app.GetContext(ctx, &total, "SELECT COUNT(*) FROM card_print_ref WHERE is_real=FALSE")
	return total, err
}

// Pending advances by UUID so failed rows are visited only once per initialization.
func (s *PrintingRefStore) Pending(ctx context.Context, after string, limit int) ([]string, error) {
	var ids []string
	err := s.app.SelectContext(ctx, &ids, "SELECT printing_uuid FROM card_print_ref WHERE is_real=FALSE AND printing_uuid>? ORDER BY printing_uuid ASC LIMIT ?", after, limit)
	return ids, err
}

// Load reads query metadata from Card DB for the requested printing UUIDs.
func (s *PrintingRefStore) Load(ctx context.Context, ids []string) (map[string]appdata.CardPrintRef, error) {
	return s.cards.printingRefs(ctx, ids)
}

// Update atomically replaces one pending reference and marks it as verified.
func (s *PrintingRefStore) Update(ctx context.Context, ref appdata.CardPrintRef) error {
	ref.CollectorSortKey = collectorSortKey(ref.CollectorNumber)
	ref.LastVerifiedAt = time.Now().UTC()
	_, err := s.app.NamedExecContext(ctx, refreshPrintingRefSQL, ref)
	return err
}

func personalBase(table string) *goqu.SelectDataset {
	dataset := mysqlDialect.From(goqu.T(table).As("c")).
		Join(goqu.T("card_print_ref").As("r"), goqu.On(goqu.I("r.printing_uuid").Eq(goqu.I("c.printing_uuid")))).Prepared(true)
	if table == "collection_item" {
		dataset = dataset.Where(goqu.Or(goqu.I("c.normal_quantity").Gt(0), goqu.I("c.foil_quantity").Gt(0)))
	}
	return dataset
}

func personalFilters(base *goqu.SelectDataset, params model.CollectionParams) *goqu.SelectDataset {
	if params.Query != "" {
		base = base.Where(goqu.L("LOWER(r.name) COLLATE utf8mb4_bin LIKE ? ESCAPE '='", "%"+escapeLike(params.Query)+"%"))
	}
	for _, filter := range []struct {
		column string
		values []string
	}{
		{"set_code", params.Sets}, {"lang", params.Languages}, {"rarity", params.Rarities},
	} {
		if len(filter.values) > 0 {
			base = base.Where(goqu.I("r." + filter.column).In(filter.values))
		}
	}
	if len(params.Colors) > 0 {
		choices := []exp.Expression{}
		for _, color := range params.Colors {
			switch color {
			case "multicolor":
				choices = append(choices, goqu.L("BIT_COUNT(r.colors_mask)>1"))
			case "colorless":
				choices = append(choices, goqu.I("r.colors_mask").Eq(0))
			default:
				index := strings.Index("WUBRG", color)
				if index >= 0 && len(color) == 1 {
					choices = append(choices, goqu.I("r.colors_mask").Eq(uint64(1)<<index))
				}
			}
		}
		base = base.Where(goqu.Or(choices...))
	}
	if len(params.ManaValues) > 0 {
		choices := []exp.Expression{}
		for _, value := range params.ManaValues {
			if value == "8_plus" {
				choices = append(choices, goqu.I("r.cmc").Gte(8))
			} else if number, err := strconv.ParseFloat(value, 64); err == nil {
				choices = append(choices, goqu.I("r.cmc").Eq(number))
			}
		}
		base = base.Where(goqu.Or(choices...))
	}
	return base
}

func personalPage(base *goqu.SelectDataset, order string, position *PersonalPosition, pageSize int) *goqu.SelectDataset {
	boundary := PersonalPosition{}
	if position != nil {
		boundary = *position
	}
	terms := []personalOrderTerm{}
	switch order {
	case "set":
		terms = append(terms, personalOrderTerm{goqu.L("COALESCE(DATE_FORMAT(r.set_released_at, '%Y-%m-%d'),'')"), boundary.SetDate, true},
			personalOrderTerm{goqu.L("r.set_code COLLATE utf8mb4_bin"), boundary.SetCode, false},
			personalOrderTerm{goqu.L("r.collector_sort_key"), collectorSortKey(boundary.Number), false})
	case "color":
		terms = append(terms, personalOrderTerm{goqu.L(personalColorOrder), boundary.Color, false})
	case "mana_value":
		terms = append(terms, personalOrderTerm{goqu.L("r.cmc"), boundary.Mana, false})
	}
	if order != "set" {
		terms = append(terms, personalOrderTerm{goqu.L("CAST(LOWER(r.name) AS BINARY)"), boundary.Name, false})
	}
	terms = append(terms, personalOrderTerm{goqu.L("r.printing_uuid"), boundary.UUID, false})
	orders := make([]exp.OrderedExpression, 0, len(terms))
	choices := make([]exp.Expression, 0, len(terms))
	equals := []exp.Expression{}
	for _, term := range terms {
		next := term.expression.Gt(term.value)
		if term.descending {
			orders = append(orders, term.expression.Desc())
			next = term.expression.Lt(term.value)
		} else {
			orders = append(orders, term.expression.Asc())
		}
		conditions := append([]exp.Expression{}, equals...)
		choices = append(choices, goqu.And(append(conditions, next)...))
		equals = append(equals, term.expression.Eq(term.value))
	}
	if position != nil {
		base = base.Where(goqu.Or(choices...))
	}
	return base.Order(orders...).Limit(uint(pageSize + 1))
}

func personalColumns(base *goqu.SelectDataset) *goqu.SelectDataset {
	return base.Select(goqu.I("r.printing_uuid"), goqu.I("r.set_code"), goqu.I("r.collector_number"), goqu.I("r.cmc"),
		goqu.L("COALESCE(DATE_FORMAT(r.set_released_at, '%Y-%m-%d'),'')").As("set_date"),
		goqu.L("LOWER(r.name) COLLATE utf8mb4_bin").As("sort_name"), goqu.L(personalColorOrder).As("color_order"))
}

func readPersonalPage(ctx context.Context, tx *sqlx.Tx, dataset *goqu.SelectDataset, pageSize int) ([]personalPageRow, *PersonalPosition, error) {
	statement, args, err := dataset.ToSQL()
	if err != nil {
		return nil, nil, err
	}
	var rows []personalPageRow
	if err := tx.SelectContext(ctx, &rows, statement, args...); err != nil {
		return nil, nil, err
	}
	var next *PersonalPosition
	if len(rows) > pageSize {
		rows = rows[:pageSize]
		last := rows[len(rows)-1]
		next = &PersonalPosition{SetDate: last.SetDate, SetCode: last.SetCode, Number: last.Number, Color: last.Color, Mana: last.Mana, Name: last.Name, UUID: last.UUID}
	}
	return rows, next, nil
}

func personalCount(ctx context.Context, tx *sqlx.Tx, dataset *goqu.SelectDataset) (int64, error) {
	statement, args, err := dataset.Select(goqu.COUNT("*")).ToSQL()
	if err != nil {
		return 0, err
	}
	var total int64
	err = tx.GetContext(ctx, &total, statement, args...)
	return total, err
}

func personalOptions(ctx context.Context, app *sqlx.DB, cards *CardStore, table string) ([]model.SetRef, []model.Language, error) {
	tx, err := app.BeginTxx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback()
	var sets, languages []string
	for _, query := range []struct {
		column string
		values *[]string
	}{{"set_code", &sets}, {"lang", &languages}} {
		statement, args, err := personalBase(table).Select(goqu.I("r." + query.column)).Distinct().ToSQL()
		if err != nil {
			return nil, nil, err
		}
		if err := tx.SelectContext(ctx, query.values, statement, args...); err != nil {
			return nil, nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, nil, err
	}
	resultSets, resultLanguages, err := cards.personalOptions(ctx, sets, languages)
	if err != nil {
		return nil, nil, fmt.Errorf("read personal filter metadata: %w", err)
	}
	return resultSets, resultLanguages, nil
}

// lockPrintingRef inserts missing query metadata and serializes mutations on the shared reference.
func lockPrintingRef(ctx context.Context, tx *sqlx.Tx, card model.CardSummary, now time.Time) error {
	ref := newPrintingRef(card, now)
	// INSERT and duplicate-key UPDATE both hold the reference row lock until commit.
	// Existing query metadata and its initialization flag are deliberately unchanged.
	_, err := tx.NamedExecContext(ctx, `INSERT INTO card_print_ref
 (printing_uuid,oracle_id,set_code,collector_number,collector_sort_key,lang,name,colors_mask,cmc,rarity,released_at,set_released_at,is_real,created_at,last_verified_at)
 VALUES (:printing_uuid,:oracle_id,:set_code,:collector_number,:collector_sort_key,:lang,:name,:colors_mask,:cmc,:rarity,:released_at,:set_released_at,:is_real,:created_at,:last_verified_at)
 ON DUPLICATE KEY UPDATE last_verified_at=VALUES(last_verified_at)`, ref)
	return err
}

func newPrintingRef(card model.CardSummary, now time.Time) appdata.CardPrintRef {
	var colors uint64
	for _, color := range card.Colors {
		if len(color) == 1 {
			if index := strings.IndexByte("WUBRG", color[0]); index >= 0 {
				colors |= 1 << index
			}
		}
	}
	return appdata.CardPrintRef{
		PrintingUUID: card.PrintingUUID, OracleID: card.OracleID, SetCode: card.Set.Code,
		CollectorNumber: card.CollectorNumber, CollectorSortKey: collectorSortKey(card.CollectorNumber),
		Lang: card.Language.Code, Name: card.Name, ColorsMask: colors, CMC: card.CMC, Rarity: card.Rarity,
		ReleasedAt: card.ReleasedAt, SetReleasedAt: card.Set.ReleasedAt, IsReal: true,
		CreatedAt: now, LastVerifiedAt: now,
	}
}

// collectorSortKey preserves CompareCollectorNumbers ordering under binary SQL comparison.
func collectorSortKey(number string) []byte {
	key := make([]byte, 0, len(number))
	for len(number) > 0 {
		if number[0] < '0' || number[0] > '9' {
			key = append(key, number[0])
			number = number[1:]
			continue
		}
		end := 0
		for end < len(number) && number[end] >= '0' && number[end] <= '9' {
			end++
		}
		digits := strings.TrimLeft(number[:end], "0")
		key = append(key, '0')
		key = binary.BigEndian.AppendUint32(key, uint32(len(digits)))
		key = append(key, digits...)
		number = number[end:]
	}
	return key
}
