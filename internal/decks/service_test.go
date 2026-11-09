package decks

import (
	"context"
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"mtgnissa/internal/apperror"
	"mtgnissa/internal/model"
)

func TestDeckNamesRejectInternalLineBreaksBeforeMutation(t *testing.T) {
	s := &Service{}
	for _, name := range []string{"A\nDeck\n1 Sol Ring", "A\rB", "A\r\nB", "A\vB", "A\fB", "A\u0085B", "A\u2028B", "A\u2029B"} {
		_, _, createErr := s.Create(context.Background(), name, "unused")
		_, saveErr := s.Save(context.Background(), 1, model.DeckSave{Name: name})
		for _, err := range []error{createErr, saveErr} {
			var appErr *apperror.Error
			if !errors.As(err, &appErr) || appErr.Code != apperror.CodeValidationFailed || len(appErr.Details) != 1 || appErr.Details[0].Field != "name" || appErr.Details[0].Reason != "must_not_contain_line_breaks" {
				t.Fatalf("name=%q error=%v", name, err)
			}
		}
		if _, err := normalizeName(copyName(name)); err == nil {
			t.Fatalf("copy accepted invalid historical name %q", name)
		}
	}
	for _, name := range []string{"  Normal Deck  ", "\r\nNormal Deck\r\n", `Literal\nName`} {
		actual, err := normalizeName(name)
		if err != nil || actual != strings.TrimSpace(name) {
			t.Fatalf("valid name=%q actual=%q error=%v", name, actual, err)
		}
	}
}

func TestSaveMergesLogicalCardsAndValidatesCoverAndCommander(t *testing.T) {
	oracle, other := "oracle", "other"
	legendary := "Legendary Planeswalker — Nissa"
	ordinary := "Artifact"
	refs := map[string]model.DeckCardData{
		"en":    {Card: model.CardSummary{OracleID: &oracle, Name: "Nissa"}, Faces: []model.CardFace{{TypeLine: &legendary}}},
		"ja":    {Card: model.CardSummary{OracleID: &oracle, Name: "Nissa"}, Faces: []model.CardFace{{TypeLine: &legendary}}},
		"other": {Card: model.CardSummary{OracleID: &other, Name: "Other"}, Faces: []model.CardFace{{TypeLine: &ordinary}}},
		"null":  {Card: model.CardSummary{Name: "No Oracle"}},
	}
	cover := "ja"
	input := model.DeckSave{Name: "Example", CoverPrintingUUID: &cover, Sections: []model.DeckSaveSection{{Type: "commander", Cards: []model.DeckSaveCard{{PrintingUUID: "en", Quantity: 1}, {PrintingUUID: "ja", Quantity: 2}}}}}
	saved, err := normalizeSave(1, "Example", input, refs)
	if err != nil || len(saved.Cards) != 1 || saved.Cards[0].Quantity != 3 || saved.CoverOracleID == nil || *saved.CoverOracleID != oracle {
		t.Fatalf("merged=%#v error=%v", saved, err)
	}
	for _, test := range []struct {
		ID     string
		Code   apperror.Code
		Reason string
	}{
		{"missing", apperror.CodeCardReferenceInvalid, "card_reference_invalid"},
		{"null", apperror.CodeDeckCardNotSupported, "deck_card_not_supported"},
		{"other", apperror.CodeValidationFailed, "commander_card_not_eligible"},
	} {
		bad := model.DeckSave{Sections: []model.DeckSaveSection{{Type: "commander", Cards: []model.DeckSaveCard{{PrintingUUID: test.ID, Quantity: 1}}}}}
		_, err := normalizeSave(1, "Example", bad, refs)
		var appErr *apperror.Error
		if !errors.As(err, &appErr) || appErr.Code != test.Code || len(appErr.Details) != 1 || appErr.Details[0].Reason != test.Reason || appErr.Details[0].Field != "sections[0].cards[0].printing_uuid" {
			t.Fatalf("%s error=%#v", test.ID, err)
		}
	}
	cover = "other"
	if _, err := normalizeSave(1, "Example", input, refs); err == nil {
		t.Fatal("cover outside deck accepted")
	}
	input.CoverPrintingUUID = nil
	input.Sections[0].Cards[0].Quantity = math.MaxUint32
	if _, err := normalizeSave(1, "Example", input, refs); err == nil {
		t.Fatal("merged quantity overflow accepted")
	}
	if _, err := normalizeSave(1, "Empty", model.DeckSave{}, refs); err != nil {
		t.Fatalf("empty commander or deck rejected: %v", err)
	}
}

func TestStatisticsRespectFacesMainManaAndCommanderEligibility(t *testing.T) {
	sorcery, land := "Sorcery", "Land — Forest"
	data := model.DeckCardData{Card: model.CardSummary{CMC: 3}, Faces: []model.CardFace{{TypeLine: &sorcery}, {TypeLine: &land}}, ProducedMana: []string{"G"}}
	stats := statistics(data)
	if !reflect.DeepEqual(stats.TypeGroups, []string{"land", "sorcery"}) || !stats.IsLand || stats.CanBeCommander || stats.ProducedMana[0] != "G" {
		t.Fatalf("multiface stats=%#v", stats)
	}
	legendary, creature := "Legendary Artifact", "Creature — Vehicle"
	data.Faces = []model.CardFace{{TypeLine: &legendary}, {TypeLine: &creature}}
	if statistics(data).CanBeCommander {
		t.Fatal("commander eligibility combined different faces")
	}
	vehicle := "Legendary Artifact — Vehicle"
	data.Faces = []model.CardFace{{TypeLine: &vehicle}}
	if !statistics(data).CanBeCommander || statistics(data).TypeGroups[0] != "artifact" {
		t.Fatal("legendary vehicle eligibility changed")
	}
	for _, test := range []struct {
		Cost string
		Want bool
	}{{"{3}", false}, {"{X}", false}, {"{C}{C}", true}, {"{W/C}", true}, {"{2/W}", false}, {"{W/P}", false}} {
		if hasColorlessCost(&test.Cost) != test.Want {
			t.Errorf("colorless classification %s", test.Cost)
		}
	}
}

func TestSavedDeckTotalsExcludeMaybeboardAndKeepUsageBaselines(t *testing.T) {
	oracle := "oracle"
	cost := "{C}{C}"
	data := map[string]model.DeckCardData{oracle: {Card: model.CardSummary{PrintingUUID: "printing", OracleID: &oracle, ManaCost: &cost, Colors: []string{"W", "U"}}}}
	deck := model.SavedDeck{ID: 1, Name: "Test", Cards: []model.SavedDeckCard{{Section: "commander", OracleID: oracle, Quantity: 1}, {Section: "mainboard", OracleID: oracle, Quantity: 2}, {Section: "sideboard", OracleID: oracle, Quantity: 1}, {Section: "maybeboard", OracleID: oracle, Quantity: 99}}}
	personal := model.DeckPersonal{Owned: map[string]uint64{oracle: 4}, AllQuantity: map[string]uint64{oracle: 7}, UsingDecks: map[string]uint64{oracle: 2}}
	detail := buildDetail(deck, data, personal)
	if len(detail.Sections) != 4 || detail.Summary.CountedTotal != 4 || detail.Summary.CollectionRate != 1 || detail.Summary.DeckUsage.DistinctCards != 1 || detail.Summary.DeckUsage.MissingQuantity != 3 {
		t.Fatalf("summary=%#v", detail)
	}
	context := detail.Sections[0].Cards[0].Context
	if context.OtherDecksQuantity != 3 || context.OtherDecksCount != 1 || detail.Sections[3].Cards[0].Exception != nil {
		t.Fatalf("context=%#v", context)
	}
	list := buildListItem(deck, data, personal)
	if list.CoverCard != nil || list.CountedTotal != 4 || list.SectionCounts["maybeboard"] != 99 || list.ColorDistribution["W"] != 3 || list.ColorDistribution["U"] != 3 || list.ColorDistribution["C"] != 3 {
		t.Fatalf("list statistics=%#v", list)
	}
	personal.Owned[oracle] = 2
	detail = buildDetail(deck, data, personal)
	if detail.Summary.CollectionRate != 0.5 || detail.Summary.Shortage.MissingQuantity != 2 || detail.Summary.DeckUsage.DistinctCards != 0 {
		t.Fatalf("shortage priority=%#v", detail.Summary)
	}
}

func TestCodeParsingMetadataSectionsAnnotationsAndLimits(t *testing.T) {
	source := "About\r\nName  Example  \r\n\r\nDeck\r\n2 Sol Ring (CMM) 396 *F*\r\nSideboard\r\n1 Counterspell\r\nMaybeboard\r\n0 Invalid\r\nUnknown section"
	name, lines, issues := parseLines(source)
	if name == nil || *name != "Example" || len(lines) != 2 || lines[0].Name != "Sol Ring" || lines[0].Quantity != 2 || lines[1].Section != "sideboard" || len(issues) != 2 || issues[0].Line != 9 || issues[1].Line != 10 {
		t.Fatalf("name=%v lines=%#v issues=%#v", name, lines, issues)
	}
	_, lines, issues = parseLines("1 Sol Ring\n2 Counterspell")
	if len(issues) != 0 || len(lines) != 2 || lines[0].Section != "mainboard" {
		t.Fatal("plain list was not treated as mainboard")
	}
	name, _, issues = parseLines("About\nName " + strings.Repeat("名", 256) + "\nDeck\n1 Sol Ring")
	if name != nil || len(issues) != 0 {
		t.Fatal("invalid name prevented card parsing")
	}
	for _, source := range []string{" ", "Deck\x00", strings.Repeat("a", 768*1024+1), strings.Repeat("\n", 1000)} {
		if validateCode(source) == nil {
			t.Fatal("invalid code boundary accepted")
		}
	}
}

func TestNoopDefinitionStandardCodeAndUnicodeCopyName(t *testing.T) {
	oracle := "oracle"
	typeLine := "Artifact"
	card := model.SavedDeckCard{Section: "mainboard", OracleID: oracle, Quantity: 2, Name: "Sol Ring"}
	a := model.SavedDeck{Name: "Example", Cards: []model.SavedDeckCard{card}}
	b := a
	b.Cards = append([]model.SavedDeckCard{}, a.Cards...)
	b.Cards[0].Name = "Changed snapshot"
	if !sameDefinition(a, b) {
		t.Fatal("snapshot text caused a spurious save")
	}
	b.Cards[0].Quantity++
	if sameDefinition(a, b) {
		t.Fatal("quantity change was treated as no-op")
	}
	code := standardCode(a, map[string]model.DeckCardData{oracle: {Card: model.CardSummary{Name: "Sol Ring"}, Faces: []model.CardFace{{TypeLine: &typeLine}}}})
	if code != "About\nName Example\n\nDeck\n2 Sol Ring" {
		t.Fatalf("standard code=%q", code)
	}
	name := copyName(strings.Repeat("妮", 255))
	if !utf8.ValidString(name) || utf8.RuneCountInString(name) != 255 || !strings.HasPrefix(name, "复制 ") {
		t.Fatalf("copy name=%q", name)
	}
}

func TestDeckCursorsBindScopeAndUseAscendingIDTies(t *testing.T) {
	date := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	a, b := model.SavedDeck{ID: 2, Name: "Same", CreatedAt: date, UpdatedAt: date}, model.SavedDeck{ID: 3, Name: "SAME", CreatedAt: date, UpdatedAt: date}
	for _, order := range []string{"updated_desc", "created_desc", "name_asc", "card_count_desc"} {
		if compareDeckPositions(deckPosition(a, order), deckPosition(b, order), order) >= 0 {
			t.Fatalf("deck ID tie order changed for %s", order)
		}
	}
	fingerprint := cursorFingerprint("decks", 0, "name", "updated_desc")
	cursor, err := encodeCursor("decks", fingerprint, deckPosition(a, "updated_desc"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeCursor(cursor, fingerprint, "decks"); err != nil {
		t.Fatal(err)
	}
	if _, err := decodeCursor(cursor, cursorFingerprint("decks", 0, "other", "updated_desc"), "decks"); err == nil {
		t.Fatal("cursor accepted a different query")
	}
	if _, err := decodeCursor(cursor, fingerprint, "deck_cards"); err == nil {
		t.Fatal("cursor accepted a different endpoint")
	}
	for _, query := range []string{"ab1", "妮1", "a妮"} {
		if suggestionEligible(query) {
			t.Errorf("short suggestion %q accepted", query)
		}
	}
	for _, query := range []string{"abc", "妮莎", "a妮b莎"} {
		if !suggestionEligible(query) {
			t.Errorf("eligible suggestion %q rejected", query)
		}
	}
}
