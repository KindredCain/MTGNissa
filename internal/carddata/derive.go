package carddata

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var manaSymbolPattern = regexp.MustCompile(`\{([^{}]+)\}`)

func deriveRows(s spec, obj map[string]json.RawMessage, line int64) (map[string][]importRow, error) {
	switch s.table {
	case "scryfall_card":
		return deriveScryfallRows(obj, line)
	case "zhs_oracle":
		return deriveFormerNameRows(obj, line)
	default:
		return nil, nil
	}
}

func deriveScryfallRows(obj map[string]json.RawMessage, line int64) (map[string][]importRow, error) {
	cardUUID, err := stringField(obj, "uuid")
	if err != nil {
		return nil, err
	}
	result := make(map[string][]importRow)

	if err := appendStringArrayRows(result, obj, "keywords", "scryfall_card_keyword", cardUUID, line); err != nil {
		return nil, err
	}
	if err := appendStringArrayRows(result, obj, "artist_ids", "scryfall_card_artist", cardUUID, line); err != nil {
		return nil, err
	}
	if err := appendStringArrayRows(result, obj, "frame_effects", "scryfall_card_frame_effect", cardUUID, line); err != nil {
		return nil, err
	}
	if err := appendStringArrayRows(result, obj, "promo_types", "scryfall_card_promo_type", cardUUID, line); err != nil {
		return nil, err
	}

	manaCost, err := optionalStringField(obj, "mana_cost")
	if err != nil {
		return nil, err
	}
	if manaCost != nil {
		counts := make(map[string]int64)
		for _, match := range manaSymbolPattern.FindAllStringSubmatch(*manaCost, -1) {
			counts[match[1]]++
		}
		symbols := make([]string, 0, len(counts))
		for symbol := range counts {
			symbols = append(symbols, symbol)
		}
		sort.Strings(symbols)
		for _, symbol := range symbols {
			result["scryfall_card_mana_symbol"] = append(result["scryfall_card_mana_symbol"], importRow{[]any{cardUUID, symbol, counts[symbol]}, line})
		}
	}

	typeLine, err := stringField(obj, "type_line")
	if err != nil {
		return nil, err
	}
	parts := strings.SplitN(typeLine, "—", 2)
	appendTypeTerms(result, cardUUID, "main", parts[0], line)
	if len(parts) == 2 {
		appendTypeTerms(result, cardUUID, "subtype", parts[1], line)
	}

	setID, err := stringField(obj, "set_id")
	if err != nil {
		return nil, err
	}
	setCode, err := stringField(obj, "set_code")
	if err != nil {
		return nil, err
	}
	setName, err := stringField(obj, "set_name")
	if err != nil {
		return nil, err
	}
	setType, err := stringField(obj, "set_type")
	if err != nil {
		return nil, err
	}
	result["scryfall_set"] = append(result["scryfall_set"], importRow{[]any{setID, setCode, setName, setType}, line})

	return result, nil
}

func deriveFormerNameRows(obj map[string]json.RawMessage, line int64) (map[string][]importRow, error) {
	faceOracleID, err := stringField(obj, "face_oracle_id")
	if err != nil {
		return nil, err
	}
	names, err := stringArrayField(obj, "former_names")
	if err != nil {
		return nil, err
	}
	rows := make([]importRow, 0, len(names))
	for position, name := range names {
		rows = append(rows, importRow{[]any{faceOracleID, int64(position), name}, line})
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return map[string][]importRow{"zhs_oracle_former_name": rows}, nil
}

func appendStringArrayRows(result map[string][]importRow, obj map[string]json.RawMessage, field, table, cardUUID string, line int64) error {
	values, err := stringArrayField(obj, field)
	if err != nil {
		return err
	}
	for _, value := range uniqueStrings(values) {
		result[table] = append(result[table], importRow{[]any{cardUUID, value}, line})
	}
	return nil
}

func appendTypeTerms(result map[string][]importRow, cardUUID, group, segment string, line int64) {
	for _, term := range uniqueStrings(strings.Fields(strings.TrimSpace(segment))) {
		result["scryfall_card_type"] = append(result["scryfall_card_type"], importRow{[]any{cardUUID, group, term}, line})
	}
}

func stringField(obj map[string]json.RawMessage, field string) (string, error) {
	value, err := optionalStringField(obj, field)
	if err != nil {
		return "", err
	}
	if value == nil {
		return "", fmt.Errorf("field %q is missing or null", field)
	}
	return *value, nil
}

func optionalStringField(obj map[string]json.RawMessage, field string) (*string, error) {
	raw, ok := obj[field]
	if !ok || string(raw) == "null" {
		return nil, nil
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, fmt.Errorf("field %q: %w", field, err)
	}
	return &value, nil
}

func stringArrayField(obj map[string]json.RawMessage, field string) ([]string, error) {
	raw, ok := obj[field]
	if !ok || string(raw) == "null" {
		return nil, nil
	}
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, fmt.Errorf("field %q: %w", field, err)
	}
	return values, nil
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
