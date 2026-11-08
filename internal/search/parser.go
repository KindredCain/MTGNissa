package search

import (
	"errors"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"mtgnissa/internal/model"
)

type token struct {
	kind  string
	value string
	start int
	end   int
}

type parser struct {
	tokens   []token
	position int
	depth    int
}

var decimalPattern = regexp.MustCompile(`^[+-]?(?:[0-9]+(?:\.[0-9]*)?|\.[0-9]+)$`)

var fieldAliases = map[string]string{
	"name": "name", "c": "c", "color": "c", "id": "id", "identity": "id",
	"t": "t", "type": "t", "keyword": "kw", "kw": "kw", "mv": "mv", "manavalue": "mv",
	"power": "pow", "pow": "pow", "toughness": "tou", "tou": "tou", "pt": "pt", "powtou": "pt", "loyalty": "loy", "loy": "loy",
	"is": "is", "r": "r", "rarity": "r", "s": "set", "e": "set", "set": "set", "edition": "set",
	"otag": "otag", "oracletag": "otag", "function": "otag",
}

var colorAliases = map[string]string{
	"white": "w", "blue": "u", "black": "b", "red": "r", "green": "g", "colorless": "c", "multicolor": "m",
	"azorius": "wu", "dimir": "ub", "rakdos": "br", "gruul": "rg", "selesnya": "wg", "orzhov": "wb", "izzet": "ur", "golgari": "bg", "boros": "wr", "simic": "ug",
	"bant": "wug", "esper": "wub", "grixis": "ubr", "jund": "brg", "naya": "wrg", "abzan": "wbg", "jeskai": "wur", "sultai": "ubg", "mardu": "wbr", "temur": "urg",
	"silverquill": "wb", "prismari": "ur", "witherbloom": "bg", "lorehold": "wr", "quandrix": "ug",
	"chaos": "ubrg", "aggression": "wbrg", "altruism": "wurg", "growth": "wubg", "artifice": "wubr",
}

var rarityAliases = map[string]string{"common": "common", "c": "common", "uncommon": "uncommon", "u": "uncommon", "rare": "rare", "r": "rare", "mythic": "mythic", "m": "mythic", "special": "special", "s": "special", "bonus": "bonus", "b": "bonus"}

func parseQuery(input string) model.SearchQuery {
	text := strings.TrimSpace(input)
	fallback := model.SearchQuery{Input: input, Mode: "name", ExecutedQuery: text, Expression: &model.SearchExpression{Kind: "atom", Condition: model.SearchCondition{Field: "name", Operator: ":", Value: text}}}
	tokens, special, err := lex(text)
	if err != nil || !special || len(tokens) == 0 {
		return fallback
	}
	p := parser{tokens: tokens}
	expression, err := p.parseOr()
	if err != nil || p.position != len(tokens) {
		return fallback
	}
	return model.SearchQuery{Input: input, Mode: "syntax", ExecutedQuery: render(expression, 0), Expression: expression}
}

func lex(input string) ([]token, bool, error) {
	tokens := []token{}
	special := false
	for i := 0; i < len(input); {
		r, size := utf8.DecodeRuneInString(input[i:])
		if unicode.IsSpace(r) {
			i += size
			continue
		}
		start := i
		switch input[i] {
		case '(', ')':
			tokens = append(tokens, token{kind: string(input[i]), start: i, end: i + 1})
			i++
			special = true
		case ':', '=', '!', '>', '<':
			i++
			if i < len(input) && input[i] == '=' && (input[start] == '!' || input[start] == '>' || input[start] == '<') {
				i++
			}
			op := input[start:i]
			if op == "!" {
				return nil, false, errors.New("unsupported exact-name or negation operator")
			}
			tokens = append(tokens, token{kind: "operator", value: op, start: start, end: i})
			special = true
		case '"':
			i++
			var value strings.Builder
			closed := false
			for i < len(input) {
				if input[i] == '"' {
					i++
					closed = true
					break
				}
				if input[i] == '\\' {
					i++
					if i >= len(input) || (input[i] != '\\' && input[i] != '"') {
						return nil, false, errors.New("unsupported quoted escape")
					}
				}
				value.WriteByte(input[i])
				i++
			}
			if !closed || value.Len() == 0 {
				return nil, false, errors.New("incomplete quoted value")
			}
			tokens = append(tokens, token{kind: "quoted", value: value.String(), start: start, end: i})
			special = true
		default:
			for i < len(input) {
				r, size := utf8.DecodeRuneInString(input[i:])
				if unicode.IsSpace(r) || strings.ContainsRune("():=!<>\"", r) {
					break
				}
				i += size
			}
			value := input[start:i]
			kind := "word"
			if strings.EqualFold(value, "or") {
				kind = "or"
				special = true
			}
			tokens = append(tokens, token{kind: kind, value: value, start: start, end: i})
		}
	}
	return tokens, special, nil
}

func (p *parser) parseOr() (*model.SearchExpression, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for p.position < len(p.tokens) && p.tokens[p.position].kind == "or" {
		p.position++
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = &model.SearchExpression{Kind: "or", Left: left, Right: right}
	}
	return left, nil
}

func (p *parser) parseAnd() (*model.SearchExpression, error) {
	left, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}
	for p.position < len(p.tokens) && p.tokens[p.position].kind != "or" && p.tokens[p.position].kind != ")" {
		if p.tokens[p.position-1].end == p.tokens[p.position].start {
			return nil, errors.New("missing condition separator")
		}
		right, err := p.parsePrimary()
		if err != nil {
			return nil, err
		}
		left = &model.SearchExpression{Kind: "and", Left: left, Right: right}
	}
	return left, nil
}

func (p *parser) parsePrimary() (*model.SearchExpression, error) {
	if p.position >= len(p.tokens) || p.depth > 512 {
		return nil, errors.New("incomplete expression")
	}
	current := p.tokens[p.position]
	p.position++
	if current.kind == "(" {
		p.depth++
		node, err := p.parseOr()
		p.depth--
		if err != nil || p.position >= len(p.tokens) || p.tokens[p.position].kind != ")" {
			return nil, errors.New("unclosed group")
		}
		p.position++
		return node, nil
	}
	if current.kind != "word" && current.kind != "quoted" {
		return nil, errors.New("expected condition")
	}
	condition := model.SearchCondition{Field: "name", Operator: ":", Value: current.value, Quoted: current.kind == "quoted"}
	if current.kind == "word" && p.position < len(p.tokens) && p.tokens[p.position].kind == "operator" {
		field, ok := fieldAliases[strings.ToLower(current.value)]
		if !ok {
			return nil, errors.New("unsupported field")
		}
		condition.Field = field
		condition.Operator = p.tokens[p.position].value
		p.position++
		if p.position >= len(p.tokens) || (p.tokens[p.position].kind != "word" && p.tokens[p.position].kind != "quoted") {
			return nil, errors.New("missing field value")
		}
		value := p.tokens[p.position]
		p.position++
		condition.Value = value.value
		condition.Quoted = value.kind == "quoted"
	}
	condition, ok := normalizeCondition(condition)
	if !ok {
		return nil, errors.New("unsupported condition")
	}
	return &model.SearchExpression{Kind: "atom", Condition: condition}, nil
}

func normalizeCondition(c model.SearchCondition) (model.SearchCondition, bool) {
	lower := strings.ToLower(c.Value)
	switch c.Field {
	case "c", "id":
		if !comparisonOperator(c.Operator) {
			return c, false
		}
		c.Operator = normalizeOperator(c.Operator)
		if alias, ok := colorAliases[lower]; ok {
			lower = alias
		}
		if count, err := strconv.ParseUint(lower, 10, 63); err == nil {
			c.ColorCount = &count
			c.Value = strconv.FormatUint(count, 10)
			return c, true
		}
		if lower == "m" {
			c.Multicolor = true
			c.Value = "m"
			return c, true
		}
		if lower == "c" {
			c.Value = "c"
			return c, true
		}
		mask := uint64(0)
		for _, r := range lower {
			index := strings.IndexRune("wubrg", r)
			if index < 0 || mask&(1<<index) != 0 {
				return c, false
			}
			mask |= 1 << index
		}
		if mask == 0 {
			return c, false
		}
		c.ColorMask = mask
		c.Value = ""
		for i, r := range "wubrg" {
			if mask&(1<<i) != 0 {
				c.Value += string(r)
			}
		}
		return c, true
	case "mv", "pow", "tou", "pt", "loy":
		if !comparisonOperator(c.Operator) {
			return c, false
		}
		c.Operator = normalizeOperator(c.Operator)
		if field := fieldAliases[lower]; c.Field != "mv" && (field == "pow" || field == "tou" || field == "loy") {
			c.ValueField = field
			c.Value = field
			return c, true
		}
		if !decimalPattern.MatchString(lower) {
			return c, false
		}
		number, err := strconv.ParseFloat(lower, 64)
		if err != nil || math.IsInf(number, 0) || math.IsNaN(number) || c.Field == "mv" && number < 0 {
			return c, false
		}
		c.Value = strconv.FormatFloat(number, 'f', -1, 64)
		return c, true
	case "r":
		if !comparisonOperator(c.Operator) {
			return c, false
		}
		value, ok := rarityAliases[lower]
		c.Operator = normalizeOperator(c.Operator)
		c.Value = value
		return c, ok
	case "is":
		if c.Operator != ":" {
			return c, false
		}
		if lower == "tdfc" {
			lower = "transform"
		}
		c.Value = lower
		return c, oneOf(lower, "split", "flip", "transform", "meld", "leveler", "spell", "permanent", "historic")
	case "name", "t", "kw", "set", "otag":
		if c.Operator != ":" || strings.TrimSpace(c.Value) == "" {
			return c, false
		}
		if !c.Quoted {
			for _, r := range c.Value {
				if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
					return c, false
				}
			}
			if strings.EqualFold(c.Value, "AND") || strings.EqualFold(c.Value, "NOT") {
				return c, false
			}
		}
		if c.Field != "name" && c.Field != "otag" && c.Field != "t" {
			c.Value = lower
		}
		return c, true
	default:
		return c, false
	}
}

func comparisonOperator(op string) bool {
	return oneOf(op, ":", "=", "!=", ">", ">=", "<", "<=")
}

func normalizeOperator(op string) string {
	if op == ":" {
		return "="
	}
	return op
}

func oneOf(value string, choices ...string) bool {
	for _, choice := range choices {
		if choice == value {
			return true
		}
	}
	return false
}

func quote(value string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(value) + `"`
}

func render(node *model.SearchExpression, parent int) string {
	if node.Kind == "atom" {
		c := node.Condition
		value := c.Value
		if c.Field == "otag" || c.Field == "name" || c.Quoted && (c.Field == "t" || c.Field == "kw" || c.Field == "set") {
			value = quote(value)
		}
		return c.Field + c.Operator + value
	}
	precedence, separator := 2, " "
	if node.Kind == "or" {
		precedence, separator = 1, " OR "
	}
	value := render(node.Left, precedence) + separator + render(node.Right, precedence)
	if precedence < parent {
		return "(" + value + ")"
	}
	return value
}
