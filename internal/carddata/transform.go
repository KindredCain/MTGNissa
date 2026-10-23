package carddata

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var decimalNumberPattern = regexp.MustCompile(`^[+-]?(?:\d+(?:\.\d*)?|\.\d+)$`)

func deriveRows(s sourceSpec, obj map[string]json.RawMessage, line int64) (map[tableName][]importRow, error) {
	switch s.table {
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
		if column.kind == kindDerivedNumber {
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
	populateDerivedNumbers(obj, tableSpec, values)
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

func populateDerivedNumbers(obj map[string]json.RawMessage, tableSpec spec, values []any) {
	power, hasPower := decimalString(obj, "power")
	toughness, hasToughness := decimalString(obj, "toughness")
	loyalty, hasLoyalty := decimalString(obj, "loyalty")
	for i, column := range tableSpec.columns {
		if column.kind != kindDerivedNumber {
			continue
		}
		switch column.name {
		case "power_numeric":
			if hasPower {
				values[i] = power
			}
		case "toughness_numeric":
			if hasToughness {
				values[i] = toughness
			}
		case "loyalty_numeric":
			if hasLoyalty {
				values[i] = loyalty
			}
		case "pt_numeric":
			if hasPower && hasToughness {
				values[i] = addDecimalStrings(power, toughness)
			}
		}
	}
}

func decimalString(obj map[string]json.RawMessage, field string) (string, bool) {
	raw, ok := obj[field]
	if !ok || string(raw) == "null" {
		return "", false
	}
	var value string
	if json.Unmarshal(raw, &value) != nil {
		return "", false
	}
	value = strings.TrimSpace(value)
	return value, decimalNumberPattern.MatchString(value)
}

func addDecimalStrings(left, right string) string {
	leftValue, _ := new(big.Rat).SetString(strings.TrimPrefix(left, "+"))
	rightValue, _ := new(big.Rat).SetString(strings.TrimPrefix(right, "+"))
	leftRounded, _ := new(big.Rat).SetString(leftValue.FloatString(4))
	rightRounded, _ := new(big.Rat).SetString(rightValue.FloatString(4))
	return new(big.Rat).Add(leftRounded, rightRounded).FloatString(4)
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

var legalityFormats = []string{
	"alchemy", "brawl", "commander", "duel", "future", "gladiator", "historic",
	"competitivebrawl", "legacy", "modern", "oathbreaker", "oldschool", "pauper",
	"paupercommander", "penny", "pioneer", "predh", "premodern", "standard",
	"standardbrawl", "timeless", "vintage",
}

var legalityPriority = map[string]int{
	"Not Legal":  1,
	"Banned":     2,
	"Restricted": 3,
	"Legal":      4,
}

func decodeObject(raw json.RawMessage) (map[string]json.RawMessage, error) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, err
	}
	if obj == nil {
		return nil, fmt.Errorf("JSON value must be an object")
	}
	return obj, nil
}

func mtgjsonCardRows(raw json.RawMessage, set mtgjsonSet, line int64, state *importState) (map[tableName][]importRow, error) {
	obj, err := decodeObject(raw)
	if err != nil {
		return nil, err
	}
	uuid, err := stringField(obj, "uuid")
	if err != nil {
		return nil, err
	}
	identifiers, err := objectField(obj, "identifiers")
	if err != nil {
		return nil, err
	}
	scryfallID, err := stringField(identifiers, "scryfallId")
	if err != nil {
		return nil, fmt.Errorf("field %q: %w", "identifiers.scryfallId", err)
	}
	oracleID, err := optionalStringField(identifiers, "scryfallOracleId")
	if err != nil {
		return nil, err
	}
	index, err := faceIndex(obj)
	if err != nil {
		return nil, err
	}
	values, err := mtgjsonCardValues(obj, set, uuid, scryfallID, oracleID, index)
	if err != nil {
		return nil, err
	}
	result := map[tableName][]importRow{
		tableScryfallCard: {{values: values, line: line}},
	}
	printingRows, err := cardPrintingRows(obj, uuid, scryfallID, line)
	if err != nil {
		return nil, err
	}
	result[tableMTGNissaCardPrinting] = printingRows
	if err := appendMTGJSONArrayRows(result, obj, "keywords", tableScryfallCardKeyword, uuid, line); err != nil {
		return nil, err
	}
	if err := appendMTGJSONArrayRows(result, obj, "frameEffects", tableScryfallCardFrameEffect, uuid, line); err != nil {
		return nil, err
	}
	if err := appendMTGJSONArrayRows(result, obj, "promoTypes", tableScryfallCardPromoType, uuid, line); err != nil {
		return nil, err
	}
	for _, item := range []struct {
		field string
		group string
	}{{"supertypes", "supertype"}, {"types", "type"}, {"subtypes", "subtype"}} {
		terms, err := stringArrayField(obj, item.field)
		if err != nil {
			return nil, err
		}
		for _, term := range uniqueStrings(terms) {
			result[tableScryfallCardType] = append(result[tableScryfallCardType], importRow{values: []any{uuid, item.group, term}, line: line})
		}
	}
	if oracleID != nil {
		if err := collectLegality(state, obj, *oracleID); err != nil {
			return nil, err
		}
		rulings, err := rulingRows(obj, *oracleID, line, state)
		if err != nil {
			return nil, err
		}
		for table, rows := range rulings {
			result[table] = append(result[table], rows...)
		}
	}
	return result, nil
}

func mtgjsonCardValues(obj map[string]json.RawMessage, set mtgjsonSet, uuid, scryfallID string, oracleID *string, index int64) ([]any, error) {
	values := map[string]any{
		"uuid": uuid, "scryfall_id": scryfallID, "face_index": index, "oracle_id": pointerValue(oracleID),
		"released_at": pointerValue(set.ReleaseDate),
	}
	fieldMap := map[string]string{
		"layout": "layout", "colors_mask": "colors", "color_identity_mask": "colorIdentity",
		"color_indicator_mask": "colorIndicator", "produced_mana": "producedMana", "defense": "defense",
		"game_changer": "isGameChanger", "hand_modifier": "hand", "life_modifier": "life",
		"loyalty": "loyalty", "name": "name", "face_name": "faceName", "oracle_text": "text",
		"power": "power", "reserved": "isReserved", "toughness": "toughness", "type_line": "type",
		"artist": "artist", "artist_ids": "artistIds", "border_color": "borderColor",
		"collector_number": "number", "content_warning": "hasContentWarning", "digital": "isOnlineOnly",
		"finishes_mask": "finishes", "flavor_text": "flavorText", "frame": "frameVersion",
		"full_art": "isFullArt", "games_mask": "availability", "oversized": "isOversized",
		"promo": "isPromo", "rarity": "rarity", "reprint": "isReprint", "set_code": "setCode",
		"story_spotlight": "isStorySpotlight", "textless": "isTextless", "variation": "isAlternative",
		"variations": "variations", "security_stamp": "securityStamp", "watermark": "watermark",
		"attraction_lights_mask": "attractionLights", "mana_cost": "manaCost",
		"has_alternative_deck_limit": "hasAlternativeDeckLimit",
	}
	if _, exists := obj["faceManaValue"]; exists {
		fieldMap["cmc"] = "faceManaValue"
	} else {
		fieldMap["cmc"] = "manaValue"
	}
	for column, field := range fieldMap {
		value, err := mtgjsonColumnValue(obj, field, columnSpecByName(mustTableSpec(tableScryfallCard), column))
		if err != nil {
			return nil, fmt.Errorf("field %q: %w", field, err)
		}
		values[column] = value
	}
	flavorName, err := optionalStringField(obj, "faceFlavorName")
	if err != nil {
		return nil, err
	}
	if flavorName == nil {
		flavorName, err = optionalStringField(obj, "flavorName")
		if err != nil {
			return nil, err
		}
	}
	values["flavor_name"] = pointerValue(flavorName)
	boosterTypes, err := stringArrayField(obj, "boosterTypes")
	if err != nil {
		return nil, err
	}
	finishes, err := stringArrayField(obj, "finishes")
	if err != nil {
		return nil, err
	}
	values["booster"] = len(boosterTypes) > 0
	values["foil"] = containsString(finishes, "foil")
	values["nonfoil"] = containsString(finishes, "nonfoil")
	leaders, err := optionalObjectField(obj, "leadershipSkills")
	if err != nil {
		return nil, err
	}
	values["brawl_leader"] = booleanValue(leaders, "brawl")
	values["commander_leader"] = booleanValue(leaders, "commander")
	values["oathbreaker_leader"] = booleanValue(leaders, "oathbreaker")
	populateNamedDerivedNumbers(obj, values)
	return orderedValues(mustTableSpec(tableScryfallCard), values)
}

func mtgjsonTokenRows(raw json.RawMessage, tokenCode string, line int64) (map[tableName][]importRow, error) {
	obj, err := decodeObject(raw)
	if err != nil {
		return nil, err
	}
	values := make(map[string]any)
	for column, field := range map[string]string{
		"uuid": "uuid", "layout": "layout", "colors_mask": "colors", "color_identity_mask": "colorIdentity",
		"produced_mana": "producedMana", "name": "name", "face_name": "faceName", "oracle_text": "text",
		"power": "power", "toughness": "toughness", "type_line": "type", "artist": "artist",
		"artist_ids": "artistIds", "border_color": "borderColor", "collector_number": "number",
		"finishes_mask": "finishes", "flavor_name": "flavorName", "flavor_text": "flavorText",
		"frame": "frameVersion", "full_art": "isFullArt", "games_mask": "availability",
		"oversized": "isOversized", "promo": "isPromo", "reprint": "isReprint", "textless": "isTextless",
		"security_stamp": "securityStamp", "watermark": "watermark", "mana_cost": "manaCost",
	} {
		value, err := mtgjsonColumnValue(obj, field, columnSpecByName(mustTableSpec(tableMTGNissaToken), column))
		if err != nil {
			return nil, fmt.Errorf("field %q: %w", field, err)
		}
		values[column] = value
	}
	index, err := faceIndex(obj)
	if err != nil {
		return nil, err
	}
	values["face_index"] = index
	values["set_code"] = tokenCode
	identifiers, err := optionalObjectField(obj, "identifiers")
	if err != nil {
		return nil, err
	}
	scryfallID, err := optionalStringField(identifiers, "scryfallId")
	if err != nil {
		return nil, err
	}
	oracleID, err := optionalStringField(identifiers, "scryfallOracleId")
	if err != nil {
		return nil, err
	}
	values["scryfall_id"] = pointerValue(scryfallID)
	values["oracle_id"] = pointerValue(oracleID)
	boosterTypes, err := stringArrayField(obj, "boosterTypes")
	if err != nil {
		return nil, err
	}
	finishes, err := stringArrayField(obj, "finishes")
	if err != nil {
		return nil, err
	}
	values["booster"] = len(boosterTypes) > 0
	values["foil"] = containsString(finishes, "foil")
	values["nonfoil"] = containsString(finishes, "nonfoil")
	populateNamedDerivedNumbers(obj, values)
	row, err := orderedValues(mustTableSpec(tableMTGNissaToken), values)
	if err != nil {
		return nil, err
	}
	return map[tableName][]importRow{tableMTGNissaToken: {{values: row, line: line}}}, nil
}

func cardPrintingRows(obj map[string]json.RawMessage, cardUUID, baseScryfallID string, line int64) ([]importRow, error) {
	outerLanguage, err := stringField(obj, "language")
	if err != nil {
		return nil, err
	}
	outerCode, err := languageCode(outerLanguage)
	if err != nil {
		return nil, err
	}
	var foreign []map[string]json.RawMessage
	if raw, ok := obj["foreignData"]; ok && string(raw) != "null" {
		if err := json.Unmarshal(raw, &foreign); err != nil {
			return nil, fmt.Errorf("field %q: %w", "foreignData", err)
		}
	}
	rows := make([]importRow, 0, len(foreign)+1)
	seen := make(map[string]struct{}, len(foreign)+1)
	foreignLanguages := make(map[string]struct{}, len(foreign))
	for _, item := range foreign {
		language, err := stringField(item, "language")
		if err != nil {
			return nil, fmt.Errorf("field %q: %w", "foreignData.language", err)
		}
		code, err := languageCode(language)
		if err != nil {
			return nil, err
		}
		if _, duplicate := foreignLanguages[code]; duplicate {
			return nil, fmt.Errorf("foreignData contains duplicate normalized language %q", code)
		}
		foreignLanguages[code] = struct{}{}
		uuid, err := stringField(item, "uuid")
		if err != nil {
			return nil, err
		}
		if code == outerCode {
			uuid = cardUUID
		}
		identifiers, err := optionalObjectField(item, "identifiers")
		if err != nil {
			return nil, err
		}
		scryfallID, err := optionalStringField(identifiers, "scryfallId")
		if err != nil {
			return nil, err
		}
		name, err := optionalStringField(item, "name")
		if err != nil {
			return nil, err
		}
		text, err := optionalStringField(item, "text")
		if err != nil {
			return nil, err
		}
		typeLine, err := optionalStringField(item, "type")
		if err != nil {
			return nil, err
		}
		if name == nil && text == nil && typeLine == nil {
			continue
		}
		seen[code] = struct{}{}
		rows = append(rows, importRow{values: []any{uuid, cardUUID, code, pointerValue(scryfallID), pointerValue(name), pointerValue(text), pointerValue(typeLine)}, line: line})
	}
	if _, exists := seen[outerCode]; !exists {
		name, err := fallbackString(obj, "printedName", "name")
		if err != nil {
			return nil, err
		}
		text, err := fallbackString(obj, "printedText", "text")
		if err != nil {
			return nil, err
		}
		typeLine, err := fallbackString(obj, "printedType", "type")
		if err != nil {
			return nil, err
		}
		rows = append(rows, importRow{values: []any{cardUUID, cardUUID, outerCode, baseScryfallID, pointerValue(name), pointerValue(text), pointerValue(typeLine)}, line: line})
	}
	return rows, nil
}

func rulingRows(obj map[string]json.RawMessage, oracleID string, line int64, state *importState) (map[tableName][]importRow, error) {
	var rulings []struct {
		Date string `json:"date"`
		Text string `json:"text"`
	}
	if raw, ok := obj["rulings"]; ok && string(raw) != "null" {
		if err := json.Unmarshal(raw, &rulings); err != nil {
			return nil, fmt.Errorf("field %q: %w", "rulings", err)
		}
	}
	result := make(map[tableName][]importRow)
	for _, ruling := range rulings {
		if ruling.Date == "" || ruling.Text == "" {
			return nil, fmt.Errorf("ruling date and text must be non-empty")
		}
		key := rulingKey(ruling.Text)
		uniqueKey := oracleID + "\x1f" + key + "\x1f" + ruling.Date
		if _, exists := state.rulingKeys[uniqueKey]; exists {
			continue
		}
		state.rulingKeys[uniqueKey] = struct{}{}
		result[tableScryfallOracleRuling] = append(result[tableScryfallOracleRuling], importRow{values: []any{oracleID, key, ruling.Date, ruling.Text}, line: line})
	}
	return result, nil
}

func collectLegality(state *importState, obj map[string]json.RawMessage, oracleID string) error {
	legalities, err := optionalObjectField(obj, "legalities")
	if err != nil {
		return err
	}
	statuses := state.legalityByOracle[oracleID]
	if statuses == nil {
		statuses = make(map[string]string)
		state.legalityByOracle[oracleID] = statuses
	}
	for _, format := range legalityFormats {
		status, err := optionalStringField(legalities, format)
		if err != nil {
			return err
		}
		if status == nil {
			continue
		}
		priority, ok := legalityPriority[*status]
		if !ok {
			return fmt.Errorf("unsupported legality status %q for %s", *status, format)
		}
		if priority > legalityPriority[statuses[format]] {
			statuses[format] = *status
		}
	}
	return nil
}

func legalityRows(state *importState) []importRow {
	rows := make([]importRow, 0, len(state.legalityByOracle))
	for oracleID, statuses := range state.legalityByOracle {
		values := make([]any, 1, len(legalityFormats)+1)
		values[0] = oracleID
		for _, format := range legalityFormats {
			if status := statuses[format]; status != "" {
				values = append(values, status)
			} else {
				values = append(values, nil)
			}
		}
		rows = append(rows, importRow{values: values})
	}
	return rows
}

func mtgjsonSetRows(set mtgjsonSet, tokenCode string, line int64) ([]importRow, error) {
	languages := make([]string, 0, len(set.Languages))
	for _, language := range set.Languages {
		code, err := languageCode(language)
		if err != nil {
			return nil, err
		}
		languages = append(languages, code)
	}
	var joinedLanguages any
	if set.Languages != nil {
		joinedLanguages = strings.Join(languages, ",")
	}
	rows := []importRow{{values: []any{
		set.Code, set.Name, pointerValue(set.ReleaseDate), set.Type, int64(len(set.Cards)), pointerValue(set.KeyruneCode),
		joinedLanguages, pointerValue(set.Block), pointerValue(set.ParentCode),
	}, line: line}}
	if len(set.Tokens) > 0 {
		rows = append(rows, importRow{values: []any{
			tokenCode, set.Name + " Tokens", pointerValue(set.ReleaseDate), "token", int64(len(set.Tokens)),
			pointerValue(set.KeyruneCode), nil, pointerValue(set.Block), set.Code,
		}, line: line})
	}
	return rows, nil
}

func mtgjsonColumnValue(obj map[string]json.RawMessage, field string, column columnSpec) (any, error) {
	raw, ok := obj[field]
	if !ok || string(raw) == "null" {
		if column.kind == kindBoolean {
			return false, nil
		}
		if zeroOnNull(column.kind) {
			return int64(0), nil
		}
		if strings.Contains(column.ddl, "NOT NULL") {
			return nil, fmt.Errorf("required value is missing or null")
		}
		return nil, nil
	}
	return columnValue(raw, column)
}

func orderedValues(table spec, named map[string]any) ([]any, error) {
	values := make([]any, len(table.columns))
	for i, column := range table.columns {
		value, exists := named[column.name]
		if !exists {
			return nil, fmt.Errorf("column %q was not populated", column.name)
		}
		if value == nil && strings.Contains(column.ddl, "NOT NULL") {
			return nil, fmt.Errorf("required column %q is null", column.name)
		}
		values[i] = value
	}
	return values, nil
}

func columnSpecByName(table spec, name string) columnSpec {
	for _, column := range table.columns {
		if column.name == name {
			return column
		}
	}
	panic("unknown column " + string(table.table) + "." + name)
}

func populateNamedDerivedNumbers(obj map[string]json.RawMessage, values map[string]any) {
	power, hasPower := decimalString(obj, "power")
	toughness, hasToughness := decimalString(obj, "toughness")
	loyalty, hasLoyalty := decimalString(obj, "loyalty")
	values["power_numeric"] = nil
	values["toughness_numeric"] = nil
	values["pt_numeric"] = nil
	if hasPower {
		values["power_numeric"] = power
	}
	if hasToughness {
		values["toughness_numeric"] = toughness
	}
	if hasPower && hasToughness {
		values["pt_numeric"] = addDecimalStrings(power, toughness)
	}
	if _, isCard := values["loyalty"]; isCard {
		values["loyalty_numeric"] = nil
		if hasLoyalty {
			values["loyalty_numeric"] = loyalty
		}
	}
}

func appendMTGJSONArrayRows(result map[tableName][]importRow, obj map[string]json.RawMessage, field string, table tableName, uuid string, line int64) error {
	values, err := stringArrayField(obj, field)
	if err != nil {
		return err
	}
	for _, value := range uniqueStrings(values) {
		result[table] = append(result[table], importRow{values: []any{uuid, value}, line: line})
	}
	return nil
}

func objectField(obj map[string]json.RawMessage, field string) (map[string]json.RawMessage, error) {
	value, err := optionalObjectField(obj, field)
	if err != nil {
		return nil, err
	}
	if value == nil {
		return nil, fmt.Errorf("field %q is missing or null", field)
	}
	return value, nil
}

func optionalObjectField(obj map[string]json.RawMessage, field string) (map[string]json.RawMessage, error) {
	if obj == nil {
		return nil, nil
	}
	raw, ok := obj[field]
	if !ok || string(raw) == "null" {
		return nil, nil
	}
	var value map[string]json.RawMessage
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, fmt.Errorf("field %q: %w", field, err)
	}
	return value, nil
}

func booleanValue(obj map[string]json.RawMessage, field string) bool {
	if obj == nil {
		return false
	}
	raw, ok := obj[field]
	if !ok || string(raw) == "null" {
		return false
	}
	var value bool
	return json.Unmarshal(raw, &value) == nil && value
}

func fallbackString(obj map[string]json.RawMessage, primary, fallback string) (*string, error) {
	value, err := optionalStringField(obj, primary)
	if err != nil {
		return nil, err
	}
	if value != nil {
		return value, nil
	}
	return optionalStringField(obj, fallback)
}

func pointerValue[T any](value *T) any {
	if value == nil {
		return nil
	}
	return *value
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
