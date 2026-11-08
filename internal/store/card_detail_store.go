package store

import (
	"context"
	"database/sql"
	"sort"
	"strings"
	"time"

	"mtgnissa/internal/model"
)

type oracleContextRow struct {
	Kind        string     `db:"kind"`
	PublishedAt *time.Time `db:"published_at"`
	ID          string     `db:"entry_id"`
	Text        string     `db:"text"`
}

type personalStateRow struct {
	Normal        uint64 `db:"normal_quantity"`
	Foil          uint64 `db:"foil_quantity"`
	LogicalNormal uint64 `db:"logical_normal"`
	LogicalFoil   uint64 `db:"logical_foil"`
	Included      bool   `db:"included"`
}

// Detail reads card metadata, versions, rulings, and tags from a consistent Card DB snapshot.
func (s *CardStore) Detail(ctx context.Context, id string) (model.CardDetail, error) {
	tx, err := s.db.BeginTxx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return model.CardDetail{}, err
	}
	defer tx.Rollback()
	rows, err := readPrinting(ctx, tx, id)
	if err != nil {
		return model.CardDetail{}, err
	}
	card, faces := summarize(rows)
	d := model.CardDetail{Card: card, Faces: faces, Rulings: []model.Ruling{}, Tags: []model.OracleTag{}, Languages: []model.CardLanguageOption{}, OtherVersions: []model.CardVersion{}, Decks: []model.DeckUsage{}}
	var versions []faceRow
	condition := " WHERE c.set_code=? AND c.collector_number=? AND c.face_index IN (-1,0)"
	args := []any{card.Set.Code, card.CollectorNumber}
	if card.OracleID != nil {
		condition = " WHERE c.oracle_id=? AND c.face_index IN (-1,0)"
		args = []any{*card.OracleID}
	}
	err = tx.SelectContext(ctx, &versions, versionQuery+condition, args...)
	if err != nil {
		return model.CardDetail{}, err
	}
	groups := make(map[[2]string]*model.CardVersion)
	for _, row := range versions {
		key := [2]string{row.SetCode, row.Number}
		option := model.CardLanguageOption{PrintingUUID: row.UUID, OracleID: row.OracleID, Language: model.Language{Code: row.LanguageCode, DisplayName: row.LanguageName, SortOrder: row.LanguageOrder}}
		if key == [2]string{card.Set.Code, card.CollectorNumber} {
			d.Languages = append(d.Languages, option)
			continue
		}
		v := groups[key]
		if v == nil {
			v = &model.CardVersion{Set: model.SetRef{Code: row.SetCode, Name: row.SetName, DisplayName: row.SetDisplayName, ReleasedAt: row.SetReleasedAt}, CollectorNumber: row.Number, ReleasedAt: row.ReleasedAt, Languages: []model.CardLanguageOption{}}
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
			return model.CardDetail{}, err
		}
		for _, entry := range entries {
			if entry.Kind == "ruling" {
				d.Rulings = append(d.Rulings, model.Ruling{PublishedAt: *entry.PublishedAt, Text: entry.Text})
			} else {
				label := strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(entry.Text)
				d.Tags = append(d.Tags, model.OracleTag{ID: entry.ID, Query: `otag:"` + label + `"`})
			}
		}
	}
	err = tx.Commit()
	if err != nil {
		return model.CardDetail{}, err
	}
	return d, nil
}

func sortLanguages(options []model.CardLanguageOption) {
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

func minPrinting(options []model.CardLanguageOption) string {
	minimum := options[0].PrintingUUID
	for _, option := range options[1:] {
		if option.PrintingUUID < minimum {
			minimum = option.PrintingUUID
		}
	}
	return minimum
}

// FillCardContext combines ownership, wishlist, and usage in the active application snapshot.
func FillCardContext(ctx context.Context, stores *TxStores, detail *model.CardDetail) error {
	var state personalStateRow
	if err := stores.tx.GetContext(ctx, &state, `
 SELECT COALESCE(c.normal_quantity,0) AS normal_quantity,COALESCE(c.foil_quantity,0) AS foil_quantity,
 CASE WHEN ? IS NULL THEN COALESCE(c.normal_quantity,0) ELSE COALESCE(logical.normal_quantity,0) END AS logical_normal,
 CASE WHEN ? IS NULL THEN COALESCE(c.foil_quantity,0) ELSE COALESCE(logical.foil_quantity,0) END AS logical_foil,
 EXISTS(SELECT 1 FROM wishlist_item w WHERE w.printing_uuid=requested.printing_uuid) AS included
 FROM (SELECT ? AS printing_uuid) requested
 LEFT JOIN collection_item c ON c.printing_uuid=requested.printing_uuid
 LEFT JOIN (
  SELECT SUM(c.normal_quantity) AS normal_quantity,SUM(c.foil_quantity) AS foil_quantity
  FROM card_print_ref r JOIN collection_item c ON c.printing_uuid=r.printing_uuid WHERE r.oracle_id=?
 ) logical ON 1=1`, detail.Card.OracleID, detail.Card.OracleID, detail.Card.PrintingUUID, detail.Card.OracleID); err != nil {
		return err
	}
	detail.Collection = model.Quantity{Normal: state.Normal, Foil: state.Foil}
	detail.LogicalCollection = model.Quantity{Normal: state.LogicalNormal, Foil: state.LogicalFoil}
	detail.Included = state.Included
	ids := make([]string, 0)
	for _, option := range detail.Languages {
		ids = append(ids, option.PrintingUUID)
	}
	for _, version := range detail.OtherVersions {
		for _, option := range version.Languages {
			ids = append(ids, option.PrintingUUID)
		}
	}
	owned, err := stores.Collection.quantities(ctx, ids)
	if err != nil {
		return err
	}
	for i := range detail.Languages {
		detail.Languages[i].Quantity = owned[detail.Languages[i].PrintingUUID]
	}
	for i := range detail.OtherVersions {
		version := &detail.OtherVersions[i]
		version.Quantity = model.Quantity{}
		for j := range version.Languages {
			quantity := owned[version.Languages[j].PrintingUUID]
			version.Languages[j].Quantity = quantity
			version.Quantity.Normal += quantity.Normal
			version.Quantity.Foil += quantity.Foil
		}
	}
	detail.Decks, err = stores.Decks.Usage(ctx, detail.Card.OracleID)
	return err
}
