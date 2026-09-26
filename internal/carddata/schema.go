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

type columnKind uint8

const (
	kindString columnKind = iota
	kindInteger
	kindNumber
	kindBoolean
	kindDate
	kindDateTime
	kindColorMask
	kindFinishMask
	kindGameMask
	kindAttractionLightMask
	kindStringArray
	kindRawJSONText
	kindRulingKey
	kindAutoIncrement
)

type columnSpec struct {
	name   string
	source string
	ddl    string
	kind   columnKind
}

type indexSpec struct {
	name    string
	columns []string
}

type spec struct {
	table, file      string
	required, key    []string
	columns          []columnSpec
	derivedFields    []string
	indexes          []indexSpec
	fulltextIndexes  []indexSpec
	standardJSON     bool
	allowExtraRows   bool
	autoIncrementKey bool
}

func col(name, ddl string, kind columnKind) columnSpec {
	return columnSpec{name: name, ddl: ddl, kind: kind}
}

func derivedCol(name, source, ddl string, kind columnKind) columnSpec {
	return columnSpec{name: name, source: source, ddl: ddl, kind: kind}
}

func idx(name string, columns ...string) indexSpec {
	return indexSpec{name: name, columns: columns}
}

var specs = []spec{
	{
		table:    "scryfall_card",
		file:     "scryfall_card.json",
		required: []string{"uuid", "scryfall_id", "name", "set_code", "collector_number", "lang"},
		key:      []string{"uuid"},
		columns: []columnSpec{
			col("uuid", "CHAR(36) NOT NULL", kindString),
			col("scryfall_id", "CHAR(36) NOT NULL", kindString),
			col("face_index", "INT NOT NULL", kindInteger),
			col("lang", "VARCHAR(16) NOT NULL", kindString),
			col("oracle_id", "CHAR(36) NULL", kindString),
			col("layout", "VARCHAR(64) NOT NULL", kindString),
			col("arena_id", "BIGINT NULL", kindInteger),
			col("mtgo_id", "BIGINT NULL", kindInteger),
			col("mtgo_foil_id", "BIGINT NULL", kindInteger),
			col("multiverse_id", "BIGINT NULL", kindInteger),
			col("tcgplayer_id", "BIGINT NULL", kindInteger),
			col("tcgplayer_etched_id", "BIGINT NULL", kindInteger),
			col("cardmarket_id", "BIGINT NULL", kindInteger),
			col("resource_id", "VARCHAR(255) NULL", kindString),
			col("cmc", "DECIMAL(12,4) NOT NULL", kindNumber),
			derivedCol("colors_mask", "colors", "TINYINT UNSIGNED NOT NULL", kindColorMask),
			derivedCol("color_identity_mask", "color_identity", "TINYINT UNSIGNED NOT NULL", kindColorMask),
			derivedCol("color_indicator_mask", "color_indicator", "TINYINT UNSIGNED NOT NULL", kindColorMask),
			col("produced_mana", "VARCHAR(64) NULL", kindStringArray),
			col("defense", "VARCHAR(32) NULL", kindString),
			col("game_changer", "BOOLEAN NOT NULL", kindBoolean),
			col("hand_modifier", "VARCHAR(32) NULL", kindString),
			col("life_modifier", "VARCHAR(32) NULL", kindString),
			col("loyalty", "VARCHAR(32) NULL", kindString),
			col("name", "VARCHAR(512) NOT NULL", kindString),
			col("face_name", "VARCHAR(512) NULL", kindString),
			col("oracle_text", "LONGTEXT NULL", kindString),
			col("power", "VARCHAR(32) NULL", kindString),
			col("reserved", "BOOLEAN NOT NULL", kindBoolean),
			col("toughness", "VARCHAR(32) NULL", kindString),
			col("type_line", "VARCHAR(512) NOT NULL", kindString),
			col("artist", "VARCHAR(255) NULL", kindString),
			col("artist_ids", "VARCHAR(2048) NULL", kindStringArray),
			col("booster", "BOOLEAN NOT NULL", kindBoolean),
			col("border_color", "VARCHAR(32) NOT NULL", kindString),
			col("card_back_id", "CHAR(36) NULL", kindString),
			col("collector_number", "VARCHAR(64) NOT NULL", kindString),
			col("content_warning", "BOOLEAN NULL", kindBoolean),
			col("digital", "BOOLEAN NOT NULL", kindBoolean),
			derivedCol("finishes_mask", "finishes", "TINYINT UNSIGNED NOT NULL", kindFinishMask),
			col("flavor_name", "VARCHAR(512) NULL", kindString),
			col("flavor_text", "LONGTEXT NULL", kindString),
			col("frame", "VARCHAR(32) NOT NULL", kindString),
			col("full_art", "BOOLEAN NOT NULL", kindBoolean),
			derivedCol("games_mask", "games", "TINYINT UNSIGNED NOT NULL", kindGameMask),
			col("highres_image", "BOOLEAN NOT NULL", kindBoolean),
			col("illustration_id", "CHAR(36) NULL", kindString),
			col("image_status", "VARCHAR(32) NOT NULL", kindString),
			col("oversized", "BOOLEAN NOT NULL", kindBoolean),
			col("printed_name", "VARCHAR(512) NULL", kindString),
			col("printed_text", "LONGTEXT NULL", kindString),
			col("printed_type_line", "VARCHAR(512) NULL", kindString),
			col("promo", "BOOLEAN NOT NULL", kindBoolean),
			col("rarity", "VARCHAR(32) NOT NULL", kindString),
			col("released_at", "DATE NOT NULL", kindDate),
			col("reprint", "BOOLEAN NOT NULL", kindBoolean),
			col("set_id", "CHAR(36) NOT NULL", kindString),
			col("story_spotlight", "BOOLEAN NOT NULL", kindBoolean),
			col("textless", "BOOLEAN NOT NULL", kindBoolean),
			col("variation", "BOOLEAN NOT NULL", kindBoolean),
			col("variation_of", "CHAR(36) NULL", kindString),
			col("security_stamp", "VARCHAR(64) NULL", kindString),
			col("watermark", "VARCHAR(128) NULL", kindString),
			col("foil", "BOOLEAN NOT NULL", kindBoolean),
			col("nonfoil", "BOOLEAN NOT NULL", kindBoolean),
			derivedCol("attraction_lights_mask", "attraction_lights", "TINYINT UNSIGNED NOT NULL", kindAttractionLightMask),
			col("preview", "LONGTEXT NULL", kindRawJSONText),
			col("mana_cost", "VARCHAR(255) NULL", kindString),
			col("flavor_id", "CHAR(36) NULL", kindString),
			col("face_oracle_id", "CHAR(36) NULL", kindString),
			col("created_at", "DATETIME(6) NOT NULL", kindDateTime),
			col("updated_at", "DATETIME(6) NOT NULL", kindDateTime),
		},
		derivedFields: []string{
			"keywords", "frame_effects", "promo_types",
			"set_code", "set_name", "set_type",
		},
		indexes: []indexSpec{
			idx("idx_scryfall_card_scryfall_id", "scryfall_id"),
			idx("idx_scryfall_card_oracle_id", "oracle_id"),
			idx("idx_scryfall_card_face_oracle_id", "face_oracle_id"),
			idx("idx_scryfall_card_flavor_id", "flavor_id"),
			idx("idx_scryfall_card_set_id", "set_id"),
			idx("idx_scryfall_card_multiverse_id", "multiverse_id"),
			idx("idx_scryfall_card_set_print", "set_id", "collector_number", "lang"),
			idx("idx_scryfall_card_cmc", "cmc"),
			idx("idx_scryfall_card_mana_cost", "mana_cost"),
			idx("idx_scryfall_card_rarity", "rarity"),
			idx("idx_scryfall_card_released_at", "released_at"),
			idx("idx_scryfall_card_name", "name"),
			idx("idx_scryfall_card_language", "lang"),
			idx("idx_scryfall_card_layout", "layout"),
			idx("idx_scryfall_card_frame", "frame"),
			idx("idx_scryfall_card_colors_mask", "colors_mask"),
			idx("idx_scryfall_card_identity_mask", "color_identity_mask"),
			idx("idx_scryfall_card_finishes_mask", "finishes_mask"),
			idx("idx_scryfall_card_games_mask", "games_mask"),
		},
		fulltextIndexes: []indexSpec{idx("ft_scryfall_card_search", "name", "face_name", "type_line", "oracle_text")},
	},
	{
		table: "zhs_card", file: "zhs_card.json", required: []string{"card_id"}, key: []string{"card_id"},
		columns: []columnSpec{
			col("card_id", "CHAR(36) NOT NULL", kindString),
			col("name", "VARCHAR(512) NULL", kindString),
			col("face_name", "VARCHAR(512) NULL", kindString),
			col("flavor_name", "VARCHAR(512) NULL", kindString),
			col("type_line", "VARCHAR(512) NULL", kindString),
			col("text", "LONGTEXT NULL", kindString),
			col("flavor_text", "LONGTEXT NULL", kindString),
			col("multiverse_id", "BIGINT NULL", kindInteger),
			col("source", "VARCHAR(128) NULL", kindString),
			col("extra", "LONGTEXT NULL", kindString),
		},
		indexes: []indexSpec{idx("idx_zhs_card_multiverse_id", "multiverse_id")},
	},
	{
		table: "zhs_flavor", file: "zhs_flavor.json", required: []string{"flavor_id"}, key: []string{"flavor_id"},
		columns: []columnSpec{
			col("flavor_id", "CHAR(36) NOT NULL", kindString),
			col("name", "VARCHAR(512) NULL", kindString),
			col("flavor_name", "VARCHAR(512) NULL", kindString),
			col("flavor_text", "LONGTEXT NULL", kindString),
			col("set", "VARCHAR(32) NULL", kindString),
			col("collector_number", "VARCHAR(64) NULL", kindString),
			col("released_at", "DATE NULL", kindDate),
			col("translated_flavor_name", "VARCHAR(512) NULL", kindString),
			col("translated_flavor_text", "LONGTEXT NULL", kindString),
			col("flavor_updated_at", "DATE NULL", kindDate),
			col("extra", "LONGTEXT NULL", kindString),
			col("name_source", "VARCHAR(128) NULL", kindString),
			col("name_stage", "INT NULL", kindInteger),
			col("text_source", "VARCHAR(128) NULL", kindString),
			col("text_stage", "INT NULL", kindInteger),
		},
		indexes: []indexSpec{idx("idx_zhs_flavor_print", "set", "collector_number")},
	},
	{
		table: "zhs_oracle", file: "zhs_oracle.json", required: []string{"face_oracle_id"}, key: []string{"face_oracle_id"},
		columns: []columnSpec{
			col("face_oracle_id", "CHAR(36) NOT NULL", kindString),
			col("oracle_id", "CHAR(36) NULL", kindString),
			col("name", "VARCHAR(512) NULL", kindString),
			col("set", "VARCHAR(32) NULL", kindString),
			col("collector_number", "VARCHAR(64) NULL", kindString),
			col("released_at", "DATE NULL", kindDate),
			col("type_line", "VARCHAR(512) NULL", kindString),
			col("oracle_text", "LONGTEXT NULL", kindString),
			col("translated_name", "VARCHAR(512) NULL", kindString),
			col("name_stage", "INT NULL", kindInteger),
			col("name_source", "VARCHAR(128) NULL", kindString),
			col("translated_type", "VARCHAR(512) NULL", kindString),
			col("type_stage", "INT NULL", kindInteger),
			col("translated_text", "LONGTEXT NULL", kindString),
			col("text_stage", "INT NULL", kindInteger),
			col("text_source", "VARCHAR(128) NULL", kindString),
			col("extra", "LONGTEXT NULL", kindString),
			col("former_names", "LONGTEXT NULL", kindRawJSONText),
		},
		indexes: []indexSpec{
			idx("idx_zhs_oracle_oracle_id", "oracle_id"),
			idx("idx_zhs_oracle_print", "set", "collector_number"),
		},
	},
	{
		table: "zhs_ruling", file: "zhs_ruling.json", required: []string{"ruling", "comment"}, key: []string{"ruling_key"}, allowExtraRows: true,
		columns: []columnSpec{
			derivedCol("ruling_key", "comment", "CHAR(64) NOT NULL", kindRulingKey),
			derivedCol("ruling_id", "ruling", "CHAR(36) NULL", kindString),
			col("comment", "LONGTEXT NOT NULL", kindString),
			col("translation", "LONGTEXT NULL", kindString),
			derivedCol("translation_source", "source", "VARCHAR(128) NULL", kindString),
			derivedCol("translation_stage", "stage", "INT NULL", kindInteger),
			col("last_published_at", "DATE NULL", kindDate),
			col("extra", "LONGTEXT NULL", kindRawJSONText),
		},
		indexes: []indexSpec{idx("idx_zhs_ruling_source_id", "ruling_id")},
	},
	{
		table: "scryfall_oracle_ruling", file: "rulings.jsonl",
		required: []string{"object", "oracle_id", "source", "published_at", "comment"},
		key:      []string{"id"},
		columns: []columnSpec{
			col("id", "BIGINT UNSIGNED NOT NULL AUTO_INCREMENT", kindAutoIncrement),
			col("oracle_id", "CHAR(36) NOT NULL", kindString),
			derivedCol("ruling_key", "comment", "CHAR(64) NOT NULL", kindRulingKey),
			col("source", "VARCHAR(32) NOT NULL", kindString),
			col("published_at", "DATE NOT NULL", kindDate),
		},
		derivedFields: []string{"object"},
		indexes: []indexSpec{
			idx("idx_scryfall_oracle_ruling_oracle", "oracle_id", "published_at"),
			idx("idx_scryfall_oracle_ruling_rule", "ruling_key", "oracle_id"),
		},
		standardJSON:     true,
		autoIncrementKey: true,
	},
	{
		table: "oracle_tag", file: "oracle-tags.jsonl",
		required: []string{"object", "id", "label", "type", "parent_ids", "child_ids", "aliases", "taggings"},
		key:      []string{"tag_id"},
		columns: []columnSpec{
			derivedCol("tag_id", "id", "CHAR(36) NOT NULL", kindString),
			col("label", "VARCHAR(255) NOT NULL", kindString),
			col("description", "LONGTEXT NULL", kindString),
			col("aliases", "VARCHAR(1024) NULL", kindStringArray),
		},
		derivedFields: []string{"object", "slug", "type", "uri", "parent_ids", "child_ids", "taggings"},
		indexes:       []indexSpec{idx("idx_oracle_tag_label", "label")},
		standardJSON:  true,
	},
	{
		table: "zhs_set", file: "zhs_set.json", required: []string{"set_id"}, key: []string{"set_id"},
		columns: []columnSpec{
			col("set_id", "CHAR(36) NOT NULL", kindString),
			col("code", "VARCHAR(32) NULL", kindString),
			col("name", "VARCHAR(255) NULL", kindString),
			col("source", "VARCHAR(128) NULL", kindString),
			col("stage", "INT NULL", kindInteger),
		},
		indexes: []indexSpec{idx("idx_zhs_set_code", "code")},
	},
	{
		table: "zhs_type", file: "zhs_type.json", required: []string{"type_name", "type_type"}, key: []string{"type_name", "type_type"},
		columns: []columnSpec{
			col("type_name", "VARCHAR(255) NOT NULL", kindString),
			col("type_type", "VARCHAR(64) NOT NULL", kindString),
			col("translation", "VARCHAR(512) NULL", kindString),
			col("stage", "INT NULL", kindInteger),
			col("created_at", "DATETIME(6) NULL", kindDateTime),
			col("is_funny", "BOOLEAN NULL", kindBoolean),
		},
	},
}

var derivedSpecs = []spec{
	{
		table: "scryfall_card_keyword", key: []string{"card_uuid", "keyword"},
		columns: []columnSpec{
			col("card_uuid", "CHAR(36) NOT NULL", kindString),
			col("keyword", "VARCHAR(255) NOT NULL", kindString),
		},
		indexes: []indexSpec{idx("idx_scryfall_card_keyword_lookup", "keyword", "card_uuid")},
	},
	{
		table: "scryfall_card_type", key: []string{"card_uuid", "type_group", "type_name"},
		columns: []columnSpec{
			col("card_uuid", "CHAR(36) NOT NULL", kindString),
			col("type_group", "VARCHAR(16) NOT NULL", kindString),
			col("type_name", "VARCHAR(128) NOT NULL", kindString),
		},
		indexes: []indexSpec{idx("idx_scryfall_card_type_lookup", "type_group", "type_name", "card_uuid")},
	},
	{
		table: "scryfall_card_frame_effect", key: []string{"card_uuid", "frame_effect"},
		columns: []columnSpec{
			col("card_uuid", "CHAR(36) NOT NULL", kindString),
			col("frame_effect", "VARCHAR(64) NOT NULL", kindString),
		},
		indexes: []indexSpec{idx("idx_scryfall_card_frame_effect_lookup", "frame_effect", "card_uuid")},
	},
	{
		table: "scryfall_card_promo_type", key: []string{"card_uuid", "promo_type"},
		columns: []columnSpec{
			col("card_uuid", "CHAR(36) NOT NULL", kindString),
			col("promo_type", "VARCHAR(128) NOT NULL", kindString),
		},
		indexes: []indexSpec{idx("idx_scryfall_card_promo_type_lookup", "promo_type", "card_uuid")},
	},
	{
		table: "scryfall_set", key: []string{"set_id"},
		columns: []columnSpec{
			col("set_id", "CHAR(36) NOT NULL", kindString),
			col("code", "VARCHAR(32) NOT NULL", kindString),
			col("name", "VARCHAR(255) NOT NULL", kindString),
			col("set_type", "VARCHAR(64) NOT NULL", kindString),
		},
		indexes: []indexSpec{
			idx("idx_scryfall_set_code", "code"),
			idx("idx_scryfall_set_type", "set_type"),
		},
	},
	{
		table: "oracle_tag_relation", key: []string{"parent_tag_id", "child_tag_id"},
		columns: []columnSpec{
			col("parent_tag_id", "CHAR(36) NOT NULL", kindString),
			col("child_tag_id", "CHAR(36) NOT NULL", kindString),
		},
		indexes: []indexSpec{idx("idx_oracle_tag_relation_child", "child_tag_id", "parent_tag_id")},
	},
	{
		table: "oracle_tagging", key: []string{"oracle_id", "tag_id"},
		columns: []columnSpec{
			col("oracle_id", "CHAR(36) NOT NULL", kindString),
			col("tag_id", "CHAR(36) NOT NULL", kindString),
			col("weight", "VARCHAR(16) NOT NULL", kindString),
		},
		indexes: []indexSpec{idx("idx_oracle_tagging_tag", "tag_id", "weight", "oracle_id")},
	},
}

var dictionarySpecs = []spec{
	{
		table: "scryfall_color", key: []string{"code"},
		columns: []columnSpec{
			col("code", "CHAR(1) NOT NULL", kindString),
			col("bit_value", "TINYINT UNSIGNED NOT NULL", kindInteger),
			col("name", "VARCHAR(32) NOT NULL", kindString),
			col("description", "VARCHAR(255) NOT NULL", kindString),
			col("sort_order", "TINYINT UNSIGNED NOT NULL", kindInteger),
		},
	},
	{
		table: "scryfall_language", key: []string{"code"},
		columns: []columnSpec{
			col("code", "VARCHAR(8) NOT NULL", kindString),
			col("name", "VARCHAR(64) NOT NULL", kindString),
		},
	},
	{
		table: "scryfall_layout", key: []string{"code"},
		columns: []columnSpec{
			col("code", "VARCHAR(64) NOT NULL", kindString),
			col("name", "VARCHAR(128) NOT NULL", kindString),
			col("description", "VARCHAR(512) NOT NULL", kindString),
			col("face_category", "VARCHAR(32) NOT NULL", kindString),
		},
	},
	{
		table: "scryfall_frame", key: []string{"code"},
		columns: []columnSpec{
			col("code", "VARCHAR(32) NOT NULL", kindString),
			col("name", "VARCHAR(128) NOT NULL", kindString),
			col("description", "VARCHAR(512) NOT NULL", kindString),
		},
	},
	{
		table: "scryfall_frame_effect", key: []string{"code"},
		columns: []columnSpec{
			col("code", "VARCHAR(64) NOT NULL", kindString),
			col("description", "VARCHAR(512) NOT NULL", kindString),
		},
	},
	{
		table: "scryfall_finish", key: []string{"code"},
		columns: []columnSpec{
			col("code", "VARCHAR(32) NOT NULL", kindString),
			col("bit_value", "TINYINT UNSIGNED NOT NULL", kindInteger),
			col("name", "VARCHAR(64) NOT NULL", kindString),
		},
	},
	{
		table: "scryfall_game", key: []string{"code"},
		columns: []columnSpec{
			col("code", "VARCHAR(32) NOT NULL", kindString),
			col("bit_value", "TINYINT UNSIGNED NOT NULL", kindInteger),
			col("name", "VARCHAR(64) NOT NULL", kindString),
		},
	},
}

func allTableSpecs() []spec {
	tables := make([]spec, 0, len(specs)+len(derivedSpecs)+len(dictionarySpecs))
	tables = append(tables, specs...)
	tables = append(tables, derivedSpecs...)
	tables = append(tables, dictionarySpecs...)
	return tables
}

func createTableSQL(table string, s spec) string {
	parts := make([]string, 0, len(s.columns)+len(s.indexes)+len(s.fulltextIndexes)+1)
	for _, column := range s.columns {
		parts = append(parts, quote(column.name)+" "+column.ddl)
	}
	parts = append(parts, "PRIMARY KEY ("+quotedList(s.key)+")")
	for _, index := range s.indexes {
		parts = append(parts, "KEY "+quote(index.name)+" ("+quotedList(index.columns)+")")
	}
	for _, index := range s.fulltextIndexes {
		parts = append(parts, "FULLTEXT KEY "+quote(index.name)+" ("+quotedList(index.columns)+")")
	}
	return "CREATE TABLE " + quote(table) + " (" + strings.Join(parts, ",") + ") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci"
}

func quotedList(names []string) string {
	quoted := make([]string, len(names))
	for i, name := range names {
		quoted[i] = quote(name)
	}
	return strings.Join(quoted, ",")
}

func rowValues(obj map[string]json.RawMessage, s spec) ([]any, error) {
	known := make(map[string]struct{}, len(s.columns)+len(s.derivedFields))
	values := make([]any, len(s.columns))
	for i, column := range s.columns {
		if column.kind == kindAutoIncrement {
			values[i] = nil
			continue
		}
		source := column.name
		if column.source != "" {
			source = column.source
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
	for _, name := range s.derivedFields {
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
		return parsed.Format("2006-01-02 15:04:05.999999"), nil
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
