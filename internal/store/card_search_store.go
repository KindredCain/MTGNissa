package store

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"

	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/exp"
	"github.com/jmoiron/sqlx"

	"mtgnissa/internal/model"
)

type cardNameRow struct {
	Name     string  `db:"name"`
	OracleID *string `db:"oracle_id"`
}

// SearchPosition is the decoded compound name and Oracle boundary of a card page.
type SearchPosition struct {
	Name     string
	OracleID string
}

type searchRepresentativeRow struct {
	MainUUID *string `db:"main_uuid"`
	SourceID string  `db:"source_id"`
	OracleID string  `db:"oracle_id"`
	Name     string  `db:"name"`
}

type searchEntryRow struct {
	MainUUID      string `db:"main_uuid"`
	PrintingUUID  string `db:"printing_uuid"`
	LanguageCode  string `db:"language_code"`
	LanguageName  string `db:"language_name"`
	LanguageOrder int    `db:"language_order"`
}

// Search evaluates independent Oracle predicates before selecting and paging representatives.
func (s *CardStore) Search(ctx context.Context, query model.SearchQuery, position *SearchPosition, pageSize int) ([]model.CardSummary, int64, bool, error) {
	hits, err := searchOracleSet(query.Expression)
	if err != nil {
		return nil, 0, false, err
	}
	tx, err := s.db.BeginTxx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, 0, false, err
	}
	defer tx.Rollback()
	hitSQL, hitArgs, err := hits.ToSQL()
	if err != nil {
		return nil, 0, false, err
	}
	var oracleIDs []string
	err = tx.SelectContext(ctx, &oracleIDs, hitSQL, hitArgs...)
	if err != nil {
		return nil, 0, false, err
	}
	total := int64(len(oracleIDs))
	if total == 0 {
		err = tx.Commit()
		return []model.CardSummary{}, 0, false, err
	}
	matches := searchRepresentativeSources(oracleIDs)
	dataset := searchRepresentatives(matches, position).Limit(uint(pageSize + 1))
	statement, args, err := dataset.ToSQL()
	if err != nil {
		return nil, 0, false, err
	}
	var rows []searchRepresentativeRow
	err = tx.SelectContext(ctx, &rows, statement, args...)
	if err != nil {
		return nil, 0, false, err
	}
	more := len(rows) > pageSize
	if more {
		rows = rows[:pageSize]
	}
	items, err := readSearchSummaries(ctx, tx, rows)
	if err != nil {
		return nil, 0, false, err
	}
	err = tx.Commit()
	if err != nil {
		return nil, 0, false, err
	}
	return items, total, more, nil
}

func searchRepresentativeSources(oracleIDs []string) *goqu.SelectDataset {
	return mysqlDialect.From(goqu.T("mtgnissa_card").As("c")).
		Select(goqu.I("c.oracle_id"), goqu.MIN(goqu.I("c.scryfall_id")).As("source_id")).
		Where(goqu.I("c.oracle_id").In(oracleIDs)).
		GroupBy(goqu.I("c.oracle_id")).Prepared(true)
}

func searchOracleSet(node *model.SearchExpression) (*goqu.SelectDataset, error) {
	if node == nil {
		return nil, fmt.Errorf("missing search expression")
	}
	if node.Kind == "and" || node.Kind == "or" {
		left, err := searchOracleSet(node.Left)
		if err != nil {
			return nil, err
		}
		right, err := searchOracleSet(node.Right)
		if err != nil {
			return nil, err
		}
		if node.Kind == "and" {
			return mysqlDialect.From(left.As("left_set")).
				Join(right.As("right_set"), goqu.On(goqu.I("left_set.oracle_id").Eq(goqu.I("right_set.oracle_id")))).
				Select(goqu.I("left_set.oracle_id")).Distinct().Prepared(true), nil
		}
		return left.Union(right).Prepared(true), nil
	}
	if node.Kind != "atom" {
		return nil, fmt.Errorf("unknown search expression %q", node.Kind)
	}
	predicate, err := searchCondition(node.Condition)
	if err != nil {
		return nil, err
	}
	// Each atom gets its own face/printing scope; conjunction never joins face predicates.
	return mysqlDialect.From(goqu.T("mtgnissa_card").As("f")).Select(goqu.I("f.oracle_id")).Distinct().
		Where(goqu.I("f.oracle_id").IsNotNull(), goqu.I("f.oracle_id").Neq(""), predicate).Prepared(true), nil
}

func searchCondition(c model.SearchCondition) (exp.Expression, error) {
	switch c.Field {
	case "name":
		return goqu.L("? LIKE ? ESCAPE '='", goqu.I("f.name"), "%"+escapeLike(c.Value)+"%"), nil
	case "c", "id":
		return searchColorCondition(c)
	case "mv", "pow", "tou", "pt", "loy":
		columns := map[string]string{"mv": "cmc", "pow": "power_numeric", "tou": "toughness_numeric", "pt": "pt_numeric", "loy": "loyalty_numeric"}
		left := goqu.I("f." + columns[c.Field])
		if c.ValueField != "" {
			right, ok := columns[c.ValueField]
			if !ok || c.ValueField == "mv" || c.ValueField == "pt" {
				return nil, fmt.Errorf("invalid numeric search operand")
			}
			return searchComparison(left, c.Operator, goqu.I("f."+right))
		}
		number, err := strconv.ParseFloat(c.Value, 64)
		if err != nil {
			return nil, err
		}
		return searchComparison(left, c.Operator, number)
	case "t":
		if c.Quoted {
			return goqu.L("? LIKE ? ESCAPE '='", goqu.I("f.type_line"), "%"+escapeLike(c.Value)+"%"), nil
		}
		types := mysqlDialect.From("scryfall_card_type").Select("card_uuid").Where(goqu.L("? LIKE ? ESCAPE '='", goqu.C("type_name"), escapeLike(c.Value)+"%"))
		return goqu.L("? IN ?", goqu.I("f.uuid"), types), nil
	case "kw":
		keywords := mysqlDialect.From("scryfall_card_keyword").Select("card_uuid").Where(goqu.C("keyword").Eq(c.Value))
		return goqu.L("? IN ?", goqu.I("f.uuid"), keywords), nil
	case "set":
		sets := mysqlDialect.From("mtgnissa_set").Select("code").Where(goqu.Or(goqu.C("code").Eq(c.Value), goqu.C("name").Eq(c.Value)))
		return goqu.L("? IN ?", goqu.I("f.set_code"), sets), nil
	case "otag":
		tags := mysqlDialect.From(goqu.T("oracle_tag_mapping").As("tagging")).
			Join(goqu.T("oracle_tag").As("tag"), goqu.On(goqu.I("tag.tag_id").Eq(goqu.I("tagging.tag_id")))).
			Select(goqu.I("tagging.oracle_id")).Where(goqu.I("tag.label").Eq(c.Value))
		return goqu.L("? IN ?", goqu.I("f.oracle_id"), tags), nil
	case "is":
		switch c.Value {
		case "split", "flip", "transform", "meld", "leveler":
			return goqu.I("f.layout").Eq(c.Value), nil
		case "spell", "permanent", "historic":
			kinds := map[string][]string{"spell": {"Instant", "Sorcery"}, "permanent": {"Artifact", "Battle", "Creature", "Enchantment", "Land", "Planeswalker"}, "historic": {"Artifact", "Legendary", "Saga"}}
			types := mysqlDialect.From("scryfall_card_type").Select("card_uuid").Where(goqu.C("type_name").In(stringsToInterfaces(kinds[c.Value])...))
			return goqu.L("? IN ?", goqu.I("f.uuid"), types), nil
		}
	case "r":
		ranks := map[string]int{"common": 0, "uncommon": 1, "rare": 2, "mythic": 3, "special": 4, "bonus": 5}
		rank, ok := ranks[c.Value]
		if !ok {
			return nil, fmt.Errorf("invalid search rarity")
		}
		order := goqu.Case()
		for _, rarity := range []string{"common", "uncommon", "rare", "mythic", "special", "bonus"} {
			order = order.When(goqu.I("f.rarity").Eq(rarity), ranks[rarity])
		}
		return searchComparison(order.Else(nil), c.Operator, rank)
	}
	return nil, fmt.Errorf("unsupported search condition %q", c.Field)
}

func searchColorCondition(c model.SearchCondition) (exp.Expression, error) {
	column := goqu.I("f.colors_mask")
	if c.Field == "id" {
		column = goqu.I("f.color_identity_mask")
	}
	if c.ColorCount != nil {
		return searchComparison(goqu.L("BIT_COUNT(?)", column), c.Operator, *c.ColorCount)
	}
	if c.Multicolor {
		op := c.Operator
		if op == "=" {
			op = ">="
		} else if op == "!=" {
			op = "<"
		}
		return searchComparison(goqu.L("BIT_COUNT(?)", column), op, 2)
	}
	mask := c.ColorMask
	switch c.Operator {
	case "=", "!=":
		return searchComparison(column, c.Operator, mask)
	case ">", ">=":
		contains := goqu.L("(? & ?) = ?", column, mask, mask)
		if c.Operator == ">" {
			return goqu.And(contains, column.Neq(mask)), nil
		}
		return contains, nil
	case "<", "<=":
		subset := goqu.L("(? & ?) = ?", column, mask, column)
		if c.Operator == "<" {
			return goqu.And(subset, column.Neq(mask)), nil
		}
		return subset, nil
	}
	return nil, fmt.Errorf("unsupported color comparison")
}

func searchComparison(left exp.Expression, op string, right any) (exp.Expression, error) {
	switch op {
	case "=", "!=", ">", ">=", "<", "<=":
		return goqu.L("? "+op+" ?", left, right), nil
	}
	return nil, fmt.Errorf("unsupported comparison %q", op)
}

func searchRepresentatives(matches *goqu.SelectDataset, position *SearchPosition) *goqu.SelectDataset {
	name := goqu.L("LOWER(?) COLLATE utf8mb4_bin", goqu.I("main.name"))
	dataset := mysqlDialect.From(matches.As("matched")).
		LeftJoin(goqu.T("mtgnissa_card").As("main"), goqu.On(goqu.I("main.oracle_id").Eq(goqu.I("matched.oracle_id")), goqu.I("main.scryfall_id").Eq(goqu.I("matched.source_id")), goqu.I("main.face_index").In(-1, 0))).
		Select(goqu.I("main.uuid").As("main_uuid"), goqu.I("matched.source_id"), goqu.I("matched.oracle_id"), goqu.L("COALESCE(?, '')", goqu.I("main.name")).As("name")).Prepared(true)
	if position != nil {
		dataset = dataset.Where(goqu.Or(name.Gt(position.Name), goqu.And(name.Eq(position.Name), goqu.I("matched.oracle_id").Gt(position.OracleID))))
	}
	return dataset.Order(name.Asc(), goqu.I("matched.oracle_id").Asc())
}

func readSearchSummaries(ctx context.Context, tx *sqlx.Tx, representatives []searchRepresentativeRow) ([]model.CardSummary, error) {
	result := make([]model.CardSummary, 0, len(representatives))
	ids := make([]string, 0, len(representatives))
	for _, row := range representatives {
		if row.MainUUID == nil {
			return nil, fmt.Errorf("search Oracle %s has no representative main face", row.OracleID)
		}
		ids = append(ids, *row.MainUUID)
	}
	entries := make(map[string]searchEntryRow)
	for start := 0; start < len(ids); start += 500 {
		statement, args, err := sqlx.In(`SELECT p.card_uuid AS main_uuid,p.uuid AS printing_uuid,
 l.code AS language_code,l.display_name AS language_name,l.sort_order AS language_order
 FROM mtgnissa_card_printing p JOIN scryfall_language l ON l.code=p.language_code
 WHERE p.card_uuid IN (?)`, ids[start:min(start+500, len(ids))])
		if err != nil {
			return nil, err
		}
		var rows []searchEntryRow
		err = tx.SelectContext(ctx, &rows, statement, args...)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			if current, exists := entries[row.MainUUID]; !exists || preferredSearchEntry(row, current) {
				entries[row.MainUUID] = row
			}
		}
	}
	sourceIDs := make([]string, 0, len(representatives))
	entriesBySource := make(map[string]searchEntryRow, len(representatives))
	for _, representative := range representatives {
		entry, ok := entries[*representative.MainUUID]
		if !ok {
			return nil, fmt.Errorf("search representative %s has no main-face language entry", *representative.MainUUID)
		}
		sourceIDs = append(sourceIDs, representative.SourceID)
		entriesBySource[representative.SourceID] = entry
	}
	cards := make(map[string]model.CardSummary)
	for start := 0; start < len(sourceIDs); start += 500 {
		// The chosen main-face printing supplies the language entry for every base face.
		// Pictures always come from the representative base printing, not its language row.
		statement, args, err := sqlx.In(`SELECT `+cardSummaryColumns+`
 FROM mtgnissa_card c
 JOIN mtgnissa_set s ON s.code=c.set_code
 JOIN scryfall_layout ly ON ly.code=c.layout
 LEFT JOIN card_translation ct ON ct.card_id=c.uuid
 WHERE c.scryfall_id IN (?)`, sourceIDs[start:min(start+500, len(sourceIDs))])
		if err != nil {
			return nil, err
		}
		var rows []faceRow
		err = tx.SelectContext(ctx, &rows, statement, args...)
		if err != nil {
			return nil, err
		}
		for i := range rows {
			entry := entriesBySource[rows[i].SourceID]
			rows[i].UUID = entry.PrintingUUID
			rows[i].LanguageCode = entry.LanguageCode
			rows[i].LanguageName = entry.LanguageName
			rows[i].LanguageOrder = entry.LanguageOrder
			rows[i].ImageID = rows[i].SourceID
		}
		for _, group := range groupFaces(rows) {
			card, _ := summarize(group)
			cards[card.PrintingUUID] = card
		}
	}
	for _, representative := range representatives {
		card, ok := cards[entries[*representative.MainUUID].PrintingUUID]
		if !ok || card.OracleID == nil || *card.OracleID != representative.OracleID {
			return nil, fmt.Errorf("search representative identity is unavailable")
		}
		result = append(result, card)
	}
	return result, nil
}

func preferredSearchEntry(candidate, current searchEntryRow) bool {
	if candidate.LanguageOrder != current.LanguageOrder {
		return candidate.LanguageOrder < current.LanguageOrder
	}
	if candidate.LanguageCode != current.LanguageCode {
		return candidate.LanguageCode < current.LanguageCode
	}
	return candidate.PrintingUUID < current.PrintingUUID
}

// DeckSearch pages logical candidates using literal printed-name contains matching.
func (s *CardStore) DeckSearch(ctx context.Context, query string, position *SearchPosition, pageSize int) ([]model.DeckName, int64, bool, error) {
	tx, err := s.db.BeginTxx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, 0, false, err
	}
	defer tx.Rollback()
	matches := deckSearchMatches(query)
	statement, args, err := mysqlDialect.From(matches.As("matched")).Select(goqu.COUNT("*")).Prepared(true).ToSQL()
	if err != nil {
		return nil, 0, false, err
	}
	var total int64
	if err := tx.GetContext(ctx, &total, statement, args...); err != nil {
		return nil, 0, false, err
	}
	statement, args, err = deckSearchPage(matches, position, pageSize).ToSQL()
	if err != nil {
		return nil, 0, false, err
	}
	var rows []cardNameRow
	if err := tx.SelectContext(ctx, &rows, statement, args...); err != nil {
		return nil, 0, false, err
	}
	more := len(rows) > pageSize
	if more {
		rows = rows[:pageSize]
	}
	result := make([]model.DeckName, 0, len(rows))
	for _, row := range rows {
		result = append(result, model.DeckName{Name: row.Name, OracleID: row.OracleID})
	}
	if err := tx.Commit(); err != nil {
		return nil, 0, false, err
	}
	return result, total, more, nil
}

func deckSearchMatches(query string) *goqu.SelectDataset {
	dataset := mysqlDialect.From(goqu.T("mtgnissa_card_printing").As("p")).
		Join(goqu.T("mtgnissa_card").As("c"), goqu.On(goqu.I("c.uuid").Eq(goqu.I("p.card_uuid")))).
		Join(goqu.T("mtgnissa_card").As("main"), goqu.On(goqu.I("main.scryfall_id").Eq(goqu.I("c.scryfall_id")), goqu.I("main.face_index").In(-1, 0))).
		Select(goqu.MIN(goqu.I("main.name")).As("name"), goqu.I("main.oracle_id")).
		Where(goqu.I("main.oracle_id").IsNotNull(), goqu.I("main.oracle_id").Neq("")).GroupBy(goqu.I("main.oracle_id")).Prepared(true)
	if query != "" {
		dataset = dataset.Where(goqu.L("? LIKE ? ESCAPE '='", goqu.I("p.printed_name"), "%"+escapeLike(query)+"%"))
	}
	return dataset
}

func deckSearchPage(matches *goqu.SelectDataset, position *SearchPosition, pageSize int) *goqu.SelectDataset {
	name := goqu.L("LOWER(matched.name) COLLATE utf8mb4_bin")
	oracle := goqu.I("matched.oracle_id")
	dataset := mysqlDialect.From(matches.As("matched")).Select(goqu.I("matched.name"), oracle).Prepared(true)
	if position != nil {
		dataset = dataset.Where(goqu.Or(name.Gt(position.Name), goqu.And(name.Eq(position.Name), oracle.Gt(position.OracleID))))
	}
	return dataset.Order(name.Asc(), oracle.Asc()).Limit(uint(pageSize + 1))
}

// DeckNameSuggestions returns at most five case-insensitively distinct printed names.
func (s *CardStore) DeckNameSuggestions(ctx context.Context, query string) ([]string, error) {
	var names []string
	err := s.db.SelectContext(ctx, &names, `SELECT CONVERT(MIN(BINARY p.printed_name) USING utf8mb4) AS name FROM mtgnissa_card_printing p
 JOIN mtgnissa_card c ON c.uuid=p.card_uuid WHERE NULLIF(c.oracle_id,'') IS NOT NULL AND TRIM(p.printed_name)<>''
 AND p.printed_name LIKE ? ESCAPE '=' GROUP BY LOWER(p.printed_name)
 ORDER BY LOWER(CONVERT(MIN(BINARY p.printed_name) USING utf8mb4)),MIN(BINARY p.printed_name) LIMIT 5`, escapeLike(query)+"%")
	if names == nil {
		names = []string{}
	}
	return names, err
}
