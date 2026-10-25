package carddata

import "strings"

type tableName string

const (
	tableScryfallCard            tableName = "mtgnissa_card"
	tableMTGNissaToken           tableName = "mtgnissa_token"
	tableMTGNissaCardPrinting    tableName = "mtgnissa_card_printing"
	tableCardTranslation         tableName = "card_translation"
	tableFlavorTranslation       tableName = "flavor_translation"
	tableOracleTranslation       tableName = "oracle_translation"
	tableRulingTranslation       tableName = "ruling_translation"
	tableScryfallOracleRuling    tableName = "oracle_ruling"
	tableOracleLegality          tableName = "oracle_legality"
	tableOracleTag               tableName = "oracle_tag"
	tableSetTranslation          tableName = "set_translation"
	tableTypeTranslation         tableName = "type_translation"
	tableScryfallCardKeyword     tableName = "scryfall_card_keyword"
	tableScryfallCardType        tableName = "scryfall_card_type"
	tableScryfallCardFrameEffect tableName = "scryfall_card_frame_effect"
	tableScryfallCardPromoType   tableName = "scryfall_card_promo_type"
	tableScryfallSet             tableName = "mtgnissa_set"
	tableCardFlavorMapping       tableName = "card_flavor_translation_mapping"
	tableCardOracleMapping       tableName = "card_oracle_translation_mapping"
	tableOracleTagRelation       tableName = "oracle_tag_relation"
	tableOracleTagging           tableName = "oracle_tag_mapping"
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
	kindDerivedNumber
)

type columnSpec struct {
	name string
	ddl  string
	kind columnKind
}

type indexSpec struct {
	name    string
	columns []string
	unique  bool
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

func uniqueIdx(name string, columns ...string) indexSpec {
	return indexSpec{name: name, columns: columns, unique: true}
}

var specs = []spec{
	{
		table: tableScryfallCard,
		key:   []string{"uuid"},
		columns: []columnSpec{
			col("uuid", "CHAR(36) NOT NULL", kindString),
			col("scryfall_id", "CHAR(36) NOT NULL", kindString),
			col("face_index", "INT NOT NULL", kindInteger),
			col("oracle_id", "CHAR(36) NULL", kindString),
			col("layout", "VARCHAR(32) NOT NULL", kindString),
			col("cmc", "DECIMAL(12,4) NOT NULL", kindNumber),
			col("colors_mask", "BIGINT UNSIGNED NOT NULL", kindColorMask),
			col("color_identity_mask", "BIGINT UNSIGNED NOT NULL", kindColorMask),
			col("color_indicator_mask", "BIGINT UNSIGNED NOT NULL", kindColorMask),
			col("produced_mana", "VARCHAR(32) NULL", kindStringArray),
			col("defense", "VARCHAR(32) NULL", kindString),
			col("game_changer", "BOOLEAN NOT NULL", kindBoolean),
			col("hand_modifier", "VARCHAR(32) NULL", kindString),
			col("life_modifier", "VARCHAR(32) NULL", kindString),
			col("loyalty", "VARCHAR(32) NULL", kindString),
			col("loyalty_numeric", "DECIMAL(8,2) NULL", kindDerivedNumber),
			col("name", "VARCHAR(512) NOT NULL", kindString),
			col("face_name", "VARCHAR(512) NULL", kindString),
			col("oracle_text", "LONGTEXT NULL", kindString),
			col("power", "VARCHAR(32) NULL", kindString),
			col("power_numeric", "DECIMAL(8,2) NULL", kindDerivedNumber),
			col("reserved", "BOOLEAN NOT NULL", kindBoolean),
			col("toughness", "VARCHAR(32) NULL", kindString),
			col("toughness_numeric", "DECIMAL(8,2) NULL", kindDerivedNumber),
			col("pt_numeric", "DECIMAL(12,4) NULL", kindDerivedNumber),
			col("type_line", "VARCHAR(512) NOT NULL", kindString),
			col("artist", "VARCHAR(512) NULL", kindString),
			col("artist_ids", "VARCHAR(2048) NULL", kindStringArray),
			col("booster", "BOOLEAN NOT NULL", kindBoolean),
			col("border_color", "VARCHAR(32) NOT NULL", kindString),
			col("collector_number", "VARCHAR(64) NOT NULL", kindString),
			col("content_warning", "BOOLEAN NOT NULL", kindBoolean),
			col("digital", "BOOLEAN NOT NULL", kindBoolean),
			col("finishes_mask", "BIGINT UNSIGNED NOT NULL", kindFinishMask),
			col("flavor_name", "VARCHAR(512) NULL", kindString),
			col("flavor_text", "LONGTEXT NULL", kindString),
			col("frame", "VARCHAR(16) NULL", kindString),
			col("full_art", "BOOLEAN NOT NULL", kindBoolean),
			col("games_mask", "BIGINT UNSIGNED NOT NULL", kindGameMask),
			col("oversized", "BOOLEAN NOT NULL", kindBoolean),
			col("promo", "BOOLEAN NOT NULL", kindBoolean),
			col("rarity", "VARCHAR(32) NOT NULL", kindString),
			col("released_at", "DATE NULL", kindDate),
			col("reprint", "BOOLEAN NOT NULL", kindBoolean),
			col("set_code", "VARCHAR(16) NOT NULL", kindString),
			col("story_spotlight", "BOOLEAN NOT NULL", kindBoolean),
			col("textless", "BOOLEAN NOT NULL", kindBoolean),
			col("variation", "BOOLEAN NOT NULL", kindBoolean),
			col("variations", "TEXT NULL", kindStringArray),
			col("security_stamp", "VARCHAR(64) NULL", kindString),
			col("watermark", "VARCHAR(128) NULL", kindString),
			col("foil", "BOOLEAN NOT NULL", kindBoolean),
			col("nonfoil", "BOOLEAN NOT NULL", kindBoolean),
			col("attraction_lights_mask", "BIGINT UNSIGNED NOT NULL", kindAttractionLightMask),
			col("mana_cost", "VARCHAR(512) NULL", kindString),
			col("has_alternative_deck_limit", "BOOLEAN NOT NULL", kindBoolean),
			col("brawl_leader", "BOOLEAN NOT NULL", kindBoolean),
			col("commander_leader", "BOOLEAN NOT NULL", kindBoolean),
			col("oathbreaker_leader", "BOOLEAN NOT NULL", kindBoolean),
		},
		indexes: []indexSpec{
			uniqueIdx("uq_mtgnissa_card_scryfall_face", "scryfall_id", "face_index"),
			idx("idx_mtgnissa_card_oracle_id", "oracle_id"),
			idx("idx_mtgnissa_card_set_code", "set_code"),
			idx("idx_mtgnissa_card_name", "name"),
			idx("idx_mtgnissa_card_released_at", "released_at"),
			idx("idx_mtgnissa_card_cmc", "cmc"),
			idx("idx_mtgnissa_card_power_numeric", "power_numeric"),
			idx("idx_mtgnissa_card_toughness_numeric", "toughness_numeric"),
			idx("idx_mtgnissa_card_loyalty_numeric", "loyalty_numeric"),
			idx("idx_mtgnissa_card_pt_numeric", "pt_numeric"),
		},
	},
	{
		table: tableMTGNissaToken,
		key:   []string{"uuid"},
		columns: []columnSpec{
			col("uuid", "CHAR(36) NOT NULL", kindString),
			col("scryfall_id", "CHAR(36) NULL", kindString),
			col("face_index", "INT NOT NULL", kindInteger),
			col("oracle_id", "CHAR(36) NULL", kindString),
			col("layout", "VARCHAR(32) NOT NULL", kindString),
			col("colors_mask", "BIGINT UNSIGNED NOT NULL", kindColorMask),
			col("color_identity_mask", "BIGINT UNSIGNED NOT NULL", kindColorMask),
			col("produced_mana", "VARCHAR(32) NULL", kindStringArray),
			col("name", "VARCHAR(512) NOT NULL", kindString),
			col("face_name", "VARCHAR(512) NULL", kindString),
			col("oracle_text", "LONGTEXT NULL", kindString),
			col("power", "VARCHAR(32) NULL", kindString),
			col("power_numeric", "DECIMAL(8,2) NULL", kindDerivedNumber),
			col("toughness", "VARCHAR(32) NULL", kindString),
			col("toughness_numeric", "DECIMAL(8,2) NULL", kindDerivedNumber),
			col("pt_numeric", "DECIMAL(12,4) NULL", kindDerivedNumber),
			col("type_line", "VARCHAR(512) NOT NULL", kindString),
			col("artist", "VARCHAR(512) NULL", kindString),
			col("artist_ids", "VARCHAR(2048) NULL", kindStringArray),
			col("booster", "BOOLEAN NOT NULL", kindBoolean),
			col("border_color", "VARCHAR(32) NOT NULL", kindString),
			col("collector_number", "VARCHAR(64) NOT NULL", kindString),
			col("finishes_mask", "BIGINT UNSIGNED NOT NULL", kindFinishMask),
			col("flavor_name", "VARCHAR(512) NULL", kindString),
			col("flavor_text", "LONGTEXT NULL", kindString),
			col("frame", "VARCHAR(16) NULL", kindString),
			col("full_art", "BOOLEAN NOT NULL", kindBoolean),
			col("games_mask", "BIGINT UNSIGNED NOT NULL", kindGameMask),
			col("oversized", "BOOLEAN NOT NULL", kindBoolean),
			col("promo", "BOOLEAN NOT NULL", kindBoolean),
			col("reprint", "BOOLEAN NOT NULL", kindBoolean),
			col("set_code", "VARCHAR(16) NOT NULL", kindString),
			col("textless", "BOOLEAN NOT NULL", kindBoolean),
			col("security_stamp", "VARCHAR(64) NULL", kindString),
			col("watermark", "VARCHAR(128) NULL", kindString),
			col("foil", "BOOLEAN NOT NULL", kindBoolean),
			col("nonfoil", "BOOLEAN NOT NULL", kindBoolean),
			col("mana_cost", "VARCHAR(512) NULL", kindString),
		},
		indexes: []indexSpec{
			idx("idx_mtgnissa_token_set_code", "set_code"),
			idx("idx_mtgnissa_token_scryfall_id", "scryfall_id"),
		},
	},
	{
		table: tableMTGNissaCardPrinting,
		key:   []string{"uuid"},
		columns: []columnSpec{
			col("uuid", "CHAR(36) NOT NULL", kindString),
			col("card_uuid", "CHAR(36) NOT NULL", kindString),
			col("language_code", "VARCHAR(8) NOT NULL", kindString),
			col("scryfall_id", "CHAR(36) NULL", kindString),
			col("printed_name", "VARCHAR(512) NULL", kindString),
			col("printed_text", "LONGTEXT NULL", kindString),
			col("printed_type_line", "VARCHAR(512) NULL", kindString),
		},
		indexes: []indexSpec{
			uniqueIdx("uq_mtgnissa_card_printing_language", "card_uuid", "language_code"),
			idx("idx_mtgnissa_card_printing_scryfall_id", "scryfall_id"),
			idx("idx_mtgnissa_card_printing_language", "language_code"),
			idx("idx_mtgnissa_card_printing_printed_name", "printed_name"),
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
			col("set", "VARCHAR(16) NULL", kindString),
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
			col("set", "VARCHAR(16) NULL", kindString),
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
		table: tableRulingTranslation, key: []string{"ruling_key"},
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
		key:   []string{"oracle_id", "ruling_key", "published_at"},
		columns: []columnSpec{
			col("oracle_id", "CHAR(36) NOT NULL", kindString),
			col("ruling_key", "CHAR(64) NOT NULL", kindRulingKey),
			col("published_at", "DATE NOT NULL", kindDate),
			col("text", "LONGTEXT NOT NULL", kindString),
		},
		indexes: []indexSpec{
			idx("idx_oracle_ruling_oracle", "oracle_id", "published_at"),
			idx("idx_oracle_ruling_rule", "ruling_key", "oracle_id"),
		},
	},
	{
		table: tableOracleLegality,
		key:   []string{"oracle_id"},
		columns: []columnSpec{
			col("oracle_id", "CHAR(36) NOT NULL", kindString),
			col("alchemy_status", "VARCHAR(32) NULL", kindString),
			col("brawl_status", "VARCHAR(32) NULL", kindString),
			col("commander_status", "VARCHAR(32) NULL", kindString),
			col("duel_status", "VARCHAR(32) NULL", kindString),
			col("future_status", "VARCHAR(32) NULL", kindString),
			col("gladiator_status", "VARCHAR(32) NULL", kindString),
			col("historic_status", "VARCHAR(32) NULL", kindString),
			col("competitivebrawl_status", "VARCHAR(32) NULL", kindString),
			col("legacy_status", "VARCHAR(32) NULL", kindString),
			col("modern_status", "VARCHAR(32) NULL", kindString),
			col("oathbreaker_status", "VARCHAR(32) NULL", kindString),
			col("oldschool_status", "VARCHAR(32) NULL", kindString),
			col("pauper_status", "VARCHAR(32) NULL", kindString),
			col("paupercommander_status", "VARCHAR(32) NULL", kindString),
			col("penny_status", "VARCHAR(32) NULL", kindString),
			col("pioneer_status", "VARCHAR(32) NULL", kindString),
			col("predh_status", "VARCHAR(32) NULL", kindString),
			col("premodern_status", "VARCHAR(32) NULL", kindString),
			col("standard_status", "VARCHAR(32) NULL", kindString),
			col("standardbrawl_status", "VARCHAR(32) NULL", kindString),
			col("timeless_status", "VARCHAR(32) NULL", kindString),
			col("vintage_status", "VARCHAR(32) NULL", kindString),
		},
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
			col("code", "VARCHAR(32) NOT NULL", kindString),
			col("name", "VARCHAR(512) NULL", kindString),
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
			col("keyword", "VARCHAR(128) NOT NULL", kindString),
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
		indexes: []indexSpec{idx("idx_scryfall_card_type_lookup", "type_name", "type_group", "card_uuid")},
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
			col("promo_type", "VARCHAR(64) NOT NULL", kindString),
		},
		indexes: []indexSpec{idx("idx_scryfall_card_promo_type_lookup", "promo_type", "card_uuid")},
	},
	{
		table: tableScryfallSet, key: []string{"code"},
		columns: []columnSpec{
			col("code", "VARCHAR(16) NOT NULL", kindString),
			col("name", "VARCHAR(512) NOT NULL", kindString),
			col("released_at", "DATE NULL", kindDate),
			col("set_type", "VARCHAR(32) NOT NULL", kindString),
			col("card_count", "BIGINT UNSIGNED NOT NULL", kindInteger),
			col("keyrune_code", "VARCHAR(64) NULL", kindString),
			col("languages", "VARCHAR(255) NULL", kindStringArray),
			col("block", "VARCHAR(512) NULL", kindString),
			col("parent_code", "VARCHAR(16) NULL", kindString),
		},
		indexes: []indexSpec{
			idx("idx_mtgnissa_set_type", "set_type"),
			idx("idx_mtgnissa_set_released_at", "released_at"),
			idx("idx_mtgnissa_set_parent_code", "parent_code"),
		},
	},
	{
		table: tableCardFlavorMapping, key: []string{"card_uuid", "flavor_id"},
		columns: []columnSpec{
			col("card_uuid", "CHAR(36) NOT NULL", kindString),
			col("flavor_id", "CHAR(36) NOT NULL", kindString),
		},
		indexes: []indexSpec{idx("idx_card_flavor_mapping_flavor", "flavor_id", "card_uuid")},
	},
	{
		table: tableCardOracleMapping, key: []string{"card_uuid", "face_oracle_id"},
		columns: []columnSpec{
			col("card_uuid", "CHAR(36) NOT NULL", kindString),
			col("face_oracle_id", "CHAR(36) NOT NULL", kindString),
		},
		indexes: []indexSpec{idx("idx_card_oracle_mapping_oracle", "face_oracle_id", "card_uuid")},
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
		indexes: []indexSpec{idx("idx_oracle_tag_mapping_tag", "tag_id", "weight", "oracle_id")},
	},
}

var dictionarySpecs = []spec{
	{
		table: tableScryfallColor, key: []string{"code"},
		columns: []columnSpec{
			col("code", "VARCHAR(8) NOT NULL", kindString),
			col("bit_value", "BIGINT UNSIGNED NOT NULL", kindInteger),
			col("name", "VARCHAR(64) NOT NULL", kindString),
			col("description", "VARCHAR(255) NULL", kindString),
			col("sort_order", "SMALLINT UNSIGNED NOT NULL", kindInteger),
		},
		indexes: []indexSpec{uniqueIdx("uq_scryfall_color_bit_value", "bit_value")},
	},
	{
		table: tableScryfallLanguage, key: []string{"code"},
		columns: []columnSpec{
			col("code", "VARCHAR(8) NOT NULL", kindString),
			col("name", "VARCHAR(64) NOT NULL", kindString),
			col("display_name", "VARCHAR(64) NOT NULL", kindString),
			col("sort_order", "SMALLINT UNSIGNED NOT NULL", kindInteger),
		},
	},
	{
		table: tableScryfallLayout, key: []string{"code"},
		columns: []columnSpec{
			col("code", "VARCHAR(32) NOT NULL", kindString),
			col("name", "VARCHAR(64) NOT NULL", kindString),
			col("description", "VARCHAR(255) NULL", kindString),
			col("face_category", "VARCHAR(32) NOT NULL", kindString),
		},
	},
	{
		table: tableScryfallFrame, key: []string{"code"},
		columns: []columnSpec{
			col("code", "VARCHAR(16) NOT NULL", kindString),
			col("name", "VARCHAR(64) NOT NULL", kindString),
			col("description", "VARCHAR(255) NULL", kindString),
		},
	},
	{
		table: tableScryfallFrameEffectDef, key: []string{"code"},
		columns: []columnSpec{
			col("code", "VARCHAR(64) NOT NULL", kindString),
			col("description", "VARCHAR(255) NULL", kindString),
		},
	},
	{
		table: tableScryfallFinish, key: []string{"code"},
		columns: []columnSpec{
			col("code", "VARCHAR(32) NOT NULL", kindString),
			col("bit_value", "BIGINT UNSIGNED NOT NULL", kindInteger),
			col("name", "VARCHAR(64) NOT NULL", kindString),
		},
		indexes: []indexSpec{uniqueIdx("uq_scryfall_finish_bit_value", "bit_value")},
	},
	{
		table: tableScryfallGame, key: []string{"code"},
		columns: []columnSpec{
			col("code", "VARCHAR(32) NOT NULL", kindString),
			col("bit_value", "BIGINT UNSIGNED NOT NULL", kindInteger),
			col("name", "VARCHAR(64) NOT NULL", kindString),
		},
		indexes: []indexSpec{uniqueIdx("uq_scryfall_game_bit_value", "bit_value")},
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
		prefix := "KEY "
		if index.unique {
			prefix = "UNIQUE KEY "
		}
		parts = append(parts, prefix+quote(index.name)+" ("+quotedList(index.columns)+")")
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
