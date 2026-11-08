package store

import (
	"context"

	"mtgnissa/internal/model"
)

type cardNameRow struct {
	Name     string  `db:"name"`
	OracleID *string `db:"oracle_id"`
}

// DeckSearch finds logical candidates using literal printed-name contains matching.
func (s *CardStore) DeckSearch(ctx context.Context, query string) ([]model.DeckName, error) {
	var rows []cardNameRow
	statement := `SELECT MIN(main.name) AS name,main.oracle_id FROM mtgnissa_card_printing p
 JOIN mtgnissa_card c ON c.uuid=p.card_uuid
 JOIN mtgnissa_card main ON main.scryfall_id=c.scryfall_id AND main.face_index IN (-1,0)
 WHERE NULLIF(main.oracle_id,'') IS NOT NULL`
	args := []any{}
	if query != "" {
		statement += " AND p.printed_name LIKE ? ESCAPE '='"
		args = append(args, "%"+escapeLike(query)+"%")
	}
	statement += " GROUP BY main.oracle_id ORDER BY LOWER(MIN(main.name)),main.oracle_id"
	if err := s.db.SelectContext(ctx, &rows, statement, args...); err != nil {
		return nil, err
	}
	result := make([]model.DeckName, 0, len(rows))
	for _, row := range rows {
		result = append(result, model.DeckName{Name: row.Name, OracleID: row.OracleID})
	}
	return result, nil
}

// DeckNameSuggestions returns at most five case-insensitively distinct printed names.
func (s *CardStore) DeckNameSuggestions(ctx context.Context, query string) ([]string, error) {
	var names []string
	err := s.db.SelectContext(ctx, &names, `SELECT CONVERT(MIN(BINARY p.printed_name) USING utf8mb4) AS name FROM mtgnissa_card_printing p
 JOIN mtgnissa_card c ON c.uuid=p.card_uuid WHERE NULLIF(c.oracle_id,'') IS NOT NULL AND TRIM(p.printed_name)<>''
 AND p.printed_name LIKE ? ESCAPE '=' GROUP BY LOWER(p.printed_name)
 ORDER BY LOWER(CONVERT(MIN(BINARY p.printed_name) USING utf8mb4)),MIN(BINARY p.printed_name) LIMIT 5`, escapeLike(query)+"%")
	if names == nil {
		names = []string{}
	}
	return names, err
}
