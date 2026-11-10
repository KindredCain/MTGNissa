package store

import (
	"context"
	"database/sql"
	"time"

	"mtgnissa/internal/model"
)

type oracleContextRow struct {
	Kind        string     `db:"kind"`
	PublishedAt *time.Time `db:"published_at"`
	ID          string     `db:"entry_id"`
	Text        string     `db:"text"`
}

// Detail reads printing attributes, versions, and Oracle entries in one Card DB snapshot.
func (s *CardStore) Detail(ctx context.Context, id string) (model.CardDetailData, error) {
	tx, err := s.db.BeginTxx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return model.CardDetailData{}, err
	}
	defer tx.Rollback()
	rows, err := readPrinting(ctx, tx, id)
	if err != nil {
		return model.CardDetailData{}, err
	}
	card, faces := summarize(rows)
	data := model.CardDetailData{Card: card, Faces: faces, Rulings: []model.Ruling{}, Tags: []model.OracleTagLabel{}}
	condition := " WHERE c.set_code=? AND c.collector_number=? AND c.face_index IN (-1,0)"
	args := []any{card.Set.Code, card.CollectorNumber}
	if card.OracleID != nil {
		condition = " WHERE c.oracle_id=? AND c.face_index IN (-1,0)"
		args = []any{*card.OracleID}
	}
	var versions []faceRow
	if err := tx.SelectContext(ctx, &versions, versionQuery+condition, args...); err != nil {
		return model.CardDetailData{}, err
	}
	for _, row := range versions {
		data.Versions = append(data.Versions, model.CardVersionCandidate{
			Set:             model.SetRef{Code: row.SetCode, Name: row.SetName, DisplayName: row.SetDisplayName, ReleasedAt: row.SetReleasedAt},
			CollectorNumber: row.Number, ReleasedAt: row.ReleasedAt, PrintingUUID: row.UUID, OracleID: row.OracleID,
			Language: model.Language{Code: row.LanguageCode, DisplayName: row.LanguageName, SortOrder: row.LanguageOrder},
		})
	}
	if card.OracleID != nil {
		var entries []oracleContextRow
		if err := tx.SelectContext(ctx, &entries, `
 SELECT 'ruling' AS kind,r.published_at,r.ruling_key AS entry_id,COALESCE(NULLIF(t.translation,''),r.text) AS text
 FROM oracle_ruling r LEFT JOIN ruling_translation t ON t.ruling_key=r.ruling_key
 WHERE r.oracle_id=?
 UNION ALL
 SELECT 'tag' AS kind,NULL AS published_at,t.tag_id AS entry_id,t.label AS text
 FROM oracle_tag_mapping m JOIN oracle_tag t ON t.tag_id=m.tag_id WHERE m.oracle_id=?
 ORDER BY kind ASC,published_at DESC,entry_id ASC`, *card.OracleID, *card.OracleID); err != nil {
			return model.CardDetailData{}, err
		}
		for _, entry := range entries {
			if entry.Kind == "ruling" {
				data.Rulings = append(data.Rulings, model.Ruling{PublishedAt: *entry.PublishedAt, Text: entry.Text})
			} else {
				data.Tags = append(data.Tags, model.OracleTagLabel{ID: entry.ID, Label: entry.Text})
			}
		}
	}
	return data, tx.Commit()
}
