package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/doug-martin/goqu/v9"
	"github.com/jmoiron/sqlx"

	"mtgnissa/internal/appdata"
	"mtgnissa/internal/apperror"
	"mtgnissa/internal/model"
)

// DeckTx queries saved decks within the caller's application snapshot.
type DeckTx struct {
	tx *sqlx.Tx
}

// DeckPosition identifies the compound boundary of a saved deck page.
type DeckPosition struct {
	ID    uint64
	Name  string
	Time  time.Time
	Count uint64
}

type deckUsageRow struct {
	ID       uint64 `db:"id"`
	Name     string `db:"name"`
	Quantity uint64 `db:"quantity"`
	Section  string `db:"section_type"`
}

type deckPersonalRow struct {
	OracleID string `db:"oracle_id"`
	Quantity uint64 `db:"quantity"`
	DeckID   uint64 `db:"deck_id"`
}

type deckPreferenceRow struct {
	UUID     string `db:"printing_uuid"`
	Priority int    `db:"priority"`
}

// Get reads a saved definition and optionally locks its parent row for a mutation.
func (s *DeckTx) Get(ctx context.Context, id uint64, lock bool) (model.SavedDeck, error) {
	query := "SELECT id,name,source_text,cover_oracle_id,created_at,updated_at FROM deck WHERE id=?"
	if lock {
		query += " FOR UPDATE"
	}
	var row appdata.Deck
	if err := s.tx.GetContext(ctx, &row, query, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.SavedDeck{}, apperror.New(apperror.CodeDeckNotFound, "deck not found")
		}
		return model.SavedDeck{}, err
	}
	var cards []appdata.DeckCard
	if err := s.tx.SelectContext(ctx, &cards, "SELECT deck_id,section_type,oracle_id,quantity,name_snapshot FROM deck_card WHERE deck_id=? ORDER BY section_type,oracle_id", id); err != nil {
		return model.SavedDeck{}, err
	}
	return savedDeck(row, cards), nil
}

// List filters and pages saved definitions before reading their section quantities.
func (s *DeckTx) List(ctx context.Context, params model.DeckListParams, position *DeckPosition, pageSize int) ([]model.SavedDeck, int64, bool, error) {
	base := deckListBase(params.Query)
	statement, args, err := base.Select(goqu.COUNT("*")).ToSQL()
	if err != nil {
		return nil, 0, false, err
	}
	var total int64
	if err := s.tx.GetContext(ctx, &total, statement, args...); err != nil {
		return nil, 0, false, err
	}
	statement, args, err = deckListPage(base, params.Sort, position, pageSize).ToSQL()
	if err != nil {
		return nil, 0, false, err
	}
	var rows []appdata.Deck
	if err := s.tx.SelectContext(ctx, &rows, statement, args...); err != nil {
		return nil, 0, false, err
	}
	more := len(rows) > pageSize
	if more {
		rows = rows[:pageSize]
	}
	if len(rows) == 0 {
		return []model.SavedDeck{}, total, false, nil
	}
	ids := make([]uint64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	statement, args, err = sqlx.In("SELECT deck_id,section_type,oracle_id,quantity,name_snapshot FROM deck_card WHERE deck_id IN (?) ORDER BY deck_id,section_type,oracle_id", ids)
	if err != nil {
		return nil, 0, false, err
	}
	var cards []appdata.DeckCard
	if err := s.tx.SelectContext(ctx, &cards, statement, args...); err != nil {
		return nil, 0, false, err
	}
	groups := make(map[uint64][]appdata.DeckCard)
	for _, card := range cards {
		groups[card.DeckID] = append(groups[card.DeckID], card)
	}
	result := make([]model.SavedDeck, 0, len(rows))
	for _, row := range rows {
		result = append(result, savedDeck(row, groups[row.ID]))
	}
	return result, total, more, nil
}

func deckListBase(query string) *goqu.SelectDataset {
	dataset := mysqlDialect.From(goqu.T("deck").As("d")).Prepared(true)
	if query != "" {
		dataset = dataset.Where(goqu.L("LOWER(d.name) COLLATE utf8mb4_bin LIKE ? ESCAPE '='", "%"+escapeLike(query)+"%"))
	}
	return dataset
}

func deckListPage(base *goqu.SelectDataset, order string, position *DeckPosition, pageSize int) *goqu.SelectDataset {
	dataset := base.Select(goqu.I("d.id"), goqu.I("d.name"), goqu.L("'' AS source_text"), goqu.I("d.cover_oracle_id"), goqu.I("d.created_at"), goqu.I("d.updated_at"))
	key := goqu.L("d.updated_at")
	var boundary any
	if position != nil {
		boundary = position.Time
	}
	switch order {
	case "created_desc":
		key = goqu.L("d.created_at")
	case "name_asc":
		key = goqu.L("LOWER(d.name) COLLATE utf8mb4_bin")
		if position != nil {
			boundary = position.Name
		}
	case "card_count_desc":
		counts := mysqlDialect.From("deck_card").Select("deck_id", goqu.SUM("quantity").As("quantity")).
			Where(goqu.C("section_type").In("commander", "mainboard", "sideboard")).GroupBy("deck_id")
		dataset = dataset.LeftJoin(counts.As("counts"), goqu.On(goqu.I("counts.deck_id").Eq(goqu.I("d.id"))))
		key = goqu.L("COALESCE(counts.quantity,0)")
		if position != nil {
			boundary = position.Count
		}
	}
	if position != nil {
		next := key.Lt(boundary)
		if order == "name_asc" {
			next = key.Gt(boundary)
		}
		dataset = dataset.Where(goqu.Or(next, goqu.And(key.Eq(boundary), goqu.I("d.id").Gt(position.ID))))
	}
	if order == "name_asc" {
		return dataset.Order(key.Asc(), goqu.I("d.id").Asc()).Limit(uint(pageSize + 1))
	}
	return dataset.Order(key.Desc(), goqu.I("d.id").Asc()).Limit(uint(pageSize + 1))
}

func savedDeck(row appdata.Deck, cards []appdata.DeckCard) model.SavedDeck {
	result := model.SavedDeck{ID: row.ID, Name: row.Name, SourceText: row.SourceText, CoverOracleID: row.CoverOracleID,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, Cards: []model.SavedDeckCard{}}
	for _, card := range cards {
		result.Cards = append(result.Cards, model.SavedDeckCard{Section: string(card.SectionType), OracleID: card.OracleID, Quantity: uint64(card.Quantity), Name: card.NameSnapshot})
	}
	return result
}

// Create writes a definition and its entries without committing the transaction.
func (s *DeckTx) Create(ctx context.Context, deck model.SavedDeck) (uint64, error) {
	now := time.Now().UTC()
	result, err := s.tx.ExecContext(ctx, "INSERT INTO deck (name,source_text,cover_oracle_id,created_at,updated_at) VALUES (?,?,?,?,?)", deck.Name, deck.SourceText, deck.CoverOracleID, now, now)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	if err := s.insertCards(ctx, uint64(id), deck.Cards); err != nil {
		return 0, err
	}
	return uint64(id), nil
}

// Replace writes a complete definition after the service has locked and compared it.
func (s *DeckTx) Replace(ctx context.Context, deck model.SavedDeck) error {
	if _, err := s.tx.ExecContext(ctx, "DELETE FROM deck_card WHERE deck_id=?", deck.ID); err != nil {
		return err
	}
	if err := s.insertCards(ctx, deck.ID, deck.Cards); err != nil {
		return err
	}
	_, err := s.tx.ExecContext(ctx, "UPDATE deck SET name=?,source_text=?,cover_oracle_id=?,updated_at=? WHERE id=?", deck.Name, deck.SourceText, deck.CoverOracleID, time.Now().UTC(), deck.ID)
	return err
}

func (s *DeckTx) insertCards(ctx context.Context, id uint64, cards []model.SavedDeckCard) error {
	for _, card := range cards {
		if _, err := s.tx.ExecContext(ctx, "INSERT INTO deck_card (deck_id,section_type,oracle_id,quantity,name_snapshot) VALUES (?,?,?,?,?)", id, card.Section, card.OracleID, card.Quantity, card.Name); err != nil {
			return err
		}
	}
	return nil
}

// Delete removes a locked definition and its foreign-key cascaded entries.
func (s *DeckTx) Delete(ctx context.Context, id uint64) error {
	_, err := s.tx.ExecContext(ctx, "DELETE FROM deck WHERE id=?", id)
	return err
}

// Personal reads logical ownership, total counted usage, and printing selection priorities.
func (s *DeckTx) Personal(ctx context.Context, oracles []string) (model.DeckPersonal, error) {
	result := model.DeckPersonal{Owned: map[string]uint64{}, AllQuantity: map[string]uint64{}, UsingDecks: map[string]uint64{}, Preferred: map[string]int{}}
	for start := 0; start < len(oracles); start += 500 {
		ids := oracles[start:min(start+500, len(oracles))]
		query, args, err := sqlx.In(`SELECT r.oracle_id,SUM(c.normal_quantity+c.foil_quantity) AS quantity
 FROM card_print_ref r JOIN collection_item c ON c.printing_uuid=r.printing_uuid WHERE r.oracle_id IN (?) GROUP BY r.oracle_id`, ids)
		if err != nil {
			return result, err
		}
		var owned []deckPersonalRow
		if err := s.tx.SelectContext(ctx, &owned, query, args...); err != nil {
			return result, err
		}
		for _, row := range owned {
			result.Owned[row.OracleID] = row.Quantity
		}
		query, args, err = sqlx.In(`SELECT oracle_id,deck_id,SUM(quantity) AS quantity FROM deck_card
 WHERE oracle_id IN (?) AND section_type IN ('commander','mainboard','sideboard') GROUP BY oracle_id,deck_id`, ids)
		if err != nil {
			return result, err
		}
		var usage []deckPersonalRow
		if err := s.tx.SelectContext(ctx, &usage, query, args...); err != nil {
			return result, err
		}
		for _, row := range usage {
			result.AllQuantity[row.OracleID] += row.Quantity
			result.UsingDecks[row.OracleID]++
		}
		query, args, err = sqlx.In(`SELECT r.printing_uuid,1 AS priority FROM card_print_ref r
 JOIN collection_item c ON c.printing_uuid=r.printing_uuid WHERE r.oracle_id IN (?) AND (c.normal_quantity>0 OR c.foil_quantity>0)
 UNION ALL SELECT r.printing_uuid,2 AS priority FROM card_print_ref r JOIN wishlist_item w ON w.printing_uuid=r.printing_uuid WHERE r.oracle_id IN (?)`, ids, ids)
		if err != nil {
			return result, err
		}
		var preferences []deckPreferenceRow
		if err := s.tx.SelectContext(ctx, &preferences, query, args...); err != nil {
			return result, err
		}
		for _, row := range preferences {
			if old := result.Preferred[row.UUID]; old == 0 || row.Priority < old {
				result.Preferred[row.UUID] = row.Priority
			}
		}
	}
	return result, nil
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
