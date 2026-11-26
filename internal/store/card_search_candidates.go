package store

import (
	"fmt"

	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/exp"

	"mtgnissa/internal/model"
)

type searchCandidateColumns struct {
	Match    string
	Flavor   string
	Printing string
}

func searchCandidates(node *model.SearchExpression, named bool) (*goqu.SelectDataset, error) {
	if node == nil {
		return nil, fmt.Errorf("missing search expression")
	}
	if node.Kind == "atom" {
		if node.Condition.Field == "name" {
			return searchNameOracleSet(node.Condition), nil
		}
		predicate, err := searchCondition(node.Condition)
		if err != nil {
			return nil, err
		}
		hits := mysqlDialect.From(goqu.T("mtgnissa_card").As("f")).Select(goqu.I("f.oracle_id")).Distinct().
			Where(goqu.I("f.oracle_id").IsNotNull(), goqu.I("f.oracle_id").Neq(""), predicate)
		if named {
			hits = hits.SelectAppend(goqu.L("NULL").As("flavor_uuid"), goqu.L("NULL").As("printing_uuid"))
		}
		return hits.Prepared(true), nil
	}
	if node.Kind != "and" && node.Kind != "or" {
		return nil, fmt.Errorf("unknown search expression %q", node.Kind)
	}
	left, err := searchCandidates(node.Left, named)
	if err != nil {
		return nil, err
	}
	if node.Kind == "and" {
		return filterSearchCandidates(left, node.Right, named)
	}
	right, err := searchCandidates(node.Right, named)
	if err != nil {
		return nil, err
	}
	if named {
		return groupSearchNameMatches(left.UnionAll(right)), nil
	}
	return left.Union(right).Prepared(true), nil
}

func filterSearchCandidates(candidates *goqu.SelectDataset, node *model.SearchExpression, named bool) (*goqu.SelectDataset, error) {
	if node == nil {
		return nil, fmt.Errorf("missing search expression")
	}
	if node.Kind == "and" {
		left, err := filterSearchCandidates(candidates, node.Left, named)
		if err != nil {
			return nil, err
		}
		return filterSearchCandidates(left, node.Right, named)
	}
	if !containsSearchName(node) {
		predicate, err := searchCandidatePredicate(node)
		if err != nil {
			return nil, err
		}
		hits := mysqlDialect.From(candidates.As("candidate")).Select(goqu.I("candidate.oracle_id")).Where(predicate)
		if named {
			hits = hits.SelectAppend(goqu.I("candidate.flavor_uuid"), goqu.I("candidate.printing_uuid"))
		}
		return hits.Prepared(true), nil
	}
	next := 0
	matches, columns, err := appendSearchCandidateMatch(candidates, node, nil, &next)
	if err != nil {
		return nil, err
	}
	return mysqlDialect.From(matches.As("candidate")).Select(goqu.I("candidate.oracle_id"),
		minimumSearchVersion(goqu.I("candidate.flavor_uuid"), goqu.I("candidate."+columns.Flavor)).As("flavor_uuid"),
		minimumSearchVersion(goqu.I("candidate.printing_uuid"), goqu.I("candidate."+columns.Printing)).As("printing_uuid")).
		Where(goqu.I("candidate." + columns.Match).Eq(true)).Prepared(true), nil
}

func searchCandidatePredicate(node *model.SearchExpression) (exp.Expression, error) {
	if node == nil {
		return nil, fmt.Errorf("missing search expression")
	}
	if node.Kind == "and" || node.Kind == "or" {
		left, err := searchCandidatePredicate(node.Left)
		if err != nil {
			return nil, err
		}
		right, err := searchCandidatePredicate(node.Right)
		if err != nil {
			return nil, err
		}
		if node.Kind == "and" {
			return goqu.And(left, right), nil
		}
		return goqu.Or(left, right), nil
	}
	if node.Kind != "atom" {
		return nil, fmt.Errorf("unknown search expression %q", node.Kind)
	}
	predicate, err := searchCondition(node.Condition)
	if err != nil {
		return nil, err
	}
	// LIMIT keeps each atom correlated to the candidate instead of materializing a global semijoin set.
	match := mysqlDialect.From(goqu.T("mtgnissa_card").As("f")).Select(goqu.L("1")).
		Where(goqu.I("f.oracle_id").Eq(goqu.I("candidate.oracle_id")), predicate).Limit(1)
	return goqu.L("(?) IS NOT NULL", match), nil
}

func appendSearchCandidateMatch(candidates *goqu.SelectDataset, node *model.SearchExpression, guard exp.Expression, next *int) (*goqu.SelectDataset, searchCandidateColumns, error) {
	if node == nil {
		return nil, searchCandidateColumns{}, fmt.Errorf("missing search expression")
	}
	prefix := fmt.Sprintf("condition_%d_", *next)
	*next = *next + 1
	columns := searchCandidateColumns{Match: prefix + "match", Flavor: prefix + "flavor", Printing: prefix + "printing"}
	if node.Kind == "atom" {
		base := mysqlDialect.From(candidates.As("candidate")).Select(goqu.I("candidate.*"))
		if node.Condition.Field == "name" {
			flavor, printing := searchCandidateNameVersions(node.Condition, guard)
			base = base.SelectAppend(flavor.As(columns.Flavor), printing.As(columns.Printing))
			matched := goqu.Or(goqu.I("candidate."+columns.Flavor).IsNotNull(), goqu.I("candidate."+columns.Printing).IsNotNull())
			return mysqlDialect.From(base.As("candidate")).Select(goqu.I("candidate.*"), goqu.L("?", matched).As(columns.Match)).Prepared(true), columns, nil
		}
		matched, err := searchCandidatePredicate(node)
		if err != nil {
			return nil, columns, err
		}
		if guard != nil {
			matched = goqu.Case().When(guard, matched).Else(false)
		}
		return base.SelectAppend(goqu.L("?", matched).As(columns.Match), goqu.L("NULL").As(columns.Flavor), goqu.L("NULL").As(columns.Printing)).Prepared(true), columns, nil
	}
	if node.Kind != "and" && node.Kind != "or" {
		return nil, columns, fmt.Errorf("unknown search expression %q", node.Kind)
	}
	left, leftColumns, err := appendSearchCandidateMatch(candidates, node.Left, guard, next)
	if err != nil {
		return nil, columns, err
	}
	rightGuard := guard
	if node.Kind == "and" {
		rightGuard = goqu.I("candidate." + leftColumns.Match).Eq(true)
	}
	right, rightColumns, err := appendSearchCandidateMatch(left, node.Right, rightGuard, next)
	if err != nil {
		return nil, columns, err
	}
	var matched exp.Expression = goqu.Or(goqu.I("candidate."+leftColumns.Match).Eq(true), goqu.I("candidate."+rightColumns.Match).Eq(true))
	if node.Kind == "and" {
		matched = goqu.And(goqu.I("candidate."+leftColumns.Match).Eq(true), goqu.I("candidate."+rightColumns.Match).Eq(true))
	}
	// Failed AND branches must not contribute a name version to a surrounding OR.
	flavor := goqu.Case().When(matched, minimumSearchVersion(goqu.I("candidate."+leftColumns.Flavor), goqu.I("candidate."+rightColumns.Flavor))).Else(goqu.L("NULL"))
	printing := goqu.Case().When(matched, minimumSearchVersion(goqu.I("candidate."+leftColumns.Printing), goqu.I("candidate."+rightColumns.Printing))).Else(goqu.L("NULL"))
	return mysqlDialect.From(right.As("candidate")).Select(goqu.I("candidate.*"), goqu.L("?", matched).As(columns.Match),
		flavor.As(columns.Flavor), printing.As(columns.Printing)).Prepared(true), columns, nil
}

func minimumSearchVersion(left, right exp.Expression) exp.LiteralExpression {
	return goqu.L("COALESCE(LEAST(?, ?), ?, ?)", left, right, left, right)
}
