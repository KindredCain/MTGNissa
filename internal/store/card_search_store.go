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

// SearchPosition is the decoded compound name and Oracle boundary of a card page.
type SearchPosition struct {
	Name     string
	OracleID string
}

type searchRepresentativeRow struct {
	MainUUID            *string `db:"main_uuid"`
	SourceID            string  `db:"source_id"`
	OracleID            string  `db:"oracle_id"`
	Name                string  `db:"name"`
	MatchedLanguageCode string  `db:"matched_language_code"`
}

type searchEntryRow struct {
	MainUUID      string `db:"main_uuid"`
	PrintingUUID  string `db:"printing_uuid"`
	LanguageCode  string `db:"language_code"`
	LanguageOrder int    `db:"language_order"`
}

// Search evaluates independent Oracle predicates before selecting and paging representatives.
func (s *CardStore) Search(ctx context.Context, query model.SearchQuery, position *SearchPosition, pageSize int) ([]model.CardSummary, int64, bool, error) {
	named := containsSearchName(query.Expression)
	var hits *goqu.SelectDataset
	var err error
	if named {
		hits, err = searchNamedOracleSet(query.Expression)
	} else {
		hits, err = searchOracleSet(query.Expression)
	}
	if err != nil {
		return nil, 0, false, err
	}
	tx, err := s.db.BeginTxx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, 0, false, err
	}
	defer tx.Rollback()
	if named {
		statement, args, err := searchMatchedRepresentatives(hits).ToSQL()
		if err != nil {
			return nil, 0, false, err
		}
		var rows []searchRepresentativeRow
		if err := tx.SelectContext(ctx, &rows, statement, args...); err != nil {
			return nil, 0, false, err
		}
		total := int64(len(rows))
		rows, more := pageMatchedRepresentatives(rows, position, pageSize)
		items, err := readMatchedSearchSummaries(ctx, tx, rows)
		if err != nil {
			return nil, 0, false, err
		}
		if err := tx.Commit(); err != nil {
			return nil, 0, false, err
		}
		return items, total, more, nil
	}
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
	return searchCandidates(node, false)
}

func searchCondition(c model.SearchCondition) (exp.Expression, error) {
	switch c.Field {
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
		switch op {
		case "=":
			op = ">="
		case "!=":
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
	entries, err := readSearchEntries(ctx, tx, representatives)
	if err != nil {
		return nil, err
	}
	sourceIDs := make([]string, 0, len(representatives))
	entriesBySource := make(map[string]searchEntryRow, len(representatives))
	for _, representative := range representatives {
		entry := entries[*representative.MainUUID]
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

func readSearchEntries(ctx context.Context, tx *sqlx.Tx, representatives []searchRepresentativeRow) (map[string]searchEntryRow, error) {
	ids := make([]string, 0, len(representatives))
	for _, row := range representatives {
		if row.MainUUID == nil {
			return nil, fmt.Errorf("search Oracle %s has no representative main face", row.OracleID)
		}
		ids = append(ids, *row.MainUUID)
	}
	var candidates []searchEntryRow
	for start := 0; start < len(ids); start += 500 {
		statement, args, err := sqlx.In(`SELECT p.card_uuid AS main_uuid,p.uuid AS printing_uuid,
 l.code AS language_code,l.sort_order AS language_order
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
		candidates = append(candidates, rows...)
	}
	return selectSearchEntries(representatives, candidates)
}

func selectSearchEntries(representatives []searchRepresentativeRow, candidates []searchEntryRow) (map[string]searchEntryRow, error) {
	languages := make(map[string]string, len(representatives))
	for _, row := range representatives {
		if row.MainUUID == nil {
			return nil, fmt.Errorf("search Oracle %s has no representative main face", row.OracleID)
		}
		languages[*row.MainUUID] = row.MatchedLanguageCode
	}
	entries := make(map[string]searchEntryRow, len(representatives))
	for _, row := range candidates {
		language, exists := languages[row.MainUUID]
		if !exists || language != "" && row.LanguageCode != language {
			continue
		}
		if current, exists := entries[row.MainUUID]; !exists || preferredSearchEntry(row, current) {
			entries[row.MainUUID] = row
		}
	}
	for _, representative := range representatives {
		if _, ok := entries[*representative.MainUUID]; !ok {
			return nil, fmt.Errorf("search representative %s has no main-face language entry", *representative.MainUUID)
		}
	}
	return entries, nil
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

// DeckSearch matches printed names once and pages candidates using global display rules.
func (s *CardStore) DeckSearch(ctx context.Context, query string, exact bool, position *SearchPosition, pageSize int) ([]model.DeckCandidate, int64, bool, error) {
	tx, err := s.db.BeginTxx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, 0, false, err
	}
	defer tx.Rollback()
	statement, args, err := deckSearchRepresentatives(query, exact).ToSQL()
	if err != nil {
		return nil, 0, false, err
	}
	var rows []searchRepresentativeRow
	if err := tx.SelectContext(ctx, &rows, statement, args...); err != nil {
		return nil, 0, false, err
	}
	total := int64(len(rows))
	rows, more := pageMatchedRepresentatives(rows, position, pageSize)
	items, err := readDeckSearchCandidates(ctx, tx, rows, query != "")
	if err != nil {
		return nil, 0, false, err
	}
	if err := tx.Commit(); err != nil {
		return nil, 0, false, err
	}
	return items, total, more, nil
}

func deckSearchRepresentatives(query string, exact bool) *goqu.SelectDataset {
	if query == "" {
		sources := mysqlDialect.From(goqu.T("mtgnissa_card").As("c")).
			Select(goqu.I("c.oracle_id"), goqu.MIN(goqu.I("c.scryfall_id")).As("source_id")).
			Where(goqu.I("c.oracle_id").IsNotNull(), goqu.I("c.oracle_id").Neq("")).GroupBy(goqu.I("c.oracle_id"))
		return searchRepresentatives(sources, nil)
	}
	matches := mysqlDialect.From(goqu.T("mtgnissa_card_printing").As("p")).
		Join(goqu.T("mtgnissa_card").As("c"), goqu.On(goqu.I("c.uuid").Eq(goqu.I("p.card_uuid")))).
		Select(goqu.I("c.oracle_id"), goqu.L("NULL").As("flavor_uuid"), goqu.MIN(goqu.I("p.uuid")).As("printing_uuid")).
		Where(goqu.I("c.oracle_id").IsNotNull(), goqu.I("c.oracle_id").Neq("")).GroupBy(goqu.I("c.oracle_id"))
	if exact {
		matches = matches.Where(goqu.I("p.printed_name").Eq(query))
	} else {
		matches = matches.Where(goqu.L("? LIKE ? ESCAPE '='", goqu.I("p.printed_name"), "%"+escapeLike(query)+"%"))
	}
	return searchMatchedRepresentatives(matches)
}

func readDeckSearchCandidates(ctx context.Context, tx *sqlx.Tx, representatives []searchRepresentativeRow, matched bool) ([]model.DeckCandidate, error) {
	entries, err := readSearchEntries(ctx, tx, representatives)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(representatives))
	for _, row := range representatives {
		ids = append(ids, entries[*row.MainUUID].PrintingUUID)
	}
	candidates := make(map[string]model.DeckCandidate, len(ids))
	for start := 0; start < len(ids); start += 500 {
		statement, args, err := sqlx.In(printingQuery+" WHERE p.uuid IN (?)", ids[start:min(start+500, len(ids))])
		if err != nil {
			return nil, err
		}
		var rows []faceRow
		if err := tx.SelectContext(ctx, &rows, statement, args...); err != nil {
			return nil, err
		}
		if !matched {
			for i := range rows {
				rows[i].ImageID = rows[i].SourceID
			}
		}
		for _, group := range groupFaces(rows) {
			card, faces := summarize(group)
			candidates[card.PrintingUUID] = model.DeckCandidate{Card: card, Faces: faces}
		}
	}
	result := make([]model.DeckCandidate, 0, len(representatives))
	for i, row := range representatives {
		candidate, ok := candidates[ids[i]]
		if !ok || candidate.Card.OracleID == nil || *candidate.Card.OracleID != row.OracleID {
			return nil, fmt.Errorf("deck search candidate identity is unavailable")
		}
		result = append(result, candidate)
	}
	return result, nil
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
