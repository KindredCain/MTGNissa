package cards

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"

	"mtgnissa/internal/apperror"
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

// Translation lookups are correlated by indexed card UUID. Do not join global
// GROUP BY derived tables here: older MySQL versions materialize the full mapping.
const printingSummaryColumns = `
 p.uuid AS printing_uuid, c.scryfall_id AS source_id,
 COALESCE(NULLIF(p.scryfall_id, ''), c.scryfall_id) AS image_id,
 c.face_index, NULLIF(c.oracle_id, '') AS oracle_id, c.name,
 COALESCE(NULLIF(c.face_name, ''), c.name) AS face_name,
 COALESCE(NULLIF(ct.face_name, ''), NULLIF(ct.name, ''),
  (SELECT MIN(NULLIF(t.translated_name, '')) FROM card_oracle_translation_mapping m
   JOIN oracle_translation t ON t.face_oracle_id=m.face_oracle_id WHERE m.card_uuid=c.uuid),
  NULLIF(c.face_name, ''), c.name) AS display_name,
 c.cmc, c.mana_cost, c.colors_mask, c.layout, ly.face_category, c.rarity, c.collector_number, c.released_at,
 s.code AS set_code, s.name AS set_name,
 COALESCE((SELECT MIN(NULLIF(TRIM(st.name), '')) FROM set_translation st WHERE st.code=s.code), s.name) AS set_display_name,
 s.released_at AS set_released_at,
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

func (s *CardStore) list(ctx context.Context, code, language, query string) (List, error) {
	tx, err := s.db.BeginTxx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return List{}, err
	}
	defer tx.Rollback()
	var set setRow
	err = tx.GetContext(ctx, &set, `SELECT s.code, s.name, s.set_type, s.released_at,
 COALESCE((SELECT MIN(NULLIF(TRIM(st.name),'')) FROM set_translation st WHERE st.code=s.code),s.name) AS display_name
 FROM mtgnissa_set s WHERE s.code=?`, code)
	if errors.Is(err, sql.ErrNoRows) {
		return List{}, apperror.New(apperror.CodeSetNotFound, "set not found")
	}
	if err != nil {
		return List{}, err
	}
	if set.Kind == "token" {
		return List{}, apperror.New(apperror.CodeInvalidRequest, "use the tokens endpoint for token sets")
	}
	var lang languageRow
	if err := tx.GetContext(ctx, &lang, "SELECT code, display_name, sort_order FROM scryfall_language WHERE code=?", language); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return List{}, apperror.WithDetails(apperror.CodeValidationFailed, "unknown language", []apperror.Detail{{Field: "language", Reason: "unknown_language"}})
		}
		return List{}, err
	}
	var rows []faceRow
	if err := tx.SelectContext(ctx, &rows, summaryQuery+` WHERE s.code=? AND l.code=?`, set.Code, lang.Code); err != nil {
		return List{}, err
	}
	// Filter the complete printing group so a matching secondary face retains its main-face identity.
	matched := make(map[string]bool)
	if query != "" {
		var ids []string
		pattern := strings.NewReplacer("=", "==", "%", "=%", "_", "=_").Replace(query)
		if err := tx.SelectContext(ctx, &ids, `SELECT p.uuid FROM mtgnissa_card_printing p JOIN mtgnissa_card c ON c.uuid=p.card_uuid
 WHERE c.set_code=? AND p.language_code=? AND p.printed_name LIKE ? ESCAPE '='`, set.Code, lang.Code, "%"+pattern+"%"); err != nil {
			return List{}, err
		}
		for _, id := range ids {
			matched[id] = true
		}
	}
	items := make([]Summary, 0)
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
		return List{}, err
	}
	return List{Set: Set{Code: set.Code, Name: set.Name, DisplayName: set.DisplayName, ReleasedAt: set.ReleasedAt}, Language: Language{Code: lang.Code, DisplayName: lang.DisplayName, SortOrder: lang.SortOrder}, Items: items}, nil
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

func (s *CardStore) printing(ctx context.Context, id string) (Summary, []Face, error) {
	tx, err := s.db.BeginTxx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return Summary{}, nil, err
	}
	defer tx.Rollback()
	rows, err := readPrinting(ctx, tx, id)
	if err != nil {
		return Summary{}, nil, err
	}
	card, faces := summarize(rows)
	if err := tx.Commit(); err != nil {
		return Summary{}, nil, err
	}
	return card, faces, nil
}
