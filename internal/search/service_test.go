package search

import (
	"context"
	"strings"
	"testing"

	"mtgnissa/internal/model"
	"mtgnissa/internal/store"
)

type storeStub struct {
	query    model.SearchQuery
	position *store.SearchPosition
	items    []model.CardSummary
	more     bool
}

func (s *storeStub) Search(_ context.Context, query model.SearchQuery, position *store.SearchPosition, _ int) ([]model.CardSummary, int64, bool, error) {
	s.query, s.position = query, position
	return s.items, int64(len(s.items)), s.more, nil
}

func TestAllSearchFieldAliasesAndComparisons(t *testing.T) {
	for _, test := range []struct {
		Aliases string
		Value   string
		Field   string
	}{
		{"name", "nissa", "name"}, {"c color", "r", "c"}, {"id identity", "esper", "id"},
		{"t type", "legend", "t"}, {"kw keyword", "flying", "kw"}, {"mv manavalue", "2.5", "mv"},
		{"pow power", "-1.5", "pow"}, {"tou toughness", "2", "tou"}, {"pt powtou", "10", "pt"}, {"loy loyalty", "3", "loy"},
		{"r rarity", "rare", "r"}, {"s e set edition", "KLD", "set"}, {"otag oracletag function", "tutor", "otag"}, {"is", "tdfc", "is"},
	} {
		for _, alias := range strings.Fields(test.Aliases) {
			query := parseQuery(strings.ToUpper(alias) + ":" + test.Value)
			if query.Mode != "syntax" || query.Expression.Condition.Field != test.Field {
				t.Errorf("alias %s parsed as %#v", alias, query)
			}
		}
	}
	for _, field := range []string{"c", "id", "mv", "pow", "tou", "pt", "loy", "r"} {
		for _, operator := range []string{":", "=", "!=", ">", ">=", "<", "<="} {
			value := "2.5"
			if field == "c" || field == "id" {
				value = "wu"
			}
			if field == "r" {
				value = "rare"
			}
			if query := parseQuery(field + operator + value); query.Mode != "syntax" {
				t.Errorf("comparison fell back: %s%s%s", field, operator, value)
			}
		}
	}
	for _, left := range []string{"power", "pow", "toughness", "tou", "pt", "powtou", "loyalty", "loy"} {
		for _, right := range []string{"power", "pow", "toughness", "tou", "loyalty", "loy"} {
			if query := parseQuery(left + ">" + right); query.Mode != "syntax" || query.Expression.Condition.ValueField == "" {
				t.Errorf("same-face comparison fell back: %s>%s", left, right)
			}
		}
	}
}

func TestSearchColorAliasesCountsRaritiesAndIsValues(t *testing.T) {
	groups := map[string]string{
		"azorius": "wu", "dimir": "ub", "rakdos": "br", "gruul": "rg", "selesnya": "gw", "orzhov": "wb", "izzet": "ur", "golgari": "bg", "boros": "rw", "simic": "gu",
		"bant": "gwu", "esper": "wub", "grixis": "ubr", "jund": "brg", "naya": "rgw", "abzan": "wbg", "jeskai": "urw", "sultai": "bgu", "mardu": "rwb", "temur": "gur",
		"silverquill": "wb", "prismari": "ur", "witherbloom": "bg", "lorehold": "rw", "quandrix": "gu",
		"chaos": "ubrg", "aggression": "wbrg", "altruism": "wurg", "growth": "wubg", "artifice": "wubr",
		"white": "w", "blue": "u", "black": "b", "red": "r", "green": "g",
	}
	for alias, letters := range groups {
		query := parseQuery("identity<=" + alias)
		mask := uint64(0)
		for _, letter := range letters {
			mask |= 1 << strings.IndexRune("wubrg", letter)
		}
		if query.Mode != "syntax" || query.Expression.Condition.ColorMask != mask {
			t.Errorf("color %s=%#v, want mask %d", alias, query.Expression.Condition, mask)
		}
	}
	for _, value := range []string{"c", "colorless", "m", "multicolor", "0", "2", "6", "wubrg", "gr"} {
		if parseQuery("c:"+value).Mode != "syntax" {
			t.Errorf("color value %s fell back", value)
		}
	}
	for _, value := range []string{"common", "c", "uncommon", "u", "rare", "r", "mythic", "m", "special", "s", "bonus", "b"} {
		if parseQuery("r:"+value).Mode != "syntax" {
			t.Errorf("rarity %s fell back", value)
		}
	}
	for _, value := range []string{"split", "flip", "transform", "tdfc", "meld", "leveler", "spell", "permanent", "historic"} {
		if parseQuery("is:"+value).Mode != "syntax" {
			t.Errorf("is:%s fell back", value)
		}
	}
}

func TestSearchBooleanPrecedenceQuotedValuesAndFullFallback(t *testing.T) {
	query := parseQuery(`t:angel OR t:demon id<=esper`)
	if query.Mode != "syntax" || query.Expression.Kind != "or" || query.Expression.Right.Kind != "and" {
		t.Fatalf("implicit AND precedence=%#v", query)
	}
	query = parseQuery(`(t:angel OR t:demon) id<=esper`)
	if query.Mode != "syntax" || query.Expression.Kind != "and" || query.Expression.Left.Kind != "or" {
		t.Fatalf("grouping=%#v", query)
	}
	for _, raw := range []string{`c:r t:instant mv<=2`, `pow>tou`, `r:rare set:khm`, `name:"lightning bolt"`, `type:"legendary creature"`, `kw:"first strike"`, `set:"Kaladesh Remastered"`, `"lightning bolt" OR name:nissa`} {
		if parseQuery(raw).Mode != "syntax" {
			t.Errorf("supported syntax fell back: %s", raw)
		}
	}
	query = parseQuery(`function:"a \"quoted\" \\ tag"`)
	if query.Mode != "syntax" || query.Expression.Condition.Value != `a "quoted" \ tag` || query.ExecutedQuery != `otag:"a \"quoted\" \\ tag"` {
		t.Fatalf("quoted tag=%#v", query)
	}
	for _, raw := range []string{
		"lightning bolt", "Nissa, Vital Force", `fo:damage`, `o:damage`, `oracle:draw`, `cmc>=5`, `lang:ja`, `fo:/draw.+card/`, `order:name`, `!Nissa`,
		`c:r fo:damage`, `t:creature OR unsupported:value`, `t:/legendary/`, `name:"unterminated`, `name:"bad\nvalue"`, `(c:r`, `c:r)`, `c:r OR`, `OR c:r`, `()`,
		`c:r AND t:instant`, `NOT t:creature`, `-t:creature`, `c:ww`, `id:rainbow`, `r:gold`, `mv:-1`, `pow:*`, `pow>pt`, `c==r`, `c:`, `c:2.5`, `t=creature`,
		`name:"foo"bar`, `otag:draw-engine`,
	} {
		query := parseQuery(raw)
		if query.Mode != "name" || query.ExecutedQuery != raw || query.Expression.Condition.Value != raw {
			t.Errorf("query was partially executed instead of falling back: %s -> %#v", raw, query)
		}
	}
}

func TestSearchServiceKeepsZeroResultsInSyntaxModeAndBindsCursor(t *testing.T) {
	stub := &storeStub{}
	service := &Service{store: stub, pageSize: 2}
	page, err := service.List(context.Background(), model.SearchParams{Query: "c:r"})
	if err != nil || page.Mode != "syntax" || page.Items == nil || page.Total != 0 || page.NextCursor != nil {
		t.Fatalf("zero result=%#v error=%v", page, err)
	}
	_, err = service.List(context.Background(), model.SearchParams{Query: "c:r lang:ja"})
	if err != nil || stub.query.Mode != "name" || stub.query.Expression.Condition.Value != "c:r lang:ja" {
		t.Fatalf("fallback=%#v error=%v", stub.query, err)
	}
	oracle := "00000000-0000-4000-8000-000000000001"
	stub.items = []model.CardSummary{{Name: "Alpha", OracleID: &oracle}}
	stub.more = true
	page, err = service.List(context.Background(), model.SearchParams{Query: "c:r"})
	if err != nil || page.NextCursor == nil {
		t.Fatalf("first page=%#v error=%v", page, err)
	}
	stub.more = false
	_, err = service.List(context.Background(), model.SearchParams{Query: "c:r", Cursor: *page.NextCursor})
	if err != nil || stub.position == nil || stub.position.Name != "alpha" || stub.position.OracleID != oracle {
		t.Fatalf("cursor boundary=%#v error=%v", stub.position, err)
	}
	if _, err := service.List(context.Background(), model.SearchParams{Query: "c:g", Cursor: *page.NextCursor}); err == nil {
		t.Fatal("cursor accepted changed query")
	}
}

func TestCanonicalSearchQueriesRemainParseable(t *testing.T) {
	for _, input := range []string{
		`c:r t:instant mv<=2`, `(t:angel OR t:demon) id<=esper`, `power>toughness`, `r:rare edition:khm`,
		`function:tutor`, `function:"draw engine"`, `name:"a \"quoted\" \\ name"`, `type:"legendary creature"`,
		`kw:"first strike"`, `set:"Kaladesh Remastered"`, `"lightning bolt" OR nissa`,
	} {
		first := parseQuery(input)
		second := parseQuery(first.ExecutedQuery)
		if first.Mode != "syntax" || second.Mode != "syntax" || first.ExecutedQuery != second.ExecutedQuery {
			t.Errorf("canonical query changed semantics: %q -> %q -> %q", input, first.ExecutedQuery, second.ExecutedQuery)
		}
	}
}
