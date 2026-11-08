package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"

	"mtgnissa/internal/apperror"
	"mtgnissa/internal/model"
)

// CollectionStore reads personal ownership and batches matching metadata from the Card DB.
type CollectionStore struct {
	app   *sqlx.DB
	card  *sqlx.DB
	faces *CardStore
}

type ownedRow struct {
	UUID   string  `db:"printing_uuid"`
	Oracle *string `db:"oracle_id"`
	Set    string  `db:"set_code"`
	Number string  `db:"collector_number"`
	Lang   string  `db:"lang"`
	model.Quantity
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

// CollectionTx reads and changes collection quantities in the caller's transaction.
type CollectionTx struct {
	tx *sqlx.Tx
}

type versionQuantityRow struct {
	UUID string `db:"printing_uuid"`
	model.Quantity
}

// NewCollectionStore creates a collection reader using separate application and card databases.
func NewCollectionStore(app, card *sqlx.DB) *CollectionStore {
	return &CollectionStore{app: app, card: card, faces: NewCardStore(card)}
}

// Load reads only owned printings, without scanning the entire card catalogue.
func (s *CollectionStore) Load(ctx context.Context) ([]model.OwnedPrinting, error) {
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
		return []model.OwnedPrinting{}, nil
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
	result := make([]model.OwnedPrinting, 0, len(owned))
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
		result = append(result, model.OwnedPrinting{Quantity: row.Quantity, Card: model.CardSummary{
			PrintingUUID: meta.UUID, OracleID: meta.Oracle, Name: meta.Name, CollectorNumber: meta.Number,
			ReleasedAt: meta.Released, CMC: meta.CMC, Colors: colors, Rarity: meta.Rarity,
			Set:      model.SetRef{Code: meta.SetCode, Name: meta.SetName, DisplayName: meta.SetDisplay, ReleasedAt: meta.SetReleased},
			Language: model.Language{Code: meta.LanguageCode, DisplayName: meta.LanguageName, SortOrder: meta.LanguageSort},
		}})
	}
	if err := cardTx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}

// Summaries resolves complete card summaries only for the current page.
func (s *CollectionStore) Summaries(ctx context.Context, ids []string) (map[string]model.CardSummary, error) {
	return s.faces.Summaries(ctx, ids)
}

func sameOracle(a, b *string) bool {
	if a == nil || *a == "" {
		return b == nil || *b == ""
	}
	return b != nil && strings.EqualFold(*a, *b)
}

// Adjust changes one precise printing and returns the event for transactional logging.
func (s *CollectionTx) Adjust(ctx context.Context, card model.CardSummary, finish string, delta int) (model.CardChange, error) {
	now := time.Now().UTC()
	if err := lockPrintingRef(ctx, s.tx, card, now); err != nil {
		return model.CardChange{}, err
	}
	var quantity model.Quantity
	err := s.tx.GetContext(ctx, &quantity, "SELECT normal_quantity,foil_quantity FROM collection_item WHERE printing_uuid=? FOR UPDATE", card.PrintingUUID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return model.CardChange{}, err
	}
	quantity, err = adjustQuantity(quantity, finish, delta)
	if err != nil {
		return model.CardChange{}, err
	}
	if quantity.Normal+quantity.Foil == 0 {
		_, err = s.tx.ExecContext(ctx, "DELETE FROM collection_item WHERE printing_uuid=?", card.PrintingUUID)
	} else {
		_, err = s.tx.ExecContext(ctx, `INSERT INTO collection_item (printing_uuid,normal_quantity,foil_quantity,created_at,updated_at)
 VALUES (?,?,?,?,?) ON DUPLICATE KEY UPDATE normal_quantity=VALUES(normal_quantity),foil_quantity=VALUES(foil_quantity),updated_at=VALUES(updated_at)`,
			card.PrintingUUID, quantity.Normal, quantity.Foil, now, now)
	}
	if err != nil {
		return model.CardChange{}, err
	}
	return model.CardChange{Card: card, Finish: finish, Delta: delta, OccurredAt: now}, nil
}

// State reads precise quantities and unfiltered collection and series totals.
func (s *CollectionTx) State(ctx context.Context, card model.CardSummary) (model.CollectionState, error) {
	state := model.CollectionState{Card: card}
	err := s.tx.GetContext(ctx, &state.Group, "SELECT normal_quantity,foil_quantity FROM collection_item WHERE printing_uuid=?", card.PrintingUUID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return model.CollectionState{}, err
	}
	const totalsQuery = `SELECT COALESCE(SUM(c.normal_quantity+c.foil_quantity),0) AS total_quantity,
 COUNT(DISTINCT CASE WHEN r.oracle_id IS NULL OR r.oracle_id='' THEN CONCAT('print:',r.printing_uuid) ELSE CONCAT('oracle:',r.oracle_id) END) AS distinct_cards
 FROM collection_item c JOIN card_print_ref r ON r.printing_uuid=c.printing_uuid`
	if err := s.tx.GetContext(ctx, &state.Collection, totalsQuery); err != nil {
		return model.CollectionState{}, err
	}
	if err := s.tx.GetContext(ctx, &state.SetSummary, totalsQuery+" WHERE r.set_code=?", card.Set.Code); err != nil {
		return model.CollectionState{}, err
	}
	return state, nil
}

func (s *CollectionTx) quantities(ctx context.Context, ids []string) (map[string]model.Quantity, error) {
	owned := make(map[string]model.Quantity)
	for start := 0; start < len(ids); start += 500 {
		query, args, err := sqlx.In("SELECT printing_uuid,normal_quantity,foil_quantity FROM collection_item WHERE printing_uuid IN (?)", ids[start:min(start+500, len(ids))])
		if err != nil {
			return nil, err
		}
		var rows []versionQuantityRow
		if err := s.tx.SelectContext(ctx, &rows, query, args...); err != nil {
			return nil, err
		}
		for _, row := range rows {
			owned[row.UUID] = row.Quantity
		}
	}
	return owned, nil
}

func adjustQuantity(quantity model.Quantity, finish string, delta int) (model.Quantity, error) {
	if (finish != "normal" && finish != "foil") || (delta != 1 && delta != -1) {
		return model.Quantity{}, apperror.New(apperror.CodeValidationFailed, "invalid collection adjustment")
	}
	value := &quantity.Normal
	if finish == "foil" {
		value = &quantity.Foil
	}
	if delta < 0 {
		if *value == 0 {
			return model.Quantity{}, apperror.New(apperror.CodeCollectionUnderflow, "collection quantity cannot be negative")
		}
		*value--
	} else {
		if *value >= math.MaxUint32 {
			return model.Quantity{}, apperror.New(apperror.CodeValidationFailed, "collection quantity is at its maximum")
		}
		*value++
	}
	return quantity, nil
}
