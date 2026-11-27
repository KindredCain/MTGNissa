package decks

import (
	"context"
	"errors"
	"math"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"mtgnissa/internal/apperror"
	"mtgnissa/internal/model"
)

func TestDeckContextKeepsEveryFace(t *testing.T) {
	front, back := "front text", "back text"
	data := model.DeckCardData{Faces: []model.CardFace{
		{FaceIndex: 0, Name: "Front", OracleText: &front},
		{FaceIndex: 1, Name: "Back", OracleText: &back},
	}}
	context := editContext(data, model.DeckPersonal{}, nil, "oracle")
	if !reflect.DeepEqual(context.Faces, data.Faces) {
		t.Fatalf("incomplete deck faces: %+v", context.Faces)
	}
}

func TestStandardCodeFollowsColorWheelOrder(t *testing.T) {
	order := []string{
		"W", "U", "B", "R", "G",
		"WU", "UB", "BR", "RG", "GW", "WB", "UR", "BG", "RW", "GU",
		"WUB", "UBR", "BRG", "RGW", "GWU", "WBG", "URW", "BGU", "RWB", "GUR",
		"WUBR", "UBRG", "BRGW", "RGWU", "GWUB", "WUBRG", "C",
	}
	deck := model.SavedDeck{Name: "Color wheel"}
	data := make(map[string]model.DeckCardData)
	typeLine := "Creature"
	for i := len(order) - 1; i >= 0; i-- {
		name := order[i]
		colors := []string{}
		if name != "C" {
			for j := len(name) - 1; j >= 0; j-- {
				colors = append(colors, name[j:j+1])
			}
		}
		card := model.DeckCardData{Card: model.CardSummary{PrintingUUID: name, Name: name, CMC: 2, Colors: colors}, Faces: []model.CardFace{{TypeLine: &typeLine}}}
		data[name] = card
		deck.Cards = append(deck.Cards, model.SavedDeckCard{Section: "mainboard", OracleID: name, Name: name, Quantity: 1})
	}
	want := "About\nName Color wheel\n\nDeck\n1 " + strings.Join(order, "\n1 ")
	if actual := standardCode(deck, data); actual != want {
		t.Fatalf("standard code order:\n%s\nwant:\n%s", actual, want)
	}
	// Color order must remain below type and mana value, and above name/UUID ties.
	blue := data["U"]
	white := data["W"]
	blue.Card.CMC = 1
	if !codeCardLess(blue, white) {
		t.Fatal("color order overrode mana value")
	}
	artifact := "Artifact"
	blue.Card.CMC = 99
	blue.Faces[0].TypeLine = &artifact
	if !codeCardLess(blue, white) {
		t.Fatal("color order overrode type order")
	}
	tied := []model.DeckCardData{
		{Card: model.CardSummary{Name: "beta", PrintingUUID: "a"}},
		{Card: model.CardSummary{Name: "Alpha", PrintingUUID: "b"}},
		{Card: model.CardSummary{Name: "alpha", PrintingUUID: "a"}},
	}
	sort.Slice(tied, func(i, j int) bool {
		return codeCardLess(tied[i], tied[j])
	})
	if tied[0].Card.Name != "alpha" || tied[1].Card.Name != "Alpha" || tied[2].Card.Name != "beta" {
		t.Fatal("name or UUID tie order changed")
	}
}

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

func TestCodeNamesResolveFullAndMainFaceNamesWithoutLosingCanonicalNames(t *testing.T) {
	source := "Deck\n1 Fell the Profane // Fell Mire (MH3) 244 *F*\n2 Fell the Profane\nSideboard\n1 FIRE//ICE\n1 Sol Ring 123"
	_, lines, issues := parseLines(source)
	if len(issues) != 0 || len(lines) != 4 {
		t.Fatalf("lines=%+v issues=%+v", lines, issues)
	}
	fell, fire, ring := "fell", "fire", "ring"
	resolved := []model.DeckName{
		{MatchedName: "Fell the Profane", Name: "Fell the Profane // Fell Mire", OracleID: &fell},
		{MatchedName: "Fell the Profane", Name: "Fell the Profane // Fell Mire", OracleID: &fell},
		{MatchedName: "Fire", Name: "Fire // Ice", OracleID: &fire},
		{MatchedName: "Sol Ring", Name: "Sol Ring", OracleID: &ring},
	}
	entries, entryLines, issues := resolveCodeLines(lines, resolved)
	if len(issues) != 0 || len(entries) != 4 || !reflect.DeepEqual(entryLines, lines) {
		t.Fatalf("entries=%+v lines=%+v issues=%+v", entries, entryLines, issues)
	}
	if entries[0].OracleID != fell || entries[1].OracleID != fell || entries[0].Name != resolved[0].Name || entries[1].Quantity != 2 || entries[2].Name != "Fire // Ice" || entries[2].Section != "sideboard" || entries[3].OracleID != ring {
		t.Fatalf("resolved names or section quantities changed: %+v", entries)
	}
	if lines[0].Name != "Fell the Profane // Fell Mire" || lines[3].FallbackName != "Sol Ring" {
		t.Fatalf("original name or collector fallback was lost: %+v", lines)
	}
}

func TestCodeNameFailuresKeepOriginalInputsAndOracleAmbiguity(t *testing.T) {
	_, lines, issues := parseLines("1 Missing // Back\n1 Shared // Back\n1 Unsupported\n1 Card 123")
	if len(issues) != 0 {
		t.Fatal(issues)
	}
	first, second, numbered := "first", "second", "numbered"
	resolved := []model.DeckName{
		{MatchedName: "Shared", Name: "Shared // Back", OracleID: &first},
		{MatchedName: "Shared", Name: "Shared // Other", OracleID: &second},
		{MatchedName: "Unsupported", Name: "Unsupported"},
		{MatchedName: "Card 123", Name: "Card 123", OracleID: &numbered},
		{MatchedName: "Card", Name: "Card", OracleID: &first},
	}
	entries, entryLines, issues := resolveCodeLines(lines, resolved)
	want := []apperror.Detail{
		{Line: 1, Input: "Missing // Back", Reason: "card_not_found"},
		{Line: 2, Input: "Shared // Back", Reason: "card_name_not_unique"},
		{Line: 3, Input: "Unsupported", Reason: "deck_card_not_supported"},
	}
	if !reflect.DeepEqual(issues, want) || len(entries) != 1 || entries[0].OracleID != numbered || len(entryLines) != 1 || entryLines[0].Line != 4 {
		t.Fatalf("entries=%+v lines=%+v issues=%+v", entries, entryLines, issues)
	}
	_, lines, issues = parseLines("1 // Back")
	if len(lines) != 0 || len(issues) != 1 || issues[0].Reason != "card_not_found" {
		t.Fatalf("empty main name accepted: lines=%+v issues=%+v", lines, issues)
	}
}

func TestStandardCodeNameFormatsRoundTripThroughImport(t *testing.T) {
	for _, test := range []struct {
		Layout string
		Name   string
		Export string
	}{
		{"normal", "Sol Ring", "Sol Ring"},
		{"split", "Fire // Ice", "Fire // Ice"},
		{"modal_dfc", "Fell the Profane // Fell Mire", "Fell the Profane"},
		{"transform", "Delver of Secrets // Insectile Aberration", "Delver of Secrets"},
		{"adventure", "Brazen Borrower // Petty Theft", "Brazen Borrower"},
		{"flip", "Bushi Tenderfoot // Kenzo the Hardhearted", "Bushi Tenderfoot"},
	} {
		t.Run(test.Layout, func(t *testing.T) {
			oracle := "oracle"
			deck := model.SavedDeck{Name: "Example", Cards: []model.SavedDeckCard{{Section: "mainboard", OracleID: oracle, Quantity: 2, Name: test.Name}}}
			data := map[string]model.DeckCardData{oracle: {Card: model.CardSummary{Name: test.Name, Layout: test.Layout}}}
			code := standardCode(deck, data)
			if code != "About\nName Example\n\nDeck\n2 "+test.Export {
				t.Fatalf("export=%q", code)
			}
			_, lines, issues := parseLines(code)
			if len(issues) != 0 || len(lines) != 1 {
				t.Fatalf("lines=%+v issues=%+v", lines, issues)
			}
			entries, _, issues := resolveCodeLines(lines, []model.DeckName{{MatchedName: codeMainName(test.Name), Name: test.Name, OracleID: &oracle}})
			if len(issues) != 0 || !reflect.DeepEqual(entries, deck.Cards) || data[oracle].Card.Name != test.Name {
				t.Fatalf("round trip changed logical identity or canonical name: entries=%+v issues=%+v", entries, issues)
			}
		})
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

func TestDeckCursorsBindScope(t *testing.T) {
	date := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	a := model.SavedDeck{ID: 2, Name: "Same", CreatedAt: date, UpdatedAt: date}
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

func TestSelectDeckPrintingsKeepsRequestedOrderAndRejectsMissingOracle(t *testing.T) {
	candidates := []model.DeckPrinting{
		{UUID: "fallback", OracleID: "first"},
		{UUID: "owned", OracleID: "first"},
		{UUID: "second-printing", OracleID: "second"},
	}
	ids, err := selectDeckPrintings([]string{"second", "first"}, candidates, map[string]model.DeckPrintingState{"owned": {Owned: true}})
	if err != nil || len(ids) != 2 || ids[0] != "second-printing" || ids[1] != "owned" {
		t.Fatalf("selected=%v error=%v", ids, err)
	}
	if _, err := selectDeckPrintings([]string{"missing"}, candidates, nil); err == nil {
		t.Fatal("missing Oracle printing accepted")
	}
}

func TestDeckPrintingPreferenceUsesTierThenDateThenUUID(t *testing.T) {
	oldDate := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	newDate := oldDate.AddDate(20, 0, 0)
	owned := model.DeckPrinting{UUID: "owned", ReleasedAt: &oldDate}
	wishlist := model.DeckPrinting{UUID: "wish", ReleasedAt: &newDate}
	latest := model.DeckPrinting{UUID: "latest", ReleasedAt: &newDate}
	states := map[string]model.DeckPrintingState{"owned": {Owned: true, Included: true}, "wish": {Included: true}}
	if !preferredDeckPrinting(owned, wishlist, states) || !preferredDeckPrinting(wishlist, latest, states) || preferredDeckPrinting(latest, owned, states) {
		t.Fatal("printing preference ignored ownership or wishlist")
	}
	if !preferredDeckPrinting(latest, model.DeckPrinting{UUID: "a", ReleasedAt: &oldDate}, nil) || !preferredDeckPrinting(model.DeckPrinting{UUID: "a", ReleasedAt: &newDate}, latest, nil) {
		t.Fatal("printing date or UUID tie ordering changed")
	}
}
