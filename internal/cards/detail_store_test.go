package cards

import (
	"context"
	"database/sql/driver"
	"testing"
	"time"
)

func TestDetailLoadsFacesVersionsRulingsAndEscapedTags(t *testing.T) {
	columns := []string{"printing_uuid", "source_id", "image_id", "face_index", "oracle_id", "name", "face_name", "display_name", "set_code", "set_name", "set_display_name", "collector_number", "language_code", "language_name", "language_order", "released_at"}
	date := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	main := []driver.Value{"ja-print", "source", "", int64(0), "oracle", "Front // Back", "Front", "正", "SET", "Set", "系列", "1", "ja", "日文", int64(7), date}
	back := append([]driver.Value{}, main...)
	back[0], back[3], back[6], back[7] = "ja-back", int64(1), "Back", "背"
	english := append([]driver.Value{}, main...)
	english[0], english[12], english[13], english[14] = "en-print", "en", "英文", int64(1)
	special := append([]driver.Value{}, main...)
	special[0], special[12], special[13], special[14] = "ph-print", "ph", "非瑞", int64(17)
	other := append([]driver.Value{}, english...)
	other[0], other[8], other[9], other[10], other[15] = "other-print", "OTHER", "Other", "其他", date.AddDate(1, 0, 0)
	db, backend := testDatabase(t, []dbStep{
		{query: "JOIN mtgnissa_card_printing requested ON requested.uuid=?", columns: columns, rows: [][]driver.Value{back, main}, args: []any{"ja-print"}},
		{query: "WHERE c.oracle_id=? AND c.face_index IN (-1,0)", columns: columns, rows: [][]driver.Value{main, special, other, english},
			absent: []string{"GROUP BY", "oracle_text", "flavor_text", "card_oracle_translation_mapping", "card_flavor_translation_mapping"}},
		{query: "ORDER BY kind ASC,published_at DESC,entry_id ASC", columns: []string{"kind", "published_at", "entry_id", "text"},
			rows: [][]driver.Value{{"ruling", date, "ruling", `译文\n原样`}, {"tag", nil, "tag", `Ramp "quoted" \ label`}}, args: []any{"oracle", "oracle"}},
	})
	d, err := NewCardStore(db).detail(context.Background(), "ja-print")
	if err != nil {
		t.Fatal(err)
	}
	if d.Card.PrintingUUID != "ja-print" || len(d.Faces) != 2 || d.Faces[0].FaceIndex != 0 || len(d.Languages) != 3 || d.Languages[0].Language.Code != "en" || d.Languages[2].Language.Code != "ph" {
		t.Fatalf("faces or unrestricted language options=%#v", d)
	}
	if len(d.OtherVersions) != 1 || d.OtherVersions[0].Set.Code != "OTHER" || len(d.OtherVersions[0].Languages) != 1 {
		t.Fatalf("versions=%#v", d.OtherVersions)
	}
	if len(d.Rulings) != 1 || d.Rulings[0].Text != `译文\n原样` || len(d.Tags) != 1 || d.Tags[0].Query != `otag:"Ramp \"quoted\" \\ label"` {
		t.Fatalf("Oracle context rulings=%#v tags=%#v", d.Rulings, d.Tags)
	}
	if !backend.committed || !backend.options.ReadOnly {
		t.Fatal("Card DB detail read was not a consistent read-only transaction")
	}
}

func TestMissingOrSecondaryPrintingHasNoDetail(t *testing.T) {
	db, _ := testDatabase(t, []dbStep{{query: "seed.face_index IN (-1,0)", columns: []string{"printing_uuid"}, args: []any{"secondary"}}})
	_, err := NewCardStore(db).detail(context.Background(), "secondary")
	if err == nil {
		t.Fatal("missing canonical printing was accepted")
	}
}

func TestNullOraclePersonalStateRemainsScopedToPrinting(t *testing.T) {
	d := Detail{Card: Summary{PrintingUUID: "print"}, Languages: []LanguageOption{{PrintingUUID: "print"}}}
	db, _ := testDatabase(t, []dbStep{
		{query: "CASE WHEN ? IS NULL THEN COALESCE(c.normal_quantity,0)", columns: []string{"normal_quantity", "foil_quantity", "logical_normal", "logical_foil", "included"},
			rows: [][]driver.Value{{int64(2), int64(1), int64(2), int64(1), false}}, args: []any{nil, nil, "print", nil}},
		{query: "WHERE printing_uuid IN", columns: []string{"printing_uuid", "normal_quantity", "foil_quantity"}, rows: [][]driver.Value{{"print", int64(2), int64(1)}}},
	})
	if err := NewAppStore(db).fillDetail(context.Background(), &d); err != nil {
		t.Fatal(err)
	}
	if d.LogicalCollection != d.Collection || d.Collection.Normal != 2 || d.Included || len(d.Decks) != 0 {
		t.Fatalf("null Oracle state was changed: %#v", d)
	}
}

func TestNullOracleDoesNotQueryOracleTables(t *testing.T) {
	columns := []string{"printing_uuid", "source_id", "face_index", "oracle_id", "set_code", "collector_number", "language_code"}
	main := []driver.Value{"print", "source", int64(-1), nil, "SET", "1", "en"}
	db, _ := testDatabase(t, []dbStep{
		{query: "JOIN mtgnissa_card_printing requested ON requested.uuid=?", columns: columns, rows: [][]driver.Value{main}, args: []any{"print"}},
		{query: "WHERE c.set_code=? AND c.collector_number=?", columns: columns, rows: [][]driver.Value{main}},
	})
	d, err := NewCardStore(db).detail(context.Background(), "print")
	if err != nil || d.Tags == nil || d.Rulings == nil || len(d.Tags) != 0 || len(d.Rulings) != 0 || len(d.OtherVersions) != 0 {
		t.Fatalf("null Oracle context=%#v error=%v", d, err)
	}
}

func TestDetailJoinsPersonalQuantitiesAndDecksInOneSnapshot(t *testing.T) {
	oracle := "oracle"
	d := Detail{Card: Summary{PrintingUUID: "ja-print", OracleID: &oracle},
		Languages:     []LanguageOption{{PrintingUUID: "en-print"}, {PrintingUUID: "ja-print"}},
		OtherVersions: []Version{{Languages: []LanguageOption{{PrintingUUID: "other-en"}, {PrintingUUID: "other-ja"}}}}, Decks: []DeckUsage{}}
	db, backend := testDatabase(t, []dbStep{
		{query: "WHERE r.oracle_id=?", columns: []string{"normal_quantity", "foil_quantity", "logical_normal", "logical_foil", "included"},
			rows: [][]driver.Value{{int64(2), int64(1), int64(7), int64(4), true}}, args: []any{"oracle", "oracle", "ja-print", "oracle"}},
		{query: "WHERE printing_uuid IN", columns: []string{"printing_uuid", "normal_quantity", "foil_quantity"}, rows: [][]driver.Value{{"ja-print", int64(2), int64(1)}, {"other-en", int64(3), int64(2)}, {"other-ja", int64(2), int64(1)}}},
		{query: "AND c.section_type IN ('commander','mainboard','sideboard')", columns: []string{"id", "name", "quantity", "section_type"}, rows: [][]driver.Value{{int64(12), "Deck", int64(1), "mainboard"}, {int64(12), "Deck", int64(2), "sideboard"}, {int64(13), "Other", int64(1), "commander"}}},
	})
	if err := NewAppStore(db).fillDetail(context.Background(), &d); err != nil {
		t.Fatal(err)
	}
	if d.Collection.Normal != 2 || d.LogicalCollection.Normal != 7 || !d.Included || d.Languages[0].Quantity.Normal != 0 || d.Languages[1].Quantity.Normal != 2 {
		t.Fatalf("personal quantities=%#v", d)
	}
	if d.OtherVersions[0].Quantity.Normal != 5 || d.OtherVersions[0].Quantity.Foil != 3 || len(d.Decks) != 2 || d.Decks[0].ID != 12 || d.Decks[0].Quantity != 3 || len(d.Decks[0].Sections) != 2 || !backend.committed {
		t.Fatalf("version or deck totals=%#v %#v", d.OtherVersions, d.Decks)
	}
}
