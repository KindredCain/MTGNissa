package carddata

import "strings"

type tableName string

const (
	tableScryfallCard            tableName = "scryfall_card"
	tableCardTranslation         tableName = "card_translation"
	tableFlavorTranslation       tableName = "flavor_translation"
	tableOracleTranslation       tableName = "oracle_translation"
	tableRulingTranslation       tableName = "ruling_translation"
	tableScryfallOracleRuling    tableName = "scryfall_oracle_ruling"
	tableOracleTag               tableName = "oracle_tag"
	tableSetTranslation          tableName = "set_translation"
	tableTypeTranslation         tableName = "type_translation"
	tableScryfallCardKeyword     tableName = "scryfall_card_keyword"
	tableScryfallCardType        tableName = "scryfall_card_type"
	tableScryfallCardFrameEffect tableName = "scryfall_card_frame_effect"
	tableScryfallCardPromoType   tableName = "scryfall_card_promo_type"
	tableScryfallSet             tableName = "scryfall_set"
	tableOracleTagRelation       tableName = "oracle_tag_relation"
	tableOracleTagging           tableName = "oracle_tagging"
	tableScryfallColor           tableName = "scryfall_color"
	tableScryfallLanguage        tableName = "scryfall_language"
	tableScryfallLayout          tableName = "scryfall_layout"
	tableScryfallFrame           tableName = "scryfall_frame"
	tableScryfallFrameEffectDef  tableName = "scryfall_frame_effect"
	tableScryfallFinish          tableName = "scryfall_finish"
	tableScryfallGame            tableName = "scryfall_game"
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
	name string
	ddl  string
	kind columnKind
}

type indexSpec struct {
	name    string
	columns []string
}

type spec struct {
	table            tableName
	key              []string
	columns          []columnSpec
	indexes          []indexSpec
	allowExtraRows   bool
	autoIncrementKey bool
}

func col(name, ddl string, kind columnKind) columnSpec {
	return columnSpec{name: name, ddl: ddl, kind: kind}
}

func idx(name string, columns ...string) indexSpec {
	return indexSpec{name: name, columns: columns}
}

var specs = []spec{
	{
		table: tableScryfallCard,
		key:   []string{"uuid"},
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
			col("colors_mask", "TINYINT UNSIGNED NOT NULL", kindColorMask),
			col("color_identity_mask", "TINYINT UNSIGNED NOT NULL", kindColorMask),
			col("color_indicator_mask", "TINYINT UNSIGNED NOT NULL", kindColorMask),
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
			col("finishes_mask", "TINYINT UNSIGNED NOT NULL", kindFinishMask),
			col("flavor_name", "VARCHAR(512) NULL", kindString),
			col("flavor_text", "LONGTEXT NULL", kindString),
			col("frame", "VARCHAR(32) NOT NULL", kindString),
			col("full_art", "BOOLEAN NOT NULL", kindBoolean),
			col("games_mask", "TINYINT UNSIGNED NOT NULL", kindGameMask),
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
			col("attraction_lights_mask", "TINYINT UNSIGNED NOT NULL", kindAttractionLightMask),
			col("preview", "LONGTEXT NULL", kindRawJSONText),
			col("mana_cost", "VARCHAR(255) NULL", kindString),
			col("flavor_id", "CHAR(36) NULL", kindString),
			col("face_oracle_id", "CHAR(36) NULL", kindString),
			col("created_at", "DATETIME(6) NOT NULL", kindDateTime),
			col("updated_at", "DATETIME(6) NOT NULL", kindDateTime),
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
	},
	{
		table: tableCardTranslation, key: []string{"card_id"},
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
		indexes: []indexSpec{idx("idx_card_translation_multiverse_id", "multiverse_id")},
	},
	{
		table: tableFlavorTranslation, key: []string{"flavor_id"},
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
		indexes: []indexSpec{idx("idx_flavor_translation_print", "set", "collector_number")},
	},
	{
		table: tableOracleTranslation, key: []string{"face_oracle_id"},
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
			idx("idx_oracle_translation_oracle_id", "oracle_id"),
			idx("idx_oracle_translation_print", "set", "collector_number"),
		},
	},
	{
		table: tableRulingTranslation, key: []string{"ruling_key"}, allowExtraRows: true,
		columns: []columnSpec{
			col("ruling_key", "CHAR(64) NOT NULL", kindRulingKey),
			col("ruling_id", "CHAR(36) NULL", kindString),
			col("comment", "LONGTEXT NOT NULL", kindString),
			col("translation", "LONGTEXT NULL", kindString),
			col("translation_source", "VARCHAR(128) NULL", kindString),
			col("translation_stage", "INT NULL", kindInteger),
			col("last_published_at", "DATE NULL", kindDate),
			col("extra", "LONGTEXT NULL", kindRawJSONText),
		},
		indexes: []indexSpec{idx("idx_ruling_translation_source_id", "ruling_id")},
	},
	{
		table: tableScryfallOracleRuling,
		key:   []string{"id"},
		columns: []columnSpec{
			col("id", "BIGINT UNSIGNED NOT NULL AUTO_INCREMENT", kindAutoIncrement),
			col("oracle_id", "CHAR(36) NOT NULL", kindString),
			col("ruling_key", "CHAR(64) NOT NULL", kindRulingKey),
			col("source", "VARCHAR(32) NOT NULL", kindString),
			col("published_at", "DATE NOT NULL", kindDate),
		},
		indexes: []indexSpec{
			idx("idx_scryfall_oracle_ruling_oracle", "oracle_id", "published_at"),
			idx("idx_scryfall_oracle_ruling_rule", "ruling_key", "oracle_id"),
		},
		autoIncrementKey: true,
	},
	{
		table: tableOracleTag,
		key:   []string{"tag_id"},
		columns: []columnSpec{
			col("tag_id", "CHAR(36) NOT NULL", kindString),
			col("label", "VARCHAR(255) NOT NULL", kindString),
			col("description", "LONGTEXT NULL", kindString),
			col("aliases", "VARCHAR(1024) NULL", kindStringArray),
		},
		indexes: []indexSpec{idx("idx_oracle_tag_label", "label")},
	},
	{
		table: tableSetTranslation, key: []string{"set_id"},
		columns: []columnSpec{
			col("set_id", "CHAR(36) NOT NULL", kindString),
			col("code", "VARCHAR(32) NULL", kindString),
			col("name", "VARCHAR(255) NULL", kindString),
			col("source", "VARCHAR(128) NULL", kindString),
			col("stage", "INT NULL", kindInteger),
		},
		indexes: []indexSpec{idx("idx_set_translation_code", "code")},
	},
	{
		table: tableTypeTranslation, key: []string{"type_name", "type_type"},
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
		table: tableScryfallCardKeyword, key: []string{"card_uuid", "keyword"},
		columns: []columnSpec{
			col("card_uuid", "CHAR(36) NOT NULL", kindString),
			col("keyword", "VARCHAR(255) NOT NULL", kindString),
		},
		indexes: []indexSpec{idx("idx_scryfall_card_keyword_lookup", "keyword", "card_uuid")},
	},
	{
		table: tableScryfallCardType, key: []string{"card_uuid", "type_group", "type_name"},
		columns: []columnSpec{
			col("card_uuid", "CHAR(36) NOT NULL", kindString),
			col("type_group", "VARCHAR(16) NOT NULL", kindString),
			col("type_name", "VARCHAR(128) NOT NULL", kindString),
		},
		indexes: []indexSpec{idx("idx_scryfall_card_type_lookup", "type_group", "type_name", "card_uuid")},
	},
	{
		table: tableScryfallCardFrameEffect, key: []string{"card_uuid", "frame_effect"},
		columns: []columnSpec{
			col("card_uuid", "CHAR(36) NOT NULL", kindString),
			col("frame_effect", "VARCHAR(64) NOT NULL", kindString),
		},
		indexes: []indexSpec{idx("idx_scryfall_card_frame_effect_lookup", "frame_effect", "card_uuid")},
	},
	{
		table: tableScryfallCardPromoType, key: []string{"card_uuid", "promo_type"},
		columns: []columnSpec{
			col("card_uuid", "CHAR(36) NOT NULL", kindString),
			col("promo_type", "VARCHAR(128) NOT NULL", kindString),
		},
		indexes: []indexSpec{idx("idx_scryfall_card_promo_type_lookup", "promo_type", "card_uuid")},
	},
	{
		table: tableScryfallSet, key: []string{"set_id"},
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
		table: tableOracleTagRelation, key: []string{"parent_tag_id", "child_tag_id"},
		columns: []columnSpec{
			col("parent_tag_id", "CHAR(36) NOT NULL", kindString),
			col("child_tag_id", "CHAR(36) NOT NULL", kindString),
		},
		indexes: []indexSpec{idx("idx_oracle_tag_relation_child", "child_tag_id", "parent_tag_id")},
	},
	{
		table: tableOracleTagging, key: []string{"oracle_id", "tag_id"},
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
		table: tableScryfallColor, key: []string{"code"},
		columns: []columnSpec{
			col("code", "CHAR(1) NOT NULL", kindString),
			col("bit_value", "TINYINT UNSIGNED NOT NULL", kindInteger),
			col("name", "VARCHAR(32) NOT NULL", kindString),
			col("description", "VARCHAR(255) NOT NULL", kindString),
			col("sort_order", "TINYINT UNSIGNED NOT NULL", kindInteger),
		},
	},
	{
		table: tableScryfallLanguage, key: []string{"code"},
		columns: []columnSpec{
			col("code", "VARCHAR(8) NOT NULL", kindString),
			col("name", "VARCHAR(64) NOT NULL", kindString),
		},
	},
	{
		table: tableScryfallLayout, key: []string{"code"},
		columns: []columnSpec{
			col("code", "VARCHAR(64) NOT NULL", kindString),
			col("name", "VARCHAR(128) NOT NULL", kindString),
			col("description", "VARCHAR(512) NOT NULL", kindString),
			col("face_category", "VARCHAR(32) NOT NULL", kindString),
		},
	},
	{
		table: tableScryfallFrame, key: []string{"code"},
		columns: []columnSpec{
			col("code", "VARCHAR(32) NOT NULL", kindString),
			col("name", "VARCHAR(128) NOT NULL", kindString),
			col("description", "VARCHAR(512) NOT NULL", kindString),
		},
	},
	{
		table: tableScryfallFrameEffectDef, key: []string{"code"},
		columns: []columnSpec{
			col("code", "VARCHAR(64) NOT NULL", kindString),
			col("description", "VARCHAR(512) NOT NULL", kindString),
		},
	},
	{
		table: tableScryfallFinish, key: []string{"code"},
		columns: []columnSpec{
			col("code", "VARCHAR(32) NOT NULL", kindString),
			col("bit_value", "TINYINT UNSIGNED NOT NULL", kindInteger),
			col("name", "VARCHAR(64) NOT NULL", kindString),
		},
	},
	{
		table: tableScryfallGame, key: []string{"code"},
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

func mustTableSpec(name tableName) spec {
	for _, table := range allTableSpecs() {
		if table.table == name {
			return table
		}
	}
	panic("unknown table spec: " + string(name))
}

func createTableSQL(table string, s spec) string {
	parts := make([]string, 0, len(s.columns)+len(s.indexes)+1)
	for _, column := range s.columns {
		parts = append(parts, quote(column.name)+" "+column.ddl)
	}
	parts = append(parts, "PRIMARY KEY ("+quotedList(s.key)+")")
	for _, index := range s.indexes {
		parts = append(parts, "KEY "+quote(index.name)+" ("+quotedList(index.columns)+")")
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
