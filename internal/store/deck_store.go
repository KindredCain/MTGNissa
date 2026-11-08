package store

import (
	"context"

	"github.com/jmoiron/sqlx"

	"mtgnissa/internal/model"
)

// DeckTx queries saved decks within the caller's application snapshot.
type DeckTx struct {
	tx *sqlx.Tx
}

type deckUsageRow struct {
	ID       uint64 `db:"id"`
	Name     string `db:"name"`
	Quantity uint64 `db:"quantity"`
	Section  string `db:"section_type"`
}

// Usage returns a logical card's saved deck usage, excluding maybeboard quantities.
func (s *DeckTx) Usage(ctx context.Context, oracle *string) ([]model.DeckUsage, error) {
	result := []model.DeckUsage{}
	if oracle == nil {
		return result, nil
	}
	var rows []deckUsageRow
	if err := s.tx.SelectContext(ctx, &rows, `SELECT d.id,d.name,c.quantity,c.section_type FROM deck_card c JOIN deck d ON d.id=c.deck_id
 WHERE c.oracle_id=? AND c.section_type IN ('commander','mainboard','sideboard')
 ORDER BY d.updated_at DESC,d.id ASC,FIELD(c.section_type,'commander','mainboard','sideboard')`, *oracle); err != nil {
		return nil, err
	}
	positions := make(map[uint64]int)
	for _, row := range rows {
		index, ok := positions[row.ID]
		if !ok {
			index = len(result)
			positions[row.ID] = index
			result = append(result, model.DeckUsage{ID: row.ID, Name: row.Name, Sections: []string{}})
		}
		result[index].Quantity += row.Quantity
		result[index].Sections = append(result[index].Sections, row.Section)
	}
	return result, nil
}
