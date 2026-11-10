package store

import (
	"context"
	"database/sql"
	"sort"
	"strings"

	"github.com/jmoiron/sqlx"

	"mtgnissa/internal/model"
)

type deckFaceRow struct {
	faceRow
	ProducedMana *string `db:"produced_mana"`
}

type deckTagRow struct {
	OracleID string `db:"oracle_id"`
	Label    string `db:"label"`
}

// DeckReferences resolves only valid main-face printing IDs without replacing their identities.
func (s *CardStore) DeckReferences(ctx context.Context, ids []string) (map[string]model.DeckCardData, error) {
	tx, err := s.db.BeginTxx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	result, err := deckCardsByPrinting(ctx, tx, ids)
	if err != nil {
		return nil, err
	}
	return result, tx.Commit()
}

// DeckPrintings reads candidate identities and dates in the active Card DB snapshot.
func (s *CardTx) DeckPrintings(ctx context.Context, oracles []string) ([]model.DeckPrinting, error) {
	result := make([]model.DeckPrinting, 0)
	if len(oracles) == 0 {
		return result, nil
	}
	query, args, err := sqlx.In(`SELECT p.uuid AS printing_uuid,c.oracle_id,c.released_at
 FROM mtgnissa_card_printing p JOIN mtgnissa_card c ON c.uuid=p.card_uuid
 WHERE c.oracle_id IN (?) AND c.face_index IN (-1,0)`, oracles)
	if err != nil {
		return nil, err
	}
	if err := s.tx.SelectContext(ctx, &result, query, args...); err != nil {
		return nil, err
	}
	return result, nil
}

// DeckCards reads the requested printings in the active Card DB snapshot.
func (s *CardTx) DeckCards(ctx context.Context, ids []string) (map[string]model.DeckCardData, error) {
	return deckCardsByPrinting(ctx, s.tx, ids)
}

func deckCardsByPrinting(ctx context.Context, tx *sqlx.Tx, ids []string) (map[string]model.DeckCardData, error) {
	result := make(map[string]model.DeckCardData)
	for start := 0; start < len(ids); start += 500 {
		query, args, err := sqlx.In(strings.Replace(printingQuery, "SELECT ", "SELECT c.produced_mana,", 1)+" WHERE p.uuid IN (?)", ids[start:min(start+500, len(ids))])
		if err != nil {
			return nil, err
		}
		var rows []deckFaceRow
		if err := tx.SelectContext(ctx, &rows, query, args...); err != nil {
			return nil, err
		}
		groups := make(map[[2]string][]deckFaceRow)
		for _, row := range rows {
			key := [2]string{row.SourceID, row.LanguageCode}
			groups[key] = append(groups[key], row)
		}
		for _, group := range groups {
			sort.Slice(group, func(i, j int) bool {
				if group[i].Index != group[j].Index {
					return group[i].Index < group[j].Index
				}
				return group[i].UUID < group[j].UUID
			})
			faces := make([]faceRow, 0, len(group))
			for _, row := range group {
				faces = append(faces, row.faceRow)
			}
			card, texts := summarize(faces)
			produced := []string{}
			if group[0].ProducedMana != nil && *group[0].ProducedMana != "" {
				produced = strings.Split(*group[0].ProducedMana, ",")
			}
			result[card.PrintingUUID] = model.DeckCardData{Card: card, Faces: texts, ProducedMana: produced, OracleTags: []string{}}
		}
	}
	return result, nil
}

// DeckTags resolves all functional labels without filtering mapping weights.
func (s *CardStore) DeckTags(ctx context.Context, oracles []string) (map[string][]string, error) {
	result := make(map[string][]string)
	for start := 0; start < len(oracles); start += 500 {
		query, args, err := sqlx.In(`SELECT m.oracle_id,t.label FROM oracle_tag_mapping m JOIN oracle_tag t ON t.tag_id=m.tag_id WHERE m.oracle_id IN (?) ORDER BY m.oracle_id,t.label,t.tag_id`, oracles[start:min(start+500, len(oracles))])
		if err != nil {
			return nil, err
		}
		var rows []deckTagRow
		if err := s.db.SelectContext(ctx, &rows, query, args...); err != nil {
			return nil, err
		}
		for _, row := range rows {
			labels := result[row.OracleID]
			if len(labels) == 0 || labels[len(labels)-1] != row.Label {
				result[row.OracleID] = append(labels, row.Label)
			}
		}
	}
	return result, nil
}

// DeckNames resolves exact canonical English names without using printing-language names.
func (s *CardStore) DeckNames(ctx context.Context, names []string) ([]model.DeckName, error) {
	result := []model.DeckName{}
	for start := 0; start < len(names); start += 500 {
		query, args, err := sqlx.In(`SELECT name,NULLIF(oracle_id,'') AS oracle_id FROM mtgnissa_card
 WHERE name IN (?) AND face_index IN (-1,0) GROUP BY name,oracle_id ORDER BY name,oracle_id`, names[start:min(start+500, len(names))])
		if err != nil {
			return nil, err
		}
		var rows []cardNameRow
		if err := s.db.SelectContext(ctx, &rows, query, args...); err != nil {
			return nil, err
		}
		for _, row := range rows {
			result = append(result, model.DeckName{Name: row.Name, OracleID: row.OracleID})
		}
	}
	return result, nil
}
