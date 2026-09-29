package carddata

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type associationSpec struct {
	childTable, childColumn   string
	parentTable, parentColumn string
}

var associationSpecs = []associationSpec{
	{childTable: "scryfall_card", childColumn: "lang", parentTable: "scryfall_language", parentColumn: "code"},
	{childTable: "scryfall_card", childColumn: "layout", parentTable: "scryfall_layout", parentColumn: "code"},
	{childTable: "scryfall_card", childColumn: "frame", parentTable: "scryfall_frame", parentColumn: "code"},
	{childTable: "scryfall_card", childColumn: "set_id", parentTable: "scryfall_set", parentColumn: "set_id"},
	{childTable: "zhs_card", childColumn: "card_id", parentTable: "scryfall_card", parentColumn: "uuid"},
	{childTable: "zhs_flavor", childColumn: "flavor_id", parentTable: "scryfall_card", parentColumn: "flavor_id"},
	{childTable: "zhs_oracle", childColumn: "face_oracle_id", parentTable: "scryfall_card", parentColumn: "face_oracle_id"},
	{childTable: "zhs_oracle", childColumn: "oracle_id", parentTable: "scryfall_card", parentColumn: "oracle_id"},
	{childTable: "zhs_set", childColumn: "set_id", parentTable: "scryfall_set", parentColumn: "set_id"},
	{childTable: "scryfall_oracle_ruling", childColumn: "oracle_id", parentTable: "scryfall_card", parentColumn: "oracle_id"},
	{childTable: "scryfall_oracle_ruling", childColumn: "ruling_key", parentTable: "zhs_ruling", parentColumn: "ruling_key"},
	{childTable: "zhs_ruling", childColumn: "ruling_key", parentTable: "scryfall_oracle_ruling", parentColumn: "ruling_key"},
	{childTable: "scryfall_card_keyword", childColumn: "card_uuid", parentTable: "scryfall_card", parentColumn: "uuid"},
	{childTable: "scryfall_card_type", childColumn: "card_uuid", parentTable: "scryfall_card", parentColumn: "uuid"},
	{childTable: "scryfall_card_frame_effect", childColumn: "card_uuid", parentTable: "scryfall_card", parentColumn: "uuid"},
	{childTable: "scryfall_card_frame_effect", childColumn: "frame_effect", parentTable: "scryfall_frame_effect", parentColumn: "code"},
	{childTable: "scryfall_card_promo_type", childColumn: "card_uuid", parentTable: "scryfall_card", parentColumn: "uuid"},
	{childTable: "oracle_tag_relation", childColumn: "parent_tag_id", parentTable: "oracle_tag", parentColumn: "tag_id"},
	{childTable: "oracle_tag_relation", childColumn: "child_tag_id", parentTable: "oracle_tag", parentColumn: "tag_id"},
	{childTable: "oracle_tagging", childColumn: "tag_id", parentTable: "oracle_tag", parentColumn: "tag_id"},
	{childTable: "oracle_tagging", childColumn: "oracle_id", parentTable: "scryfall_card", parentColumn: "oracle_id"},
}

func (m *Manager) verifyImportedData(ctx context.Context, id string, results []FileResult, tableNames map[string]string) *TaskError {
	m.stage(id, StageVerifying)
	startedAt := time.Now()
	for i, s := range specs {
		tableStartedAt := time.Now()
		m.setProgress(s.file, 0, results[i].ReadRows)
		var count int64
		if err := m.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+quote(tableNames[s.table])).Scan(&count); err != nil {
			return stageFail(StageVerifying, s.file, 0, err)
		}
		if (!s.allowExtraRows && count != results[i].ReadRows) || (s.allowExtraRows && count < results[i].ReadRows) {
			return stageFail(StageVerifying, s.file, 0, fmt.Errorf("row count mismatch: read %d, inserted %d", results[i].ReadRows, count))
		}
		m.setProgress(s.file, count, results[i].ReadRows)
		m.log.Info("card data table verification completed", "task_id", id, "table", s.table, "rows", count, "duration", time.Since(tableStartedAt))
	}
	associationStartedAt := time.Now()
	if err := validateImportedAssociations(ctx, m.db, tableNames); err != nil {
		return stageFail(StageVerifying, "", 0, err)
	}
	m.log.Info("card data association validation completed", "task_id", id, "association_count", len(associationSpecs), "duration", time.Since(associationStartedAt))
	m.stageCompleted(id, StageVerifying, "table_count", len(specs), "association_count", len(associationSpecs), "duration", time.Since(startedAt))
	return nil
}

func validateImportedAssociations(ctx context.Context, db *sql.DB, tableNames map[string]string) error {
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

func associationCheckSQL(association associationSpec, tableNames map[string]string) string {
	childColumn := quote(association.childColumn)
	parentColumn := quote(association.parentColumn)
	return "SELECT child." + childColumn +
		" FROM " + quote(tableNames[association.childTable]) + " AS child" +
		" LEFT JOIN " + quote(tableNames[association.parentTable]) + " AS parent" +
		" ON parent." + parentColumn + " = child." + childColumn +
		" WHERE child." + childColumn + " IS NOT NULL" +
		" AND parent." + parentColumn + " IS NULL LIMIT 1"
}
