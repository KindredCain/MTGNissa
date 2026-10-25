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

// List reads a consistent page of complete root groups and their language matrix.
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
	var links []hierarchyRow
	if err := tx.SelectContext(ctx, &links, "SELECT code, parent_code FROM mtgnissa_set"); err != nil {
		return Page{}, false, err
	}
	hierarchy, err := newSetHierarchy(links)
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
	roots, hasMore, err := queryRootSets(ctx, tx, base, languages, params.Sort, position, pageSize)
	if err != nil {
		return Page{}, false, err
	}
	descendants, err := queryDescendantSets(ctx, tx, hierarchy.descendantCodes(roots), languages)
	if err != nil {
		return Page{}, false, err
	}
	items, err := hierarchy.expand(roots, descendants)
	if err != nil {
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

func queryRootSets(ctx context.Context, tx *sqlx.Tx, base *goqu.SelectDataset, languages []Language, sort Sort, position *cursorPosition, pageSize int) ([]Set, bool, error) {
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

func queryDescendantSets(ctx context.Context, tx *sqlx.Tx, codes []string, languages []Language) ([]Set, error) {
	if len(codes) == 0 {
		return []Set{}, nil
	}
	return readSets(ctx, tx, setDataset().Where(goqu.I("s.code").In(stringsToInterfaces(codes)...)), languages)
}

func readSets(ctx context.Context, tx *sqlx.Tx, dataset *goqu.SelectDataset, languages []Language) ([]Set, error) {
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
	var rows []setRow
	if err := tx.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, err
	}
	items := make([]Set, 0, len(rows))
	for _, row := range rows {
		item := Set{
			Code:        row.Code,
			Name:        row.Name,
			DisplayName: row.DisplayName,
			SetType:     row.SetType,
			CardCount:   row.CardCount,
			ReleasedAt:  row.ReleasedAt,
			ParentCode:  row.ParentCode,
			Languages:   make([]Language, 0),
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

func buildSetDataset(params Params) *goqu.SelectDataset {
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

type languageRow struct {
	Code        string `db:"code"`
	DisplayName string `db:"display_name"`
	SortOrder   int    `db:"sort_order"`
}

type setRow struct {
	Code        string     `db:"code"`
	Name        string     `db:"name"`
	DisplayName string     `db:"display_name"`
	SetType     string     `db:"set_type"`
	CardCount   uint64     `db:"card_count"`
	ReleasedAt  *time.Time `db:"released_at"`
	Languages   *string    `db:"languages"`
	ParentCode  *string    `db:"parent_code"`
}

func cursorExpression(sort Sort, position cursorPosition) (exp.Expression, error) {
	column, descending := sortColumn(sort)
	if isDateSort(sort) && position.isNull {
		return goqu.And(goqu.I(column).IsNull(), goqu.I("s.code").Gt(position.code)), nil
	}
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
	if sort == SortCodeAsc || sort == SortCodeDesc {
		return primary, nil
	}
	clause := goqu.Or(primary, goqu.And(goqu.I(column).Eq(value), goqu.I("s.code").Gt(position.code)))
	if isDateSort(sort) {
		return goqu.Or(clause, goqu.I(column).IsNull()), nil
	}
	return clause, nil
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

func isDateSort(sort Sort) bool {
	return sort == SortReleasedAsc || sort == SortReleasedDesc
}

func orderExpressions(sort Sort) []exp.OrderedExpression {
	column, descending := sortColumn(sort)
	primary := goqu.I(column).Asc()
	if descending {
		primary = goqu.I(column).Desc()
	}
	if sort == SortCodeAsc || sort == SortCodeDesc {
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
