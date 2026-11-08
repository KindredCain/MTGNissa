package model

// SearchParams contains the global query text and optional continuation cursor.
type SearchParams struct {
	Query  string
	Cursor string
}

// SearchCondition is one validated atomic predicate evaluated at Oracle set scope.
type SearchCondition struct {
	Field      string
	Operator   string
	Value      string
	ValueField string
	Quoted     bool
	ColorMask  uint64
	ColorCount *uint64
	Multicolor bool
}

// SearchExpression combines atomic Oracle sets with conjunction or disjunction.
type SearchExpression struct {
	Kind      string
	Condition SearchCondition
	Left      *SearchExpression
	Right     *SearchExpression
}

// SearchQuery records whether the complete input was parsed or treated as a literal name.
type SearchQuery struct {
	Input         string
	Mode          string
	ExecutedQuery string
	Expression    *SearchExpression
}

// SearchPage contains representative printings for a global Oracle search batch.
type SearchPage struct {
	Input         string
	Mode          string
	ExecutedQuery string
	Items         []CardSummary
	Total         int64
	NextCursor    *string
}
