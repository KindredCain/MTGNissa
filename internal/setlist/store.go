package setlist

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
	"time"

	"github.com/doug-martin/goqu/v9"
	_ "github.com/doug-martin/goqu/v9/dialect/mysql"
	"github.com/doug-martin/goqu/v9/exp"
	"github.com/jmoiron/sqlx"
)

var mysqlDialect = goqu.Dialect("mysql")

// Store reads Set list data from the Card DB.
type Store struct {
	db *sqlx.DB
}

// NewStore creates a Card DB Set list store.
func NewStore(db *sqlx.DB) *Store {
	return &Store{db: db}
}

// List reads a consistent Set page, its total, and the page language matrix.
func (s *Store) List(ctx context.Context, params Params, position *cursorPosition, pageSize int) (Page, bool, error) {
	tx, err := s.db.BeginTxx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return Page{}, false, err
	}
	defer tx.Rollback()

	languages, err := queryLanguages(ctx, tx)
	if err != nil {
		return Page{}, false, err
	}
	base := buildSetDataset(params)
	var total int64
	countQuery, countArgs, err := base.Select(goqu.COUNT("*")).ToSQL()
	if err != nil {
		return Page{}, false, err
	}
	if err := tx.GetContext(ctx, &total, countQuery, countArgs...); err != nil {
		return Page{}, false, err
	}
	items, hasMore, err := querySets(ctx, tx, base, params.Sort, position, pageSize)
	if err != nil {
		return Page{}, false, err
	}
	if err := populateLanguageAvailability(ctx, tx, items, languages); err != nil {
		return Page{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return Page{}, false, err
	}
	return Page{Items: items, Total: total}, hasMore, nil
}

func queryLanguages(ctx context.Context, tx *sqlx.Tx) ([]Language, error) {
	const query = `
		SELECT code, display_name, sort_order
		FROM scryfall_language
		ORDER BY sort_order ASC, code ASC
		LIMIT 11
	`

	var rows []languageRow
	if err := tx.SelectContext(ctx, &rows, query); err != nil {
		return nil, err
	}
	languages := make([]Language, 0, len(rows))
	for _, row := range rows {
		languages = append(languages, Language{Code: row.Code, DisplayName: row.DisplayName, SortOrder: row.SortOrder})
	}
	return languages, nil
}

func querySets(ctx context.Context, tx *sqlx.Tx, base *goqu.SelectDataset, sort Sort, position *cursorPosition, pageSize int) ([]Set, bool, error) {
	dataset := base.Select(
		goqu.I("s.set_id"),
		goqu.I("s.code"),
		goqu.I("s.name"),
		goqu.L("COALESCE(NULLIF(TRIM(?), ''), ?)", goqu.I("st.name"), goqu.I("s.name")).As("display_name"),
		goqu.I("s.set_type"),
		goqu.I("s.card_count"),
		goqu.I("s.released_at"),
	)
	if position != nil {
		clause, err := cursorExpression(sort, *position)
		if err != nil {
			return nil, false, err
		}
		dataset = dataset.Where(clause)
	}
	dataset = dataset.Order(orderExpressions(sort)...).Limit(uint(pageSize + 1))
	query, args, err := dataset.ToSQL()
	if err != nil {
		return nil, false, err
	}
	var rows []setRow
	if err := tx.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, false, err
	}
	items := make([]Set, 0, len(rows))
	for _, row := range rows {
		items = append(items, Set{
			SetID:       row.SetID,
			Code:        row.Code,
			Name:        row.Name,
			DisplayName: row.DisplayName,
			SetType:     row.SetType,
			CardCount:   row.CardCount,
			ReleasedAt:  row.ReleasedAt,
		})
	}
	hasMore := len(items) > pageSize
	if hasMore {
		items = items[:pageSize]
	}
	return items, hasMore, nil
}

func populateLanguageAvailability(ctx context.Context, tx *sqlx.Tx, items []Set, languages []Language) error {
	if len(items) == 0 {
		return nil
	}
	available := make(map[string]map[string]bool, len(items))
	if len(languages) != 0 {
		dataset := mysqlDialect.From("scryfall_card").
			Prepared(true).
			Select("set_id", "lang").
			Distinct().
			Where(
				goqu.I("set_id").In(interfaceValues(items, func(item Set) string {
					return item.SetID
				})...),
				goqu.I("lang").In(interfaceValues(languages, func(language Language) string {
					return language.Code
				})...),
			)
		query, queryArgs, err := dataset.ToSQL()
		if err != nil {
			return err
		}
		var rows []availabilityRow
		if err := tx.SelectContext(ctx, &rows, query, queryArgs...); err != nil {
			return err
		}
		for _, row := range rows {
			if available[row.SetID] == nil {
				available[row.SetID] = make(map[string]bool)
			}
			available[row.SetID][row.Language] = true
		}
	}
	for i := range items {
		items[i].Languages = make([]Language, len(languages))
		for j, language := range languages {
			language.Available = available[items[i].SetID][language.Code]
			items[i].Languages[j] = language
			if items[i].DefaultLanguage == nil && language.Available {
				code := language.Code
				items[i].DefaultLanguage = &code
			}
		}
	}
	return nil
}

func buildSetDataset(params Params) *goqu.SelectDataset {
	dataset := mysqlDialect.
		From(goqu.T("scryfall_set").As("s")).
		LeftJoin(
			goqu.T("set_translation").As("st"),
			goqu.On(goqu.I("st.set_id").Eq(goqu.I("s.set_id"))),
		).
		Prepared(true)
	filters := make([]exp.Expression, 0, 3)
	if params.Query != "" {
		pattern := "%" + escapeLike(params.Query) + "%"
		filters = append(filters, goqu.Or(
			goqu.L("? LIKE ? ESCAPE '='", goqu.I("s.name"), pattern),
			goqu.L("? LIKE ? ESCAPE '='", goqu.I("s.code"), pattern),
			goqu.L("? LIKE ? ESCAPE '='", goqu.I("st.name"), pattern),
		))
	}
	if len(params.Languages) != 0 {
		available := mysqlDialect.
			From(goqu.T("scryfall_card").As("c")).
			Select(goqu.L("1")).
			Where(
				goqu.I("c.set_id").Eq(goqu.I("s.set_id")),
				goqu.I("c.lang").In(stringsToInterfaces(params.Languages)...),
			)
		filters = append(filters, goqu.L("EXISTS (?)", available))
	}
	if len(params.SetTypes) != 0 {
		filters = append(filters, goqu.I("s.set_type").In(stringsToInterfaces(params.SetTypes)...))
	}
	return dataset.Where(filters...)
}

type languageRow struct {
	Code        string `db:"code"`
	DisplayName string `db:"display_name"`
	SortOrder   int    `db:"sort_order"`
}

type setRow struct {
	SetID       string    `db:"set_id"`
	Code        string    `db:"code"`
	Name        string    `db:"name"`
	DisplayName string    `db:"display_name"`
	SetType     string    `db:"set_type"`
	CardCount   uint64    `db:"card_count"`
	ReleasedAt  time.Time `db:"released_at"`
}

type availabilityRow struct {
	SetID    string `db:"set_id"`
	Language string `db:"lang"`
}

func cursorExpression(sort Sort, position cursorPosition) (exp.Expression, error) {
	column, descending := sortColumn(sort)
	value := any(position.value)
	if sort == SortCardCountAsc || sort == SortCardCountDesc {
		parsed, err := strconv.ParseUint(position.value, 10, 64)
		if err != nil {
			return nil, err
		}
		value = parsed
	}
	primary := goqu.I(column).Gt(value)
	if descending {
		primary = goqu.I(column).Lt(value)
	}
	return goqu.Or(
		primary,
		goqu.And(goqu.I(column).Eq(value), goqu.I("s.set_id").Gt(position.setID)),
	), nil
}

func sortColumn(sort Sort) (string, bool) {
	switch sort {
	case SortReleasedDesc:
		return "s.released_at", true
	case SortReleasedAsc:
		return "s.released_at", false
	case SortNameAsc:
		return "s.name", false
	case SortNameDesc:
		return "s.name", true
	case SortCodeAsc:
		return "s.code", false
	case SortCodeDesc:
		return "s.code", true
	case SortCardCountDesc:
		return "s.card_count", true
	case SortCardCountAsc:
		return "s.card_count", false
	default:
		return "s.released_at", true
	}
}

func orderExpressions(sort Sort) []exp.OrderedExpression {
	column, descending := sortColumn(sort)
	primary := goqu.I(column).Asc()
	if descending {
		primary = goqu.I(column).Desc()
	}
	return []exp.OrderedExpression{primary, goqu.I("s.set_id").Asc()}
}

func escapeLike(value string) string {
	value = strings.ReplaceAll(value, "=", "==")
	value = strings.ReplaceAll(value, "%", "=%")
	return strings.ReplaceAll(value, "_", "=_")
}

func stringsToInterfaces(values []string) []any {
	result := make([]any, len(values))
	for i, value := range values {
		result[i] = value
	}
	return result
}

func interfaceValues[T any](values []T, value func(T) string) []any {
	result := make([]any, len(values))
	for i, item := range values {
		result[i] = value(item)
	}
	return result
}
