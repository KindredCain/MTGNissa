package sets

import (
	"context"
	"database/sql"
)

// Options reads the language and root Set type filters from one consistent snapshot.
func (s *Store) Options(ctx context.Context) (Options, error) {
	tx, err := s.db.BeginTxx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return Options{}, err
	}
	defer tx.Rollback()

	languages, err := queryLanguages(ctx, tx)
	if err != nil {
		return Options{}, err
	}
	const query = `
		SELECT set_type
		FROM mtgnissa_set
		WHERE parent_code IS NULL AND set_type <> '' AND set_type <> 'token'
		GROUP BY set_type
		ORDER BY MIN(code) ASC, set_type ASC
	`
	setTypes := make([]string, 0)
	if err := tx.SelectContext(ctx, &setTypes, query); err != nil {
		return Options{}, err
	}
	if err := tx.Commit(); err != nil {
		return Options{}, err
	}
	return Options{Languages: languages, SetTypes: setTypes}, nil
}
