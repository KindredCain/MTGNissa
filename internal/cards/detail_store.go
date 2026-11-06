package cards

import (
	"context"
	"database/sql"
	"sort"
	"strings"
	"time"
)

type oracleContextRow struct {
	Kind        string     `db:"kind"`
	PublishedAt *time.Time `db:"published_at"`
	ID          string     `db:"entry_id"`
	Text        string     `db:"text"`
}

func (s *CardStore) detail(ctx context.Context, id string) (Detail, error) {
	tx, err := s.db.BeginTxx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return Detail{}, err
	}
	defer tx.Rollback()
	rows, err := readPrinting(ctx, tx, id)
	if err != nil {
		return Detail{}, err
	}
	card, faces := summarize(rows)
	d := Detail{Card: card, Faces: faces, Rulings: []Ruling{}, Tags: []Tag{}, Languages: []LanguageOption{}, OtherVersions: []Version{}, Decks: []DeckUsage{}}
	var versions []faceRow
	condition := " WHERE c.set_code=? AND c.collector_number=? AND c.face_index IN (-1,0)"
	args := []any{card.Set.Code, card.CollectorNumber}
	if card.OracleID != nil {
		condition = " WHERE c.oracle_id=? AND c.face_index IN (-1,0)"
		args = []any{*card.OracleID}
	}
	err = tx.SelectContext(ctx, &versions, versionQuery+condition, args...)
	if err != nil {
		return Detail{}, err
	}
	groups := make(map[[2]string]*Version)
	for _, row := range versions {
		key := [2]string{row.SetCode, row.Number}
		option := LanguageOption{PrintingUUID: row.UUID, OracleID: row.OracleID, Language: Language{Code: row.LanguageCode, DisplayName: row.LanguageName, SortOrder: row.LanguageOrder}}
		if key == [2]string{card.Set.Code, card.CollectorNumber} {
			d.Languages = append(d.Languages, option)
			continue
		}
		v := groups[key]
		if v == nil {
			v = &Version{Set: Set{Code: row.SetCode, Name: row.SetName, DisplayName: row.SetDisplayName, ReleasedAt: row.SetReleasedAt}, CollectorNumber: row.Number, ReleasedAt: row.ReleasedAt, Languages: []LanguageOption{}}
			groups[key] = v
		}
		v.Languages = append(v.Languages, option)
	}
	sortLanguages(d.Languages)
	for _, v := range groups {
		sortLanguages(v.Languages)
		d.OtherVersions = append(d.OtherVersions, *v)
	}
	sort.Slice(d.OtherVersions, func(i, j int) bool {
		a, b := d.OtherVersions[i], d.OtherVersions[j]
		if a.ReleasedAt == nil && b.ReleasedAt != nil {
			return false
		}
		if a.ReleasedAt != nil && b.ReleasedAt == nil {
			return true
		}
		if a.ReleasedAt != nil && !a.ReleasedAt.Equal(*b.ReleasedAt) {
			return a.ReleasedAt.After(*b.ReleasedAt)
		}
		if a.Set.Code != b.Set.Code {
			return a.Set.Code < b.Set.Code
		}
		if a.CollectorNumber != b.CollectorNumber {
			return a.CollectorNumber < b.CollectorNumber
		}
		return minPrinting(a.Languages) < minPrinting(b.Languages)
	})
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
			return Detail{}, err
		}
		for _, entry := range entries {
			if entry.Kind == "ruling" {
				d.Rulings = append(d.Rulings, Ruling{PublishedAt: *entry.PublishedAt, Text: entry.Text})
			} else {
				label := strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(entry.Text)
				d.Tags = append(d.Tags, Tag{ID: entry.ID, Query: `otag:"` + label + `"`})
			}
		}
	}
	err = tx.Commit()
	if err != nil {
		return Detail{}, err
	}
	return d, nil
}

func sortLanguages(options []LanguageOption) {
	sort.Slice(options, func(i, j int) bool {
		if options[i].Language.SortOrder != options[j].Language.SortOrder {
			return options[i].Language.SortOrder < options[j].Language.SortOrder
		}
		if options[i].Language.Code != options[j].Language.Code {
			return options[i].Language.Code < options[j].Language.Code
		}
		return options[i].PrintingUUID < options[j].PrintingUUID
	})
}

func minPrinting(options []LanguageOption) string {
	minimum := options[0].PrintingUUID
	for _, option := range options[1:] {
		if option.PrintingUUID < minimum {
			minimum = option.PrintingUUID
		}
	}
	return minimum
}
