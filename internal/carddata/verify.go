package carddata

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type associationSpec struct {
	childTable   tableName
	childColumn  string
	parentTable  tableName
	parentColumn string
}

var associationSpecs = []associationSpec{
	{childTable: tableScryfallCard, childColumn: "layout", parentTable: tableScryfallLayout, parentColumn: "code"},
	{childTable: tableScryfallCard, childColumn: "frame", parentTable: tableScryfallFrame, parentColumn: "code"},
	{childTable: tableScryfallCard, childColumn: "set_code", parentTable: tableScryfallSet, parentColumn: "code"},
	{childTable: tableMTGNissaToken, childColumn: "set_code", parentTable: tableScryfallSet, parentColumn: "code"},
	{childTable: tableMTGNissaCardPrinting, childColumn: "card_uuid", parentTable: tableScryfallCard, parentColumn: "uuid"},
	{childTable: tableMTGNissaCardPrinting, childColumn: "language_code", parentTable: tableScryfallLanguage, parentColumn: "code"},
	{childTable: tableScryfallSet, childColumn: "parent_code", parentTable: tableScryfallSet, parentColumn: "code"},
	{childTable: tableCardTranslation, childColumn: "card_id", parentTable: tableScryfallCard, parentColumn: "uuid"},
	{childTable: tableScryfallOracleRuling, childColumn: "oracle_id", parentTable: tableScryfallCard, parentColumn: "oracle_id"},
	{childTable: tableOracleLegality, childColumn: "oracle_id", parentTable: tableScryfallCard, parentColumn: "oracle_id"},
	{childTable: tableCardFlavorMapping, childColumn: "card_uuid", parentTable: tableScryfallCard, parentColumn: "uuid"},
	{childTable: tableCardFlavorMapping, childColumn: "flavor_id", parentTable: tableFlavorTranslation, parentColumn: "flavor_id"},
	{childTable: tableCardOracleMapping, childColumn: "card_uuid", parentTable: tableScryfallCard, parentColumn: "uuid"},
	{childTable: tableCardOracleMapping, childColumn: "face_oracle_id", parentTable: tableOracleTranslation, parentColumn: "face_oracle_id"},
	{childTable: tableScryfallCardKeyword, childColumn: "card_uuid", parentTable: tableScryfallCard, parentColumn: "uuid"},
	{childTable: tableScryfallCardType, childColumn: "card_uuid", parentTable: tableScryfallCard, parentColumn: "uuid"},
	{childTable: tableScryfallCardFrameEffect, childColumn: "card_uuid", parentTable: tableScryfallCard, parentColumn: "uuid"},
	{childTable: tableScryfallCardFrameEffect, childColumn: "frame_effect", parentTable: tableScryfallFrameEffectDef, parentColumn: "code"},
	{childTable: tableScryfallCardPromoType, childColumn: "card_uuid", parentTable: tableScryfallCard, parentColumn: "uuid"},
	{childTable: tableOracleTagRelation, childColumn: "parent_tag_id", parentTable: tableOracleTag, parentColumn: "tag_id"},
	{childTable: tableOracleTagRelation, childColumn: "child_tag_id", parentTable: tableOracleTag, parentColumn: "tag_id"},
	{childTable: tableOracleTagging, childColumn: "tag_id", parentTable: tableOracleTag, parentColumn: "tag_id"},
}

func (m *Manager) verifyImportedData(ctx context.Context, id string, results []FileResult, tableNames map[tableName]string, state *importState) *TaskError {
	m.stage(id, StageVerifying)
	startedAt := time.Now()
	for _, table := range []tableName{
		tableScryfallCard, tableMTGNissaToken, tableScryfallSet, tableMTGNissaCardPrinting,
		tableScryfallOracleRuling, tableOracleLegality, tableScryfallCardKeyword,
		tableScryfallCardType, tableScryfallCardFrameEffect, tableScryfallCardPromoType,
	} {
		var count int64
		if err := m.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+quote(tableNames[table])).Scan(&count); err != nil {
			return stageFail(StageVerifying, "AllPrintings.json", 0, err)
		}
		if count != state.insertedRows[table] {
			return stageFail(StageVerifying, "AllPrintings.json", 0, fmt.Errorf("table %s row count mismatch: expected %d, inserted %d", table, state.insertedRows[table], count))
		}
	}
	for table, expected := range map[tableName]int64{
		tableCardFlavorMapping: state.flavorMappingCount,
		tableCardOracleMapping: state.oracleMappingCount,
	} {
		var count int64
		if err := m.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+quote(tableNames[table])).Scan(&count); err != nil {
			return stageFail(StageVerifying, "scryfall_card.json", 0, err)
		}
		if count != expected {
			return stageFail(StageVerifying, "scryfall_card.json", 0, fmt.Errorf("table %s row count mismatch: expected %d, inserted %d", table, expected, count))
		}
	}
	for i, s := range sourceSpecs {
		if s.kind != sourceRows {
			continue
		}
		tableSpec := mustTableSpec(s.table)
		tableStartedAt := time.Now()
		expectedRows := results[i].InsertedRows
		m.setProgress(s.file, 0, expectedRows)
		var count int64
		if err := m.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+quote(tableNames[s.table])).Scan(&count); err != nil {
			return stageFail(StageVerifying, s.file, 0, err)
		}
		if (!tableSpec.allowExtraRows && count != expectedRows) || (tableSpec.allowExtraRows && count < expectedRows) {
			return stageFail(StageVerifying, s.file, 0, fmt.Errorf("row count mismatch: expected %d, inserted %d", expectedRows, count))
		}
		m.setProgress(s.file, count, expectedRows)
		m.log.Info("card data table verification completed", "task_id", id, "table", s.table, "rows", count, "duration", time.Since(tableStartedAt))
	}
	associationStartedAt := time.Now()
	if err := validateImportedAssociations(ctx, m.db, tableNames); err != nil {
		return stageFail(StageVerifying, "", 0, err)
	}
	m.log.Info("card data association validation completed", "task_id", id, "association_count", len(associationSpecs), "duration", time.Since(associationStartedAt))
	m.stageCompleted(id, StageVerifying, "table_count", len(sourceSpecs), "association_count", len(associationSpecs), "duration", time.Since(startedAt))
	return nil
}

func validateImportedAssociations(ctx context.Context, db *sql.DB, tableNames map[tableName]string) error {
	for _, association := range associationSpecs {
		var orphan string
		err := db.QueryRowContext(ctx, associationCheckSQL(association, tableNames)).Scan(&orphan)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return fmt.Errorf("check association %s.%s -> %s.%s: %w", association.childTable, association.childColumn, association.parentTable, association.parentColumn, err)
		}
		return fmt.Errorf("orphan association %s.%s -> %s.%s: value %q", association.childTable, association.childColumn, association.parentTable, association.parentColumn, orphan)
	}
	return nil
}

func associationCheckSQL(association associationSpec, tableNames map[tableName]string) string {
	childColumn := quote(association.childColumn)
	parentColumn := quote(association.parentColumn)
	return "SELECT child." + childColumn +
		" FROM " + quote(tableNames[association.childTable]) + " AS child" +
		" LEFT JOIN " + quote(tableNames[association.parentTable]) + " AS parent" +
		" ON parent." + parentColumn + " = child." + childColumn +
		" WHERE child." + childColumn + " IS NOT NULL" +
		" AND parent." + parentColumn + " IS NULL LIMIT 1"
}
