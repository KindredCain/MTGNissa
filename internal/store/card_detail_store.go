package store

import (
	"context"
	"database/sql"
	"time"

	"mtgnissa/internal/apperror"
	"mtgnissa/internal/model"
)

type oracleContextRow struct {
	Kind        string     `db:"kind"`
	PublishedAt *time.Time `db:"published_at"`
	ID          string     `db:"entry_id"`
	Text        string     `db:"text"`
}

const previewQuery = `SELECT c.face_index,
 COALESCE(NULLIF(c.face_name,''),c.name) AS face_name,
 COALESCE(NULLIF(ct.face_name,''),NULLIF(ct.name,''),
  (SELECT MIN(NULLIF(t.translated_name,'')) FROM card_oracle_translation_mapping m
   JOIN oracle_translation t ON t.face_oracle_id=m.face_oracle_id WHERE m.card_uuid=c.uuid),
  NULLIF(c.face_name,''),c.name) AS display_name,
 c.type_line,COALESCE(NULLIF(ct.type_line,''),
  (SELECT MIN(NULLIF(t.translated_type,'')) FROM card_oracle_translation_mapping m
   JOIN oracle_translation t ON t.face_oracle_id=m.face_oracle_id WHERE m.card_uuid=c.uuid),c.type_line) AS display_type_line,
 c.oracle_text,COALESCE(NULLIF(ct.text,''),
  (SELECT MIN(NULLIF(t.translated_text,'')) FROM card_oracle_translation_mapping m
   JOIN oracle_translation t ON t.face_oracle_id=m.face_oracle_id WHERE m.card_uuid=c.uuid),c.oracle_text) AS display_oracle_text
 FROM mtgnissa_card_printing p
 JOIN mtgnissa_card main ON main.uuid=p.card_uuid AND main.face_index IN (-1,0)
 JOIN mtgnissa_card c ON c.scryfall_id=main.scryfall_id
 LEFT JOIN card_translation ct ON ct.card_id=c.uuid
 WHERE p.uuid=? ORDER BY c.face_index,c.uuid`

// Preview reads one printing's face text without versions, rulings, or personal data.
func (s *CardStore) Preview(ctx context.Context, id string) ([]model.CardPreviewFace, error) {
	var rows []faceRow
	if err := s.db.SelectContext(ctx, &rows, previewQuery, id); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, apperror.New(apperror.CodeCardNotFound, "card printing not found or not a main-face entry")
	}
	faces := make([]model.CardPreviewFace, 0, len(rows))
	for _, row := range rows {
		faces = append(faces, model.CardPreviewFace{
			FaceIndex: row.Index, Name: row.FaceName, DisplayName: row.DisplayName,
			TypeLine: row.TypeLine, DisplayTypeLine: row.DisplayTypeLine,
			OracleText: row.Text, DisplayOracleText: row.DisplayText,
		})
	}
	return faces, nil
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
			Language: model.Language{Code: row.LanguageCode, SortOrder: row.LanguageOrder},
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
