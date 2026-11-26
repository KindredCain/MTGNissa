package store

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/doug-martin/goqu/v9"
	"github.com/jmoiron/sqlx"

	"mtgnissa/internal/model"
)

func containsSearchName(node *model.SearchExpression) bool {
	if node == nil {
		return false
	}
	return node.Kind == "atom" && node.Condition.Field == "name" || containsSearchName(node.Left) || containsSearchName(node.Right)
}

func searchNamedOracleSet(node *model.SearchExpression) (*goqu.SelectDataset, error) {
	if !containsSearchName(node) {
		hits, err := searchOracleSet(node)
		if err != nil {
			return nil, err
		}
		return mysqlDialect.From(hits.As("hits")).Select(goqu.I("hits.oracle_id"),
			goqu.L("NULL").As("flavor_uuid"), goqu.L("NULL").As("printing_uuid")).Prepared(true), nil
	}
	if node.Kind == "atom" {
		pattern := "%" + escapeLike(node.Condition.Value) + "%"
		base := mysqlDialect.From(goqu.T("mtgnissa_card").As("f")).
			Where(goqu.I("f.oracle_id").IsNotNull(), goqu.I("f.oracle_id").Neq(""))
		flavorNames := base.Select(goqu.I("f.oracle_id"), goqu.MIN(goqu.I("f.uuid")).As("flavor_uuid"), goqu.L("NULL").As("printing_uuid")).
			Where(goqu.I("f.flavor_name").IsNotNull(), goqu.L("? LIKE ? ESCAPE '='", goqu.I("f.flavor_name"), pattern)).GroupBy(goqu.I("f.oracle_id"))
		printedNames := base.Join(goqu.T("mtgnissa_card_printing").As("p"), goqu.On(goqu.I("f.uuid").Eq(goqu.I("p.card_uuid")))).
			Select(goqu.I("f.oracle_id"), goqu.L("NULL").As("flavor_uuid"), goqu.MIN(goqu.I("p.uuid")).As("printing_uuid")).
			Where(goqu.L("? LIKE ? ESCAPE '='", goqu.I("p.printed_name"), pattern)).GroupBy(goqu.I("f.oracle_id"))
		return groupSearchNameMatches(flavorNames.UnionAll(printedNames)), nil
	}
	if node.Kind != "and" && node.Kind != "or" {
		return nil, fmt.Errorf("unknown search expression %q", node.Kind)
	}
	left, err := searchNamedOracleSet(node.Left)
	if err != nil {
		return nil, err
	}
	right, err := searchNamedOracleSet(node.Right)
	if err != nil {
		return nil, err
	}
	if node.Kind == "or" {
		return groupSearchNameMatches(left.UnionAll(right)), nil
	}
	// Only successful branches contribute a version; AND still intersects Oracle identities.
	return mysqlDialect.From(left.As("left_set")).
		Join(right.As("right_set"), goqu.On(goqu.I("left_set.oracle_id").Eq(goqu.I("right_set.oracle_id")))).
		Select(goqu.I("left_set.oracle_id"),
			goqu.L("COALESCE(LEAST(?, ?), ?, ?)", goqu.I("left_set.flavor_uuid"), goqu.I("right_set.flavor_uuid"), goqu.I("left_set.flavor_uuid"), goqu.I("right_set.flavor_uuid")).As("flavor_uuid"),
			goqu.L("COALESCE(LEAST(?, ?), ?, ?)", goqu.I("left_set.printing_uuid"), goqu.I("right_set.printing_uuid"), goqu.I("left_set.printing_uuid"), goqu.I("right_set.printing_uuid")).As("printing_uuid")).Prepared(true), nil
}

func groupSearchNameMatches(matches *goqu.SelectDataset) *goqu.SelectDataset {
	return mysqlDialect.From(matches.As("name_matches")).
		Select(goqu.I("name_matches.oracle_id"), goqu.MIN(goqu.I("name_matches.flavor_uuid")).As("flavor_uuid"),
			goqu.MIN(goqu.I("name_matches.printing_uuid")).As("printing_uuid")).
		GroupBy(goqu.I("name_matches.oracle_id")).Prepared(true)
}

func searchMatchedRepresentatives(matches *goqu.SelectDataset) *goqu.SelectDataset {
	// An OR branch without a name can contribute an Oracle without a matching version.
	fallback := mysqlDialect.From(goqu.T("mtgnissa_card").As("fallback")).
		Select(goqu.MIN(goqu.I("fallback.scryfall_id"))).Where(goqu.I("fallback.oracle_id").Eq(goqu.I("matched.oracle_id")))
	source := goqu.Case().When(goqu.And(goqu.I("matched.flavor_uuid").IsNull(), goqu.I("matched.printing_uuid").IsNull()), fallback).
		Else(goqu.I("face.scryfall_id"))
	return mysqlDialect.From(matches.As("matched")).
		LeftJoin(goqu.T("mtgnissa_card_printing").As("printing"), goqu.On(goqu.I("printing.uuid").Eq(goqu.I("matched.printing_uuid")))).
		LeftJoin(goqu.T("mtgnissa_card").As("face"), goqu.On(goqu.I("face.uuid").Eq(goqu.L("COALESCE(?, ?)", goqu.I("printing.card_uuid"), goqu.I("matched.flavor_uuid"))))).
		LeftJoin(goqu.T("mtgnissa_card").As("main"), goqu.On(goqu.I("main.scryfall_id").Eq(source),
			goqu.I("main.oracle_id").Eq(goqu.I("matched.oracle_id")), goqu.I("main.face_index").In(-1, 0))).
		Select(goqu.I("main.uuid").As("main_uuid"), goqu.I("matched.oracle_id"), goqu.L("COALESCE(?, '')", goqu.I("main.scryfall_id")).As("source_id"),
			goqu.L("COALESCE(?, '')", goqu.I("main.name")).As("name"), goqu.L("COALESCE(?, '')", goqu.I("printing.language_code")).As("matched_language_code")).Prepared(true)
}

func pageMatchedRepresentatives(rows []searchRepresentativeRow, position *SearchPosition, pageSize int) ([]searchRepresentativeRow, bool) {
	// Keep only lightweight identities until the current page is known; never rerun name matching for a count or a cover.
	sort.Slice(rows, func(i, j int) bool {
		left, right := strings.ToLower(rows[i].Name), strings.ToLower(rows[j].Name)
		if left != right {
			return left < right
		}
		return rows[i].OracleID < rows[j].OracleID
	})
	start := 0
	if position != nil {
		start = sort.Search(len(rows), func(i int) bool {
			name := strings.ToLower(rows[i].Name)
			return name > position.Name || name == position.Name && rows[i].OracleID > position.OracleID
		})
	}
	end := min(start+pageSize, len(rows))
	return rows[start:end], end < len(rows)
}

func readMatchedSearchSummaries(ctx context.Context, tx *sqlx.Tx, representatives []searchRepresentativeRow) ([]model.CardSummary, error) {
	entries, err := readSearchEntries(ctx, tx, representatives)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(representatives))
	for _, row := range representatives {
		ids = append(ids, entries[*row.MainUUID].PrintingUUID)
	}
	cards := make(map[string]model.CardSummary, len(ids))
	for start := 0; start < len(ids); start += 500 {
		statement, args, err := sqlx.In(summaryQuery+" WHERE p.uuid IN (?)", ids[start:min(start+500, len(ids))])
		if err != nil {
			return nil, err
		}
		var rows []faceRow
		if err := tx.SelectContext(ctx, &rows, statement, args...); err != nil {
			return nil, err
		}
		for _, group := range groupFaces(rows) {
			card, _ := summarize(group)
			cards[card.PrintingUUID] = card
		}
	}
	result := make([]model.CardSummary, 0, len(representatives))
	for i, row := range representatives {
		card, ok := cards[ids[i]]
		if !ok || card.OracleID == nil || *card.OracleID != row.OracleID {
			return nil, fmt.Errorf("search matched printing identity is unavailable")
		}
		result = append(result, card)
	}
	return result, nil
}
