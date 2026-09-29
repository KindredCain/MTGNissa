package carddata

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

func deriveRows(s sourceSpec, obj map[string]json.RawMessage, line int64) (map[tableName][]importRow, error) {
	switch s.table {
	case tableScryfallCard:
		return deriveScryfallRows(obj, line)
	case tableScryfallOracleRuling:
		return deriveRulingRows(obj, line)
	case tableOracleTag:
		return deriveOracleTagRows(obj, line)
	default:
		return nil, nil
	}
}

func deriveOracleTagRows(obj map[string]json.RawMessage, line int64) (map[tableName][]importRow, error) {
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

	result := make(map[tableName][]importRow)
	seenRelations := make(map[string]struct{}, len(parents)+len(children))
	appendRelation := func(parentID, childID string) {
		key := parentID + "\x1f" + childID
		if _, exists := seenRelations[key]; exists {
			return
		}
		seenRelations[key] = struct{}{}
		result[tableOracleTagRelation] = append(result[tableOracleTagRelation], importRow{[]any{parentID, childID}, line})
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
		result[tableOracleTagging] = append(result[tableOracleTagging], importRow{[]any{tagging.OracleID, tagID, tagging.Weight}, line})
	}
	return result, nil
}

func deriveRulingRows(obj map[string]json.RawMessage, line int64) (map[tableName][]importRow, error) {
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
	return map[tableName][]importRow{tableRulingTranslation: {row}}, nil
}

func deriveScryfallRows(obj map[string]json.RawMessage, line int64) (map[tableName][]importRow, error) {
	cardUUID, err := stringField(obj, "uuid")
	if err != nil {
		return nil, err
	}
	result := make(map[tableName][]importRow)

	if err := appendStringArrayRows(result, obj, "keywords", tableScryfallCardKeyword, cardUUID, line); err != nil {
		return nil, err
	}
	if err := appendStringArrayRows(result, obj, "frame_effects", tableScryfallCardFrameEffect, cardUUID, line); err != nil {
		return nil, err
	}
	if err := appendStringArrayRows(result, obj, "promo_types", tableScryfallCardPromoType, cardUUID, line); err != nil {
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
	result[tableScryfallSet] = append(result[tableScryfallSet], importRow{[]any{setID, setCode, setName, setType}, line})

	return result, nil
}

func appendStringArrayRows(result map[tableName][]importRow, obj map[string]json.RawMessage, field string, table tableName, cardUUID string, line int64) error {
	values, err := stringArrayField(obj, field)
	if err != nil {
		return err
	}
	for _, value := range uniqueStrings(values) {
		result[table] = append(result[table], importRow{[]any{cardUUID, value}, line})
	}
	return nil
}

func appendTypeTerms(result map[tableName][]importRow, cardUUID, group, segment string, line int64) {
	for _, term := range uniqueStrings(strings.Fields(strings.TrimSpace(segment))) {
		result[tableScryfallCardType] = append(result[tableScryfallCardType], importRow{[]any{cardUUID, group, term}, line})
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

func rowValues(obj map[string]json.RawMessage, input sourceSpec) ([]any, error) {
	tableSpec := mustTableSpec(input.table)
	return rowValuesFor(obj, tableSpec, input)
}

func rowValuesFor(obj map[string]json.RawMessage, tableSpec spec, input sourceSpec) ([]any, error) {
	known := make(map[string]struct{}, len(tableSpec.columns)+len(input.extraFields))
	values := make([]any, len(tableSpec.columns))
	for i, column := range tableSpec.columns {
		if column.kind == kindAutoIncrement {
			values[i] = nil
			continue
		}
		source := column.name
		if mapped := input.columnSources[column.name]; mapped != "" {
			source = mapped
		}
		known[source] = struct{}{}
		raw, ok := obj[source]
		if !ok || string(raw) == "null" {
			if zeroOnNull(column.kind) {
				values[i] = int64(0)
				continue
			}
			if strings.Contains(column.ddl, "NOT NULL") {
				return nil, fmt.Errorf("required relational column %q (source %q) is missing or null", column.name, source)
			}
			values[i] = nil
			continue
		}
		value, err := columnValue(raw, column)
		if err != nil {
			return nil, fmt.Errorf("field %q: %w", column.name, err)
		}
		values[i] = value
	}
	for _, name := range input.extraFields {
		known[name] = struct{}{}
	}
	for name := range obj {
		if _, ok := known[name]; !ok {
			return nil, fmt.Errorf("field %q has no relational column", name)
		}
	}
	return values, nil
}

func columnValue(raw json.RawMessage, column columnSpec) (any, error) {
	switch column.kind {
	case kindString:
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, err
		}
		return value, nil
	case kindInteger:
		return strconv.ParseInt(string(raw), 10, 64)
	case kindNumber:
		if _, err := strconv.ParseFloat(string(raw), 64); err != nil {
			return nil, err
		}
		return string(raw), nil
	case kindBoolean:
		return strconv.ParseBool(string(raw))
	case kindDate:
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, err
		}
		if _, err := time.Parse("2006-01-02", value); err != nil {
			return nil, err
		}
		return value, nil
	case kindDateTime:
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, err
		}
		parsed, err := time.Parse(time.RFC3339Nano, value)
		if err != nil {
			return nil, err
		}
		return parsed.UTC().Format("2006-01-02 15:04:05.999999"), nil
	case kindColorMask:
		return stringSetMask(raw, colorBits)
	case kindFinishMask:
		return stringSetMask(raw, finishBits)
	case kindGameMask:
		return stringSetMask(raw, gameBits)
	case kindAttractionLightMask:
		var lights []int64
		if err := json.Unmarshal(raw, &lights); err != nil {
			return nil, err
		}
		var mask int64
		for _, light := range lights {
			if light < 1 || light > 6 {
				return nil, fmt.Errorf("unsupported attraction light %d", light)
			}
			mask |= 1 << (light - 1)
		}
		return mask, nil
	case kindStringArray:
		var values []string
		if err := json.Unmarshal(raw, &values); err != nil {
			return nil, err
		}
		return strings.Join(values, ","), nil
	case kindRawJSONText:
		if !json.Valid(raw) {
			return nil, fmt.Errorf("invalid JSON value")
		}
		var compact bytes.Buffer
		if err := json.Compact(&compact, raw); err != nil {
			return nil, err
		}
		return compact.String(), nil
	case kindRulingKey:
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, err
		}
		return rulingKey(value), nil
	case kindAutoIncrement:
		return nil, nil
	default:
		return nil, fmt.Errorf("unsupported column kind %d", column.kind)
	}
}

func rulingKey(comment string) string {
	normalized := normalizeRulingComment(comment)
	return fmt.Sprintf("%x", sha256.Sum256([]byte(normalized)))
}

func normalizeRulingComment(comment string) string {
	comment = strings.ReplaceAll(comment, "\r\n", "\n")
	comment = strings.ReplaceAll(comment, "\r", "\n")
	replacer := strings.NewReplacer(
		"’", "'", "‘", "'",
		"“", "\"", "”", "\"",
		"\u00a0", " ",
	)
	return strings.TrimSpace(replacer.Replace(comment))
}

func zeroOnNull(kind columnKind) bool {
	switch kind {
	case kindColorMask, kindFinishMask, kindGameMask, kindAttractionLightMask:
		return true
	default:
		return false
	}
}

func stringSetMask(raw json.RawMessage, bits map[string]int64) (int64, error) {
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil {
		return 0, err
	}
	var mask int64
	for _, value := range values {
		bit, ok := bits[value]
		if !ok {
			return 0, fmt.Errorf("unsupported dictionary value %q", value)
		}
		mask |= bit
	}
	return mask, nil
}
