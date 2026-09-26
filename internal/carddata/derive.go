package carddata

import (
	"encoding/json"
	"fmt"
	"strings"
)

func deriveRows(s spec, obj map[string]json.RawMessage, line int64) (map[string][]importRow, error) {
	switch s.table {
	case "scryfall_card":
		return deriveScryfallRows(obj, line)
	case "scryfall_oracle_ruling":
		return deriveRulingRows(obj, line)
	case "oracle_tag":
		return deriveOracleTagRows(obj, line)
	default:
		return nil, nil
	}
}

func deriveOracleTagRows(obj map[string]json.RawMessage, line int64) (map[string][]importRow, error) {
	objectType, err := stringField(obj, "object")
	if err != nil {
		return nil, err
	}
	if objectType != "tag" {
		return nil, fmt.Errorf("field %q: unsupported object type %q", "object", objectType)
	}
	tagType, err := stringField(obj, "type")
	if err != nil {
		return nil, err
	}
	if tagType != "oracle" {
		return nil, fmt.Errorf("field %q: expected oracle tag, got %q", "type", tagType)
	}
	tagID, err := stringField(obj, "id")
	if err != nil {
		return nil, err
	}
	parents, err := stringArrayField(obj, "parent_ids")
	if err != nil {
		return nil, err
	}
	children, err := stringArrayField(obj, "child_ids")
	if err != nil {
		return nil, err
	}

	result := make(map[string][]importRow)
	seenRelations := make(map[string]struct{}, len(parents)+len(children))
	appendRelation := func(parentID, childID string) {
		key := parentID + "\x1f" + childID
		if _, exists := seenRelations[key]; exists {
			return
		}
		seenRelations[key] = struct{}{}
		result["oracle_tag_relation"] = append(result["oracle_tag_relation"], importRow{[]any{parentID, childID}, line})
	}
	for _, parentID := range parents {
		appendRelation(parentID, tagID)
	}
	for _, childID := range children {
		appendRelation(tagID, childID)
	}

	var taggings []struct {
		OracleID string `json:"oracle_id"`
		Weight   string `json:"weight"`
	}
	if err := json.Unmarshal(obj["taggings"], &taggings); err != nil {
		return nil, fmt.Errorf("field %q: %w", "taggings", err)
	}
	seenTaggings := make(map[string]string, len(taggings))
	for _, tagging := range taggings {
		if tagging.OracleID == "" || tagging.Weight == "" {
			return nil, fmt.Errorf("field %q: oracle_id and weight must be non-empty", "taggings")
		}
		if previous, exists := seenTaggings[tagging.OracleID]; exists {
			if previous != tagging.Weight {
				return nil, fmt.Errorf("field %q: oracle_id %q has conflicting weights %q and %q", "taggings", tagging.OracleID, previous, tagging.Weight)
			}
			continue
		}
		seenTaggings[tagging.OracleID] = tagging.Weight
		result["oracle_tagging"] = append(result["oracle_tagging"], importRow{[]any{tagging.OracleID, tagID, tagging.Weight}, line})
	}
	return result, nil
}

func deriveRulingRows(obj map[string]json.RawMessage, line int64) (map[string][]importRow, error) {
	objectType, err := stringField(obj, "object")
	if err != nil {
		return nil, err
	}
	if objectType != "ruling" {
		return nil, fmt.Errorf("field %q: unsupported object type %q", "object", objectType)
	}
	comment, err := stringField(obj, "comment")
	if err != nil {
		return nil, err
	}
	publishedAt, err := stringField(obj, "published_at")
	if err != nil {
		return nil, err
	}
	row := importRow{values: []any{
		rulingKey(comment),
		nil,
		comment,
		nil,
		nil,
		nil,
		publishedAt,
		nil,
	}, line: line}
	return map[string][]importRow{"zhs_ruling": {row}}, nil
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
	if err := appendStringArrayRows(result, obj, "frame_effects", "scryfall_card_frame_effect", cardUUID, line); err != nil {
		return nil, err
	}
	if err := appendStringArrayRows(result, obj, "promo_types", "scryfall_card_promo_type", cardUUID, line); err != nil {
		return nil, err
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
	seen := make([]string, 0, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		duplicate := false
		for _, previous := range seen {
			if strings.EqualFold(previous, value) {
				duplicate = true
				break
			}
		}
		if duplicate {
			continue
		}
		seen = append(seen, value)
		result = append(result, value)
	}
	return result
}
