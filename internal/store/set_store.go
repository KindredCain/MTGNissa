package store

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

	"mtgnissa/internal/model"
)

// SetStore reads Set list data from the Card DB.
type SetStore struct {
	db *sqlx.DB
}

type setLanguageRow struct {
	Code        string `db:"code"`
	DisplayName string `db:"display_name"`
	SortOrder   int    `db:"sort_order"`
}

type setListRow struct {
	Code        string     `db:"code"`
	Name        string     `db:"name"`
	DisplayName string     `db:"display_name"`
	SetType     string     `db:"set_type"`
	CardCount   uint64     `db:"card_count"`
	ReleasedAt  *time.Time `db:"released_at"`
	Languages   *string    `db:"languages"`
	ParentCode  *string    `db:"parent_code"`
}

var mysqlDialect = goqu.Dialect("mysql")

// SetPosition is a decoded query boundary for a Set list database page.
type SetPosition struct {
	Value  string
	Code   string
	IsNull bool
}

// NewSetStore creates a Card DB Set list store.
func NewSetStore(db *sqlx.DB) *SetStore {
	return &SetStore{db: db}
}

// List reads a consistent page of complete root groups and their language matrix.
func (s *SetStore) List(ctx context.Context, params model.SetParams, position *SetPosition, pageSize int) (model.SetPage, bool, error) {
	tx, err := s.db.BeginTxx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return model.SetPage{}, false, err
	}
	defer tx.Rollback()

	languages, err := queryLanguages(ctx, tx)
	if err != nil {
		return model.SetPage{}, false, err
	}
	var links []hierarchyRow
	if err := tx.SelectContext(ctx, &links, "SELECT code, parent_code FROM mtgnissa_set"); err != nil {
		return model.SetPage{}, false, err
	}
	hierarchy, err := newSetHierarchy(links)
	if err != nil {
		return model.SetPage{}, false, err
	}
	base := buildSetDataset(params)
	var total int64
	countQuery, countArgs, err := base.Select(goqu.COUNT("*")).ToSQL()
	if err != nil {
		return model.SetPage{}, false, err
	}
	if err := tx.GetContext(ctx, &total, countQuery, countArgs...); err != nil {
		return model.SetPage{}, false, err
	}
	roots, hasMore, err := queryRootSets(ctx, tx, base, languages, params.Sort, position, pageSize)
	if err != nil {
		return model.SetPage{}, false, err
	}
	descendants, err := queryDescendantSets(ctx, tx, hierarchy.descendantCodes(roots), languages)
	if err != nil {
		return model.SetPage{}, false, err
	}
	items, err := hierarchy.expand(roots, descendants)
	if err != nil {
		return model.SetPage{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return model.SetPage{}, false, err
	}
	return model.SetPage{Items: items, Total: total}, hasMore, nil
}

func queryLanguages(ctx context.Context, tx *sqlx.Tx) ([]model.SetLanguage, error) {
	const query = `
		SELECT code, display_name, sort_order
		FROM scryfall_language
		ORDER BY sort_order ASC, code ASC
		LIMIT 11
	`

	var rows []setLanguageRow
	if err := tx.SelectContext(ctx, &rows, query); err != nil {
		return nil, err
	}
	languages := make([]model.SetLanguage, 0, len(rows))
	for _, row := range rows {
		languages = append(languages, model.SetLanguage{Code: row.Code, DisplayName: row.DisplayName, SortOrder: row.SortOrder})
	}
	return languages, nil
}

func queryRootSets(ctx context.Context, tx *sqlx.Tx, base *goqu.SelectDataset, languages []model.SetLanguage, sort model.SetSort, position *SetPosition, pageSize int) ([]model.SetListItem, bool, error) {
	dataset := base
	if position != nil {
		clause, err := cursorExpression(sort, *position)
		if err != nil {
			return nil, false, err
		}
		dataset = dataset.Where(clause)
	}
	items, err := readSets(ctx, tx, dataset.Order(orderExpressions(sort)...).Limit(uint(pageSize+1)), languages)
	if err != nil {
		return nil, false, err
	}
	hasMore := len(items) > pageSize
	if hasMore {
		items = items[:pageSize]
	}
	return items, hasMore, nil
}

func queryDescendantSets(ctx context.Context, tx *sqlx.Tx, codes []string, languages []model.SetLanguage) ([]model.SetListItem, error) {
	if len(codes) == 0 {
		return []model.SetListItem{}, nil
	}
	return readSets(ctx, tx, setDataset().Where(goqu.I("s.code").In(stringsToInterfaces(codes)...)), languages)
}

func readSets(ctx context.Context, tx *sqlx.Tx, dataset *goqu.SelectDataset, languages []model.SetLanguage) ([]model.SetListItem, error) {
	query, args, err := dataset.Select(
		goqu.I("s.code"),
		goqu.I("s.name"),
		goqu.L("COALESCE(?, ?, ?)", goqu.I("st.name"), goqu.I("pt.name"), goqu.I("s.name")).As("display_name"),
		goqu.I("s.set_type"),
		goqu.I("s.card_count"),
		goqu.I("s.released_at"),
		goqu.I("s.languages"),
		goqu.I("s.parent_code"),
	).ToSQL()
	if err != nil {
		return nil, err
	}
	var rows []setListRow
	if err := tx.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, err
	}
	items := make([]model.SetListItem, 0, len(rows))
	for _, row := range rows {
		item := model.SetListItem{
			Code:        row.Code,
			Name:        row.Name,
			DisplayName: row.DisplayName,
			SetType:     row.SetType,
			CardCount:   row.CardCount,
			ReleasedAt:  row.ReleasedAt,
			ParentCode:  row.ParentCode,
			Languages:   make([]model.SetLanguage, 0),
		}
		if row.SetType != "token" {
			available := make(map[string]bool)
			if row.Languages != nil {
				for _, code := range strings.Split(*row.Languages, ",") {
					available[strings.ToLower(strings.TrimSpace(code))] = true
				}
			}
			for _, language := range languages {
				language.Available = available[strings.ToLower(language.Code)]
				item.Languages = append(item.Languages, language)
				if item.DefaultLanguage == nil && language.Available {
					code := language.Code
					item.DefaultLanguage = &code
				}
			}
		}
		items = append(items, item)
	}
	return items, nil
}

func setDataset() *goqu.SelectDataset {
	translations := mysqlDialect.From("set_translation").
		Select(goqu.C("code"), goqu.MIN(goqu.L("NULLIF(TRIM(?), '')", goqu.C("name"))).As("name")).
		GroupBy(goqu.C("code"))
	return mysqlDialect.
		From(goqu.T("mtgnissa_set").As("s")).
		LeftJoin(translations.As("st"), goqu.On(goqu.I("st.code").Eq(goqu.I("s.code")))).
		LeftJoin(translations.As("pt"), goqu.On(
			goqu.I("pt.code").Eq(goqu.I("s.parent_code")),
			goqu.L("? = 'token'", goqu.I("s.set_type")),
		)).
		Prepared(true)
}

func buildSetDataset(params model.SetParams) *goqu.SelectDataset {
	filters := []exp.Expression{goqu.I("s.parent_code").IsNull()}
	if params.Query != "" {
		pattern := "%" + escapeLike(params.Query) + "%"
		filters = append(filters, goqu.Or(
			goqu.L("? LIKE ? ESCAPE '='", goqu.I("s.name"), pattern),
			goqu.L("? LIKE ? ESCAPE '='", goqu.I("s.code"), pattern),
			goqu.L("? LIKE ? ESCAPE '='", goqu.I("st.name"), pattern),
		))
	}
	if len(params.Languages) != 0 {
		choices := make([]exp.Expression, 0, len(params.Languages))
		for _, language := range params.Languages {
			choices = append(choices, goqu.L("FIND_IN_SET(?, ?) > 0", language, goqu.I("s.languages")))
		}
		filters = append(filters, goqu.Or(choices...))
	}
	if len(params.SetTypes) != 0 {
		filters = append(filters, goqu.I("s.set_type").In(stringsToInterfaces(params.SetTypes)...))
	}
	return setDataset().Where(filters...)
}

func cursorExpression(sort model.SetSort, position SetPosition) (exp.Expression, error) {
	column, descending := sortColumn(sort)
	if isDateSort(sort) && position.IsNull {
		return goqu.And(goqu.I(column).IsNull(), goqu.I("s.code").Gt(position.Code)), nil
	}
	value := any(position.Value)
	if sort == model.SortCardCountAsc || sort == model.SortCardCountDesc {
		parsed, err := strconv.ParseUint(position.Value, 10, 64)
		if err != nil {
			return nil, err
		}
		value = parsed
	}
	primary := goqu.I(column).Gt(value)
	if descending {
		primary = goqu.I(column).Lt(value)
	}
	if sort == model.SortCodeAsc || sort == model.SortCodeDesc {
		return primary, nil
	}
	clause := goqu.Or(primary, goqu.And(goqu.I(column).Eq(value), goqu.I("s.code").Gt(position.Code)))
	if isDateSort(sort) {
		return goqu.Or(clause, goqu.I(column).IsNull()), nil
	}
	return clause, nil
}

func sortColumn(sort model.SetSort) (string, bool) {
	switch sort {
	case model.SortReleasedDesc:
		return "s.released_at", true
	case model.SortReleasedAsc:
		return "s.released_at", false
	case model.SortNameAsc:
		return "s.name", false
	case model.SortNameDesc:
		return "s.name", true
	case model.SortCodeAsc:
		return "s.code", false
	case model.SortCodeDesc:
		return "s.code", true
	case model.SortCardCountDesc:
		return "s.card_count", true
	case model.SortCardCountAsc:
		return "s.card_count", false
	default:
		return "s.released_at", true
	}
}

func isDateSort(sort model.SetSort) bool {
	return sort == model.SortReleasedAsc || sort == model.SortReleasedDesc
}

func orderExpressions(sort model.SetSort) []exp.OrderedExpression {
	column, descending := sortColumn(sort)
	primary := goqu.I(column).Asc()
	if descending {
		primary = goqu.I(column).Desc()
	}
	if sort == model.SortCodeAsc || sort == model.SortCodeDesc {
		return []exp.OrderedExpression{primary}
	}
	order := make([]exp.OrderedExpression, 0, 3)
	if isDateSort(sort) {
		order = append(order, goqu.L("? IS NULL", goqu.I(column)).Asc())
	}
	return append(order, primary, goqu.I("s.code").Asc())
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

func (s *SetStore) Options(ctx context.Context) (model.SetOptions, error) {
	tx, err := s.db.BeginTxx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return model.SetOptions{}, err
	}
	defer tx.Rollback()

	languages, err := queryLanguages(ctx, tx)
	if err != nil {
		return model.SetOptions{}, err
	}
	const query = `
		SELECT set_type
		FROM mtgnissa_set
		WHERE parent_code IS NULL AND set_type <> '' AND set_type <> 'token'
		GROUP BY set_type
		ORDER BY MIN(code) ASC, set_type ASC
	`
	setTypes := make([]string, 0)
	if err := tx.SelectContext(ctx, &setTypes, query); err != nil {
		return model.SetOptions{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.SetOptions{}, err
	}
	return model.SetOptions{Languages: languages, SetTypes: setTypes}, nil
}
