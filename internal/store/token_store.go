package store

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jmoiron/sqlx"

	"mtgnissa/internal/apperror"
	"mtgnissa/internal/model"
)

// TokenStore reads Token Set metadata and faces from a consistent Card DB snapshot.
type TokenStore struct {
	db *sqlx.DB
}

// NewTokenStore creates a read-only Token query store.
func NewTokenStore(db *sqlx.DB) *TokenStore {
	return &TokenStore{db: db}
}

// Read returns the requested Token Set and all of its source faces.
func (s *TokenStore) Read(ctx context.Context, code string) (model.TokenSet, []model.TokenFace, error) {
	tx, err := s.db.BeginTxx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return model.TokenSet{}, nil, err
	}
	defer tx.Rollback()
	const setQuery = `
		SELECT s.code, s.name, s.set_type, s.released_at,
		       COALESCE(st.name, pt.name, s.name) AS display_name
		FROM mtgnissa_set s
		LEFT JOIN (
			SELECT code, MIN(NULLIF(TRIM(name), '')) AS name FROM set_translation GROUP BY code
		) st ON st.code = s.code
		LEFT JOIN (
			SELECT code, MIN(NULLIF(TRIM(name), '')) AS name FROM set_translation GROUP BY code
		) pt ON pt.code = s.parent_code AND s.set_type = 'token'
		WHERE s.code = ?
	`
	var set model.TokenSet
	if err := tx.GetContext(ctx, &set, setQuery, code); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.TokenSet{}, nil, apperror.New(apperror.CodeSetNotFound, "set not found")
		}
		return model.TokenSet{}, nil, err
	}
	if set.SetType != "token" {
		return model.TokenSet{}, nil, apperror.New(apperror.CodeInvalidRequest, "set must be a token set")
	}
	const facesQuery = `
		SELECT uuid, scryfall_id, face_index, name, collector_number, layout
		FROM mtgnissa_token WHERE set_code = ?
	`
	faces := make([]model.TokenFace, 0)
	if err := tx.SelectContext(ctx, &faces, facesQuery, set.Code); err != nil {
		return model.TokenSet{}, nil, err
	}
	if err := tx.Commit(); err != nil {
		return model.TokenSet{}, nil, err
	}
	return set, faces, nil
}
