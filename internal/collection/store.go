package collection

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"

	"mtgnissa/internal/cards"
)

// Store reads personal ownership and batches matching metadata from the Card DB.
type Store struct {
	app   *sqlx.DB
	card  *sqlx.DB
	faces *cards.CardStore
}

type ownedRow struct {
	UUID   string  `db:"printing_uuid"`
	Oracle *string `db:"oracle_id"`
	Set    string  `db:"set_code"`
	Number string  `db:"collector_number"`
	Lang   string  `db:"lang"`
	cards.Quantity
}

type metadataRow struct {
	UUID         string     `db:"printing_uuid"`
	Oracle       *string    `db:"oracle_id"`
	Name         string     `db:"name"`
	Number       string     `db:"collector_number"`
	Released     *time.Time `db:"released_at"`
	CMC          float64    `db:"cmc"`
	Colors       uint64     `db:"colors_mask"`
	Rarity       string     `db:"rarity"`
	SetCode      string     `db:"set_code"`
	SetName      string     `db:"set_name"`
	SetDisplay   string     `db:"set_display_name"`
	SetReleased  *time.Time `db:"set_released_at"`
	LanguageCode string     `db:"language_code"`
	LanguageName string     `db:"language_name"`
	LanguageSort int        `db:"language_order"`
}

// NewStore creates a collection reader using separate application and card databases.
func NewStore(app, card *sqlx.DB) *Store {
	return &Store{app: app, card: card, faces: cards.NewCardStore(card)}
}

// Load reads only owned printings, without scanning the entire card catalogue.
func (s *Store) Load(ctx context.Context) ([]record, error) {
	tx, err := s.app.BeginTxx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var owned []ownedRow
	if err := tx.SelectContext(ctx, &owned, `SELECT c.printing_uuid,c.normal_quantity,c.foil_quantity,
 r.oracle_id,r.set_code,r.collector_number,r.lang
 FROM collection_item c LEFT JOIN card_print_ref r ON r.printing_uuid=c.printing_uuid
 WHERE c.normal_quantity>0 OR c.foil_quantity>0 ORDER BY c.printing_uuid`); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	if len(owned) == 0 {
		return []record{}, nil
	}
	cardTx, err := s.card.BeginTxx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer cardTx.Rollback()
	metadata := make(map[string]metadataRow, len(owned))
	for start := 0; start < len(owned); start += 500 {
		end := min(start+500, len(owned))
		ids := make([]string, 0, end-start)
		for _, row := range owned[start:end] {
			ids = append(ids, row.UUID)
		}
		query, args, err := sqlx.In(`SELECT p.uuid AS printing_uuid,NULLIF(c.oracle_id,'') AS oracle_id,c.name,
 c.collector_number,c.released_at,c.cmc,c.colors_mask,c.rarity,
 s.code AS set_code,s.name AS set_name,s.released_at AS set_released_at,
 COALESCE((SELECT MIN(NULLIF(TRIM(st.name),'')) FROM set_translation st WHERE st.code=s.code),s.name) AS set_display_name,
 l.code AS language_code,l.display_name AS language_name,l.sort_order AS language_order
 FROM mtgnissa_card_printing p JOIN mtgnissa_card c ON c.uuid=p.card_uuid
 JOIN mtgnissa_set s ON s.code=c.set_code JOIN scryfall_language l ON l.code=p.language_code
 WHERE p.uuid IN (?) AND c.face_index IN (-1,0)`, ids)
		if err != nil {
			return nil, err
		}
		var rows []metadataRow
		if err := cardTx.SelectContext(ctx, &rows, query, args...); err != nil {
			return nil, err
		}
		for _, row := range rows {
			metadata[row.UUID] = row
		}
	}
	result := make([]record, 0, len(owned))
	for _, row := range owned {
		meta, ok := metadata[row.UUID]
		if !ok {
			return nil, fmt.Errorf("owned printing %s is unavailable in card data", row.UUID)
		}
		if !sameOracle(row.Oracle, meta.Oracle) || row.Set != meta.SetCode || row.Number != meta.Number || row.Lang != meta.LanguageCode {
			return nil, fmt.Errorf("owned printing %s identity does not match card data", row.UUID)
		}
		colors := make([]string, 0, 5)
		for i, color := range []string{"W", "U", "B", "R", "G"} {
			if meta.Colors&(1<<i) != 0 {
				colors = append(colors, color)
			}
		}
		result = append(result, record{Quantity: row.Quantity, Card: cards.Summary{
			PrintingUUID: meta.UUID, OracleID: meta.Oracle, Name: meta.Name, CollectorNumber: meta.Number,
			ReleasedAt: meta.Released, CMC: meta.CMC, Colors: colors, Rarity: meta.Rarity,
			Set:      cards.Set{Code: meta.SetCode, Name: meta.SetName, DisplayName: meta.SetDisplay, ReleasedAt: meta.SetReleased},
			Language: cards.Language{Code: meta.LanguageCode, DisplayName: meta.LanguageName, SortOrder: meta.LanguageSort},
		}})
	}
	if err := cardTx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}

// Summaries resolves complete card summaries only for the current page.
func (s *Store) Summaries(ctx context.Context, ids []string) (map[string]cards.Summary, error) {
	return s.faces.Summaries(ctx, ids)
}

func sameOracle(a, b *string) bool {
	if a == nil || *a == "" {
		return b == nil || *b == ""
	}
	return b != nil && strings.EqualFold(*a, *b)
}
