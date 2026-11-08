package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"

	"mtgnissa/internal/apperror"
	"mtgnissa/internal/model"
)

// CardStore reads printing identities, card text, and Oracle metadata from Card DB.
type CardStore struct {
	db *sqlx.DB
}

type faceRow struct {
	UUID            string     `db:"printing_uuid"`
	SourceID        string     `db:"source_id"`
	ImageID         string     `db:"image_id"`
	Index           int        `db:"face_index"`
	OracleID        *string    `db:"oracle_id"`
	Name            string     `db:"name"`
	FaceName        string     `db:"face_name"`
	DisplayName     string     `db:"display_name"`
	CMC             float64    `db:"cmc"`
	ManaCost        *string    `db:"mana_cost"`
	Colors          uint64     `db:"colors_mask"`
	Layout          string     `db:"layout"`
	FaceCategory    string     `db:"face_category"`
	Rarity          string     `db:"rarity"`
	Number          string     `db:"collector_number"`
	ReleasedAt      *time.Time `db:"released_at"`
	SetCode         string     `db:"set_code"`
	SetName         string     `db:"set_name"`
	SetDisplayName  string     `db:"set_display_name"`
	SetReleasedAt   *time.Time `db:"set_released_at"`
	LanguageCode    string     `db:"language_code"`
	LanguageName    string     `db:"language_name"`
	LanguageOrder   int        `db:"language_order"`
	TypeLine        *string    `db:"type_line"`
	DisplayTypeLine *string    `db:"display_type_line"`
	Text            *string    `db:"oracle_text"`
	DisplayText     *string    `db:"display_oracle_text"`
	Flavor          *string    `db:"flavor_text"`
	DisplayFlavor   *string    `db:"display_flavor_text"`
	Power           *string    `db:"power"`
	Toughness       *string    `db:"toughness"`
	Loyalty         *string    `db:"loyalty"`
}

type setRow struct {
	Code        string     `db:"code"`
	Name        string     `db:"name"`
	DisplayName string     `db:"display_name"`
	ReleasedAt  *time.Time `db:"released_at"`
	Kind        string     `db:"set_type"`
}

type languageRow struct {
	Code        string `db:"code"`
	DisplayName string `db:"display_name"`
	SortOrder   int    `db:"sort_order"`
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

// Translation lookups are correlated by indexed card UUID. Do not join global
// GROUP BY derived tables here: older MySQL versions materialize the full mapping.
const cardSummaryColumns = `
 c.scryfall_id AS source_id,
 c.face_index, NULLIF(c.oracle_id, '') AS oracle_id, c.name,
 COALESCE(NULLIF(c.face_name, ''), c.name) AS face_name,
 COALESCE(NULLIF(ct.face_name, ''), NULLIF(ct.name, ''),
  (SELECT MIN(NULLIF(t.translated_name, '')) FROM card_oracle_translation_mapping m
   JOIN oracle_translation t ON t.face_oracle_id=m.face_oracle_id WHERE m.card_uuid=c.uuid),
  NULLIF(c.face_name, ''), c.name) AS display_name,
 c.cmc, c.mana_cost, c.colors_mask, c.layout, ly.face_category, c.rarity, c.collector_number, c.released_at,
 s.code AS set_code, s.name AS set_name,
 COALESCE((SELECT MIN(NULLIF(TRIM(st.name), '')) FROM set_translation st WHERE st.code=s.code), s.name) AS set_display_name,
 s.released_at AS set_released_at
`

const printingSummaryColumns = `
 p.uuid AS printing_uuid,
 COALESCE(NULLIF(p.scryfall_id, ''), c.scryfall_id) AS image_id,
` + cardSummaryColumns + `,
 l.code AS language_code, l.display_name AS language_name, l.sort_order AS language_order
`

const printingFrom = `
 FROM mtgnissa_card_printing p
 JOIN mtgnissa_card c ON c.uuid = p.card_uuid
 JOIN mtgnissa_set s ON s.code = c.set_code
 JOIN scryfall_language l ON l.code = p.language_code
 JOIN scryfall_layout ly ON ly.code = c.layout
 LEFT JOIN card_translation ct ON ct.card_id = c.uuid
`

const summaryQuery = `SELECT ` + printingSummaryColumns + printingFrom

const printingQuery = `SELECT ` + printingSummaryColumns + `,
 c.type_line, COALESCE(NULLIF(ct.type_line, ''),
  (SELECT MIN(NULLIF(t.translated_type, '')) FROM card_oracle_translation_mapping m
   JOIN oracle_translation t ON t.face_oracle_id=m.face_oracle_id WHERE m.card_uuid=c.uuid), c.type_line) AS display_type_line,
 c.oracle_text, COALESCE(NULLIF(ct.text, ''),
  (SELECT MIN(NULLIF(t.translated_text, '')) FROM card_oracle_translation_mapping m
   JOIN oracle_translation t ON t.face_oracle_id=m.face_oracle_id WHERE m.card_uuid=c.uuid), c.oracle_text) AS display_oracle_text,
 c.flavor_text, COALESCE(NULLIF(ct.flavor_text, ''),
  (SELECT MIN(NULLIF(t.translated_flavor_text, '')) FROM card_flavor_translation_mapping m
   JOIN flavor_translation t ON t.flavor_id=m.flavor_id WHERE m.card_uuid=c.uuid), c.flavor_text) AS display_flavor_text,
 c.power, c.toughness, c.loyalty
` + printingFrom

const versionQuery = `SELECT p.uuid AS printing_uuid, NULLIF(c.oracle_id,'') AS oracle_id,
 s.code AS set_code, s.name AS set_name,
 COALESCE((SELECT MIN(NULLIF(TRIM(st.name),'')) FROM set_translation st WHERE st.code=s.code),s.name) AS set_display_name,
 s.released_at AS set_released_at, c.collector_number, c.released_at,
 l.code AS language_code, l.display_name AS language_name, l.sort_order AS language_order
 FROM mtgnissa_card_printing p JOIN mtgnissa_card c ON c.uuid=p.card_uuid
 JOIN mtgnissa_set s ON s.code=c.set_code JOIN scryfall_language l ON l.code=p.language_code
`

// NewCardStore creates a printing store backed by Card DB.
func NewCardStore(db *sqlx.DB) *CardStore {
	return &CardStore{db: db}
}

// List reads the complete lightweight list for one ordinary series and language.
func (s *CardStore) List(ctx context.Context, code, language, query string) (model.CardList, error) {
	tx, err := s.db.BeginTxx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return model.CardList{}, err
	}
	defer tx.Rollback()
	var set setRow
	err = tx.GetContext(ctx, &set, `SELECT s.code, s.name, s.set_type, s.released_at,
 COALESCE((SELECT MIN(NULLIF(TRIM(st.name),'')) FROM set_translation st WHERE st.code=s.code),s.name) AS display_name
 FROM mtgnissa_set s WHERE s.code=?`, code)
	if errors.Is(err, sql.ErrNoRows) {
		return model.CardList{}, apperror.New(apperror.CodeSetNotFound, "set not found")
	}
	if err != nil {
		return model.CardList{}, err
	}
	if set.Kind == "token" {
		return model.CardList{}, apperror.New(apperror.CodeInvalidRequest, "use the tokens endpoint for token sets")
	}
	var lang languageRow
	if err := tx.GetContext(ctx, &lang, "SELECT code, display_name, sort_order FROM scryfall_language WHERE code=?", language); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.CardList{}, apperror.WithDetails(apperror.CodeValidationFailed, "unknown language", []apperror.Detail{{Field: "language", Reason: "unknown_language"}})
		}
		return model.CardList{}, err
	}
	var rows []faceRow
	if err := tx.SelectContext(ctx, &rows, summaryQuery+` WHERE s.code=? AND l.code=?`, set.Code, lang.Code); err != nil {
		return model.CardList{}, err
	}
	// Filter the complete printing group so a matching secondary face retains its main-face identity.
	matched := make(map[string]bool)
	if query != "" {
		var ids []string
		pattern := strings.NewReplacer("=", "==", "%", "=%", "_", "=_").Replace(query)
		if err := tx.SelectContext(ctx, &ids, `SELECT p.uuid FROM mtgnissa_card_printing p JOIN mtgnissa_card c ON c.uuid=p.card_uuid
 WHERE c.set_code=? AND p.language_code=? AND p.printed_name LIKE ? ESCAPE '='`, set.Code, lang.Code, "%"+pattern+"%"); err != nil {
			return model.CardList{}, err
		}
		for _, id := range ids {
			matched[id] = true
		}
	}
	items := make([]model.CardSummary, 0)
	for _, group := range groupFaces(rows) {
		include := query == ""
		for _, row := range group {
			include = include || matched[row.UUID]
		}
		if include {
			card, _ := summarize(group)
			items = append(items, card)
		}
	}
	sortSummaries(items)
	if err := tx.Commit(); err != nil {
		return model.CardList{}, err
	}
	return model.CardList{Set: model.SetRef{Code: set.Code, Name: set.Name, DisplayName: set.DisplayName, ReleasedAt: set.ReleasedAt}, Language: model.Language{Code: lang.Code, DisplayName: lang.DisplayName, SortOrder: lang.SortOrder}, Items: items}, nil
}

func readPrinting(ctx context.Context, tx *sqlx.Tx, id string) ([]faceRow, error) {
	var rows []faceRow
	const requestedPrinting = `
 JOIN mtgnissa_card_printing requested ON requested.uuid=?
 JOIN mtgnissa_card seed ON seed.uuid=requested.card_uuid AND seed.face_index IN (-1,0)
 WHERE c.scryfall_id=seed.scryfall_id AND p.language_code=requested.language_code
`
	if err := tx.SelectContext(ctx, &rows, printingQuery+requestedPrinting, id); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, apperror.New(apperror.CodeCardNotFound, "card printing not found or not a main-face entry")
	}
	return rows, nil
}

// Printing resolves a canonical language printing and its complete face data.
func (s *CardStore) Printing(ctx context.Context, id string) (model.CardSummary, []model.CardFace, error) {
	tx, err := s.db.BeginTxx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return model.CardSummary{}, nil, err
	}
	defer tx.Rollback()
	rows, err := readPrinting(ctx, tx, id)
	if err != nil {
		return model.CardSummary{}, nil, err
	}
	card, faces := summarize(rows)
	if err := tx.Commit(); err != nil {
		return model.CardSummary{}, nil, err
	}
	return card, faces, nil
}

// Summaries resolves canonical printing summaries and all their faces in batches.
func (s *CardStore) Summaries(ctx context.Context, ids []string) (map[string]model.CardSummary, error) {
	result := make(map[string]model.CardSummary, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	tx, err := s.db.BeginTxx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	for start := 0; start < len(ids); start += 500 {
		query, args, err := sqlx.In(summaryQuery+`
 JOIN mtgnissa_card_printing requested ON requested.uuid IN (?)
 JOIN mtgnissa_card seed ON seed.uuid=requested.card_uuid AND seed.face_index IN (-1,0)
 WHERE c.scryfall_id=seed.scryfall_id AND p.language_code=requested.language_code`, ids[start:min(start+500, len(ids))])
		if err != nil {
			return nil, err
		}
		var rows []faceRow
		if err := tx.SelectContext(ctx, &rows, query, args...); err != nil {
			return nil, err
		}
		for _, group := range groupFaces(rows) {
			card, _ := summarize(group)
			result[card.PrintingUUID] = card
		}
	}
	for _, id := range ids {
		if _, ok := result[id]; !ok {
			return nil, fmt.Errorf("printing %s is unavailable or not a main-face entry", id)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}

func groupFaces(rows []faceRow) [][]faceRow {
	groups := make(map[[2]string][]faceRow)
	for _, row := range rows {
		key := [2]string{row.SourceID, row.LanguageCode}
		groups[key] = append(groups[key], row)
	}
	result := make([][]faceRow, 0, len(groups))
	for _, group := range groups {
		result = append(result, group)
	}
	return result
}

func summarize(rows []faceRow) (model.CardSummary, []model.CardFace) {
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Index != rows[j].Index {
			return rows[i].Index < rows[j].Index
		}
		return rows[i].UUID < rows[j].UUID
	})
	r := rows[0]
	displayName := r.DisplayName
	if len(rows) > 1 {
		names := make([]string, 0, len(rows))
		for _, row := range rows {
			names = append(names, row.DisplayName)
		}
		displayName = strings.Join(names, " // ")
	}
	faces := make([]model.CardFace, 0, len(rows))
	images := model.CardImages{FrontURL: picture(r.ImageID, "front")}
	if distinctBack(r) {
		images.BackURL = picture(r.ImageID, "back")
	}
	for _, row := range rows {
		image := picture(row.ImageID, "front")
		if row.Index > 0 {
			image = nil
			if distinctBack(row) {
				image = picture(row.ImageID, "back")
			}
		}
		faces = append(faces, model.CardFace{FaceIndex: row.Index, Name: row.FaceName, DisplayName: row.DisplayName,
			ManaCost: row.ManaCost, TypeLine: row.TypeLine, DisplayTypeLine: row.DisplayTypeLine,
			OracleText: row.Text, DisplayOracleText: row.DisplayText, FlavorText: row.Flavor,
			DisplayFlavorText: row.DisplayFlavor, Power: row.Power, Toughness: row.Toughness, Loyalty: row.Loyalty, ImageURL: image})
	}
	colors := make([]string, 0)
	for i, code := range []string{"W", "U", "B", "R", "G"} {
		if r.Colors&(1<<i) != 0 {
			colors = append(colors, code)
		}
	}
	return model.CardSummary{PrintingUUID: r.UUID, OracleID: r.OracleID, Name: r.Name, DisplayName: displayName,
		Set:             model.SetRef{Code: r.SetCode, Name: r.SetName, DisplayName: r.SetDisplayName, ReleasedAt: r.SetReleasedAt},
		CollectorNumber: r.Number, Language: model.Language{Code: r.LanguageCode, DisplayName: r.LanguageName, SortOrder: r.LanguageOrder},
		ReleasedAt: r.ReleasedAt, CMC: r.CMC, ManaCost: r.ManaCost, Colors: colors, Rarity: r.Rarity, Layout: r.Layout, Images: images}, faces
}

func distinctBack(row faceRow) bool {
	return row.FaceCategory == "double_sided" || row.FaceCategory == "reversible" || row.Layout == "battle"
}

func picture(id, side string) *string {
	id = strings.ToLower(id)
	if !model.ValidUUID(id) {
		return nil
	}
	url := "https://cards.scryfall.io/normal/" + side + "/" + id[:1] + "/" + id[1:2] + "/" + id + ".jpg"
	return &url
}

func sortSummaries(items []model.CardSummary) {
	sort.Slice(items, func(i, j int) bool {
		if order := model.CompareCollectorNumbers(items[i].CollectorNumber, items[j].CollectorNumber); order != 0 {
			return order < 0
		}
		return items[i].PrintingUUID < items[j].PrintingUUID
	})
}

// metadata reads only fields needed to filter and order personal card lists.
func (s *CardStore) metadata(ctx context.Context, ids []string) (map[string]metadataRow, error) {
	if len(ids) == 0 {
		return map[string]metadataRow{}, nil
	}
	cardTx, err := s.db.BeginTxx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer cardTx.Rollback()
	metadata := make(map[string]metadataRow, len(ids))
	for start := 0; start < len(ids); start += 500 {
		end := min(start+500, len(ids))
		query, args, err := sqlx.In(`SELECT p.uuid AS printing_uuid,NULLIF(c.oracle_id,'') AS oracle_id,c.name,
 c.collector_number,c.released_at,c.cmc,c.colors_mask,c.rarity,
 s.code AS set_code,s.name AS set_name,s.released_at AS set_released_at,
 COALESCE((SELECT MIN(NULLIF(TRIM(st.name),'')) FROM set_translation st WHERE st.code=s.code),s.name) AS set_display_name,
 l.code AS language_code,l.display_name AS language_name,l.sort_order AS language_order
 FROM mtgnissa_card_printing p JOIN mtgnissa_card c ON c.uuid=p.card_uuid
 JOIN mtgnissa_set s ON s.code=c.set_code JOIN scryfall_language l ON l.code=p.language_code
 WHERE p.uuid IN (?) AND c.face_index IN (-1,0)`, ids[start:end])
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
	if err := cardTx.Commit(); err != nil {
		return nil, err
	}
	return metadata, nil
}

func (meta metadataRow) summary() model.CardSummary {
	colors := make([]string, 0, 5)
	for i, color := range []string{"W", "U", "B", "R", "G"} {
		if meta.Colors&(1<<i) != 0 {
			colors = append(colors, color)
		}
	}
	return model.CardSummary{
		PrintingUUID: meta.UUID, OracleID: meta.Oracle, Name: meta.Name, CollectorNumber: meta.Number,
		ReleasedAt: meta.Released, CMC: meta.CMC, Colors: colors, Rarity: meta.Rarity,
		Set:      model.SetRef{Code: meta.SetCode, Name: meta.SetName, DisplayName: meta.SetDisplay, ReleasedAt: meta.SetReleased},
		Language: model.Language{Code: meta.LanguageCode, DisplayName: meta.LanguageName, SortOrder: meta.LanguageSort},
	}
}

func sameOracle(a, b *string) bool {
	if a == nil || *a == "" {
		return b == nil || *b == ""
	}
	return b != nil && strings.EqualFold(*a, *b)
}
