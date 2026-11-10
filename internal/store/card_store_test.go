package store

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/doug-martin/goqu/v9"

	"mtgnissa/internal/model"
)

func TestPrintingReferenceCapturesQueryMetadata(t *testing.T) {
	now := time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)
	released := now.AddDate(-10, 0, 0)
	setReleased := released.AddDate(0, 0, -1)
	oracle := "00000000-0000-0000-0000-000000000001"
	card := model.CardSummary{PrintingUUID: "00000000-0000-0000-0000-000000000002", OracleID: &oracle,
		Name: "Query Name", DisplayName: "Display Name", CollectorNumber: "107a", CMC: 2.5, Colors: []string{"U", "G"}, Rarity: "rare",
		Set: model.SetRef{Code: "TEST", ReleasedAt: &setReleased}, Language: model.Language{Code: "en"}, ReleasedAt: &released}
	ref := newPrintingRef(card, now)
	if ref.PrintingUUID != card.PrintingUUID || ref.OracleID == nil || *ref.OracleID != oracle || ref.SetCode != "TEST" || ref.Lang != "en" || ref.Name != "Query Name" || ref.CollectorNumber != "107a" || len(ref.CollectorSortKey) == 0 || ref.ColorsMask != 18 || ref.CMC != 2.5 || ref.Rarity != "rare" || !ref.ReleasedAt.Equal(released) || !ref.SetReleasedAt.Equal(setReleased) || !ref.IsReal || !ref.CreatedAt.Equal(now) || !ref.LastVerifiedAt.Equal(now) {
		t.Fatalf("query metadata=%+v", ref)
	}
	ref = newPrintingRef(model.CardSummary{}, now)
	if ref.OracleID != nil || ref.ReleasedAt != nil || ref.SetReleasedAt != nil || ref.ColorsMask != 0 || ref.CollectorSortKey == nil || !ref.IsReal {
		t.Fatalf("nullable metadata=%+v", ref)
	}
}

func TestPrintingReferenceRefreshIsAtomicAndPreservesCreationTime(t *testing.T) {
	for _, field := range []string{"oracle_id", "set_code", "collector_number", "collector_sort_key", "lang", "name", "colors_mask", "cmc", "rarity", "released_at", "set_released_at", "last_verified_at"} {
		if !strings.Contains(refreshPrintingRefSQL, field+"=:"+field) {
			t.Fatalf("refresh does not update %s", field)
		}
	}
	if !strings.Contains(refreshPrintingRefSQL, "is_real=TRUE") || !strings.Contains(refreshPrintingRefSQL, "WHERE printing_uuid=:printing_uuid AND is_real=FALSE") || strings.Contains(refreshPrintingRefSQL, "created_at=") {
		t.Fatalf("invalid refresh scope: %s", refreshPrintingRefSQL)
	}
}

func TestCollectorSortKeyMatchesNaturalComparison(t *testing.T) {
	numbers := []string{"", "0", "00", "000a", "0a", "1", "01", "001", "1a", "1A", "1.2", "1.10", "2", "10", "107", "107a", "107b", "107A", "107†", "107★", "108", "A1", "A01", "A2", "A10", "A-2", "A-10", "(1)", "/1", ":1", "中1", "中2", "中10", "107a2", "107a10", strings.Repeat("9", 64)}
	for value := byte(0); value < 127; value++ {
		numbers = append(numbers, string([]byte{value}), "1"+string([]byte{value})+"02")
	}
	for _, a := range numbers {
		for _, b := range numbers {
			actual := bytes.Compare(collectorSortKey(a), collectorSortKey(b))
			expected := model.CompareCollectorNumbers(a, b)
			if actual != expected {
				t.Fatalf("%q vs %q: binary=%d natural=%d", a, b, actual, expected)
			}
		}
	}
	for _, number := range []string{strings.Repeat("1a", 32), strings.Repeat("1中", 32), strings.Repeat("1𐀀", 32)} {
		if len(collectorSortKey(number)) > 512 {
			t.Fatalf("64-character number exceeds key column: %q", number)
		}
	}
}

func TestPrintingIdentityIsIndependentOfImageFallback(t *testing.T) {
	id := "01234567-1234-1234-1234-123456789abc"
	rows := []faceRow{
		{UUID: "back-print", SourceID: id, ImageID: id, Index: 1, Name: "Front // Back", FaceName: "Back", DisplayName: "背", Layout: "transform", FaceCategory: "double_sided", LanguageCode: "ja"},
		{UUID: "front-print", SourceID: id, ImageID: id, Index: 0, Name: "Front // Back", FaceName: "Front", DisplayName: "正", Layout: "transform", FaceCategory: "double_sided", LanguageCode: "ja", Colors: 18},
	}
	card, faces := summarize(rows)
	if card.PrintingUUID != "front-print" || card.Language.Code != "ja" || len(faces) != 2 || faces[0].FaceIndex != 0 || faces[1].FaceIndex != 1 {
		t.Fatalf("identity or face order changed: %#v %#v", card, faces)
	}
	if card.Images.FrontURL == nil || *card.Images.FrontURL != "https://cards.scryfall.io/normal/front/0/1/"+id+".jpg" || card.Images.BackURL == nil || faces[1].ImageURL == nil {
		t.Fatalf("double-faced image resolution failed: %#v", card.Images)
	}
	if !reflect.DeepEqual(card.Colors, []string{"U", "G"}) {
		t.Fatalf("colors=%v", card.Colors)
	}
	rows[0].Layout, rows[0].FaceCategory = "split", "single_sided_multi_face"
	rows[1].Layout, rows[1].FaceCategory = "split", "single_sided_multi_face"
	card, faces = summarize(rows)
	if card.Images.BackURL != nil || faces[1].ImageURL != nil {
		t.Fatal("split card invented an independent back")
	}
}

func TestLanguagePrintingSQLRetainsFacesWithoutLanguageEntries(t *testing.T) {
	for _, query := range []string{summaryQuery, printingQuery} {
		query = strings.Join(strings.Fields(query), " ")
		for _, required := range []string{
			"p.uuid AS printing_uuid",
			"JOIN mtgnissa_card main ON main.uuid=p.card_uuid AND main.face_index IN (-1,0)",
			"JOIN mtgnissa_card c ON c.scryfall_id=main.scryfall_id",
			"LEFT JOIN mtgnissa_card_printing fp ON fp.card_uuid=c.uuid AND fp.language_code=p.language_code",
			"COALESCE(NULLIF(fp.scryfall_id, ''), c.scryfall_id) AS image_id",
			"l.code = p.language_code",
		} {
			if !strings.Contains(query, required) {
				t.Fatalf("missing identity/face expansion rule %q in %s", required, query)
			}
		}
		from := query[strings.LastIndex(query, "FROM mtgnissa_card_printing p"):]
		where := strings.Index(from, " WHERE ")
		if strings.Contains(query, "fp.uuid AS printing_uuid") || where >= 0 && strings.Contains(from[where:], "fp.language_code=") {
			t.Fatalf("optional face language became an identity or row filter: %s", query)
		}
	}
}

func TestMissingSecondaryLanguageKeepsCompleteFacesAndMainIdentity(t *testing.T) {
	sourceID := "01234567-1234-1234-1234-123456789abc"
	languageImageID := "abcdef01-1234-1234-1234-123456789abc"
	frontType, backType := "Sorcery", "Land — Forest"
	backText := "Base back-face text"
	rows := []faceRow{
		{UUID: "ja-main", SourceID: sourceID, ImageID: sourceID, Index: 1, Name: "Spell // Land", FaceName: "Land", DisplayName: "Land", Layout: "modal_dfc", FaceCategory: "double_sided", LanguageCode: "ja", TypeLine: &backType, Text: &backText},
		{UUID: "ja-main", SourceID: sourceID, ImageID: languageImageID, Index: 0, Name: "Spell // Land", FaceName: "Spell", DisplayName: "Spell", Layout: "modal_dfc", FaceCategory: "double_sided", LanguageCode: "ja", TypeLine: &frontType, CMC: 3, Colors: 2},
	}
	card, faces := summarize(rows)
	if card.PrintingUUID != "ja-main" || card.Language.Code != "ja" || len(faces) != 2 || card.DisplayName != "Spell // Land" || card.CMC != 3 || !reflect.DeepEqual(card.Colors, []string{"U"}) {
		t.Fatalf("main identity or complete summary lost: card=%+v faces=%+v", card, faces)
	}
	if faces[1].FaceIndex != 1 || faces[1].TypeLine == nil || *faces[1].TypeLine != backType || faces[1].OracleText == nil || *faces[1].OracleText != backText {
		t.Fatalf("base secondary-face attributes were dropped: %+v", faces[1])
	}
	if !reflect.DeepEqual(card.Images.FrontURL, picture(languageImageID, "front")) || !reflect.DeepEqual(faces[1].ImageURL, picture(sourceID, "back")) {
		t.Fatalf("language/base image fallback changed: card=%+v back=%+v", card.Images, faces[1])
	}
}

func TestGroupingKeepsLanguagesAndPrintingsSeparate(t *testing.T) {
	rows := []faceRow{{SourceID: "one", LanguageCode: "en", Index: 0}, {SourceID: "one", LanguageCode: "en", Index: 1}, {SourceID: "one", LanguageCode: "ja"}, {SourceID: "two", LanguageCode: "en"}}
	if len(groupFaces(rows)) != 3 {
		t.Fatal("different languages or printings merged")
	}
	items := []model.CardSummary{{PrintingUUID: "z", CollectorNumber: "10"}, {PrintingUUID: "b", CollectorNumber: "2"}, {PrintingUUID: "a", CollectorNumber: "2"}, {PrintingUUID: "c", CollectorNumber: "2a"}}
	sortSummaries(items)
	var ids []string
	for _, item := range items {
		ids = append(ids, item.PrintingUUID)
	}
	if !reflect.DeepEqual(ids, []string{"a", "b", "c", "z"}) {
		t.Fatalf("order=%v", ids)
	}
}

func TestSearchLanguageSelectionDoesNotDependOnDatabaseRowOrder(t *testing.T) {
	rows := []searchEntryRow{
		{PrintingUUID: "english", LanguageCode: "en", LanguageOrder: 1},
		{PrintingUUID: "chinese", LanguageCode: "zhs", LanguageOrder: 10},
		{PrintingUUID: "later-language", LanguageCode: "xx", LanguageOrder: 20},
	}
	for start := range rows {
		selected := rows[start]
		for offset := 1; offset < len(rows); offset++ {
			candidate := rows[(start+offset)%len(rows)]
			if preferredSearchEntry(candidate, selected) {
				selected = candidate
			}
		}
		if selected.PrintingUUID != "english" {
			t.Fatalf("selected=%+v", selected)
		}
	}
	for _, pair := range [][2]searchEntryRow{
		{{PrintingUUID: "z", LanguageCode: "en", LanguageOrder: 1}, {PrintingUUID: "a", LanguageCode: "fr", LanguageOrder: 1}},
		{{PrintingUUID: "a", LanguageCode: "en", LanguageOrder: 1}, {PrintingUUID: "z", LanguageCode: "en", LanguageOrder: 1}},
	} {
		if !preferredSearchEntry(pair[0], pair[1]) || preferredSearchEntry(pair[1], pair[0]) {
			t.Fatalf("language or UUID tie ordering changed: %+v", pair)
		}
	}
}

func TestGlobalSearchCombinesIndependentOracleSetsBeforeRepresentativeSelection(t *testing.T) {
	expression := &model.SearchExpression{Kind: "and", Left: &model.SearchExpression{Kind: "atom", Condition: model.SearchCondition{Field: "t", Operator: ":", Value: "sorcery"}}, Right: &model.SearchExpression{Kind: "atom", Condition: model.SearchCondition{Field: "t", Operator: ":", Value: "land"}}}
	hits, err := searchOracleSet(expression)
	if err != nil {
		t.Fatal(err)
	}
	query, args, err := hits.ToSQL()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(query, "SELECT DISTINCT `f`.`oracle_id`") != 2 || !strings.Contains(query, "`left_set`.`oracle_id` = `right_set`.`oracle_id`") {
		t.Fatalf("different faces were forced into one scope: %s", query)
	}
	if strings.Contains(query, "scryfall_id") || strings.Contains(query, "GROUP BY") || strings.Contains(query, "mtgnissa_card_printing") {
		t.Fatalf("hit evaluation unexpectedly used language printings: %s", query)
	}
	if len(args) == 0 || strings.Contains(query, "sorcery%") || strings.Contains(query, "land%") {
		t.Fatalf("search values were not parameterized: query=%s args=%v", query, args)
	}
	expression.Kind = "or"
	union, err := searchOracleSet(expression)
	if err != nil {
		t.Fatal(err)
	}
	unionSQL, _, err := union.ToSQL()
	if err != nil || !strings.Contains(unionSQL, " UNION ") || strings.Contains(unionSQL, "UNION ALL") {
		t.Fatalf("OR must deduplicate Oracle identities: %s error=%v", unionSQL, err)
	}
	position := &SearchPosition{Name: "alpha", OracleID: "oracle"}
	dataset := searchRepresentativeSources([]string{"oracle-one", "oracle-two"})
	sourcesSQL, sourceArgs, err := dataset.ToSQL()
	if err != nil || !strings.Contains(sourcesSQL, "MIN(`c`.`scryfall_id`)") || !strings.Contains(sourcesSQL, "GROUP BY `c`.`oracle_id`") || !strings.Contains(sourcesSQL, "`c`.`oracle_id` IN (?, ?)") || strings.Contains(sourcesSQL, "LIKE") || !reflect.DeepEqual(sourceArgs, []any{"oracle-one", "oracle-two"}) {
		t.Fatalf("representatives must reuse hit IDs across all printings: %s args=%v error=%v", sourcesSQL, sourceArgs, err)
	}
	query, _, err = searchRepresentatives(dataset, position).Limit(21).ToSQL()
	if err != nil || !strings.Contains(query, "LEFT JOIN `mtgnissa_card` AS `main`") || !strings.Contains(query, "`main`.`face_index` IN") || !strings.Contains(query, "`matched`.`oracle_id` >") || strings.Contains(query, "released_at") {
		t.Fatalf("representative/cursor selection=%s error=%v", query, err)
	}
}

func TestGlobalSearchNumericFieldsExcludeNonnumericValuesWithoutRuntimeCasts(t *testing.T) {
	for _, field := range []string{"mv", "pow", "tou", "pt", "loy"} {
		for _, op := range []string{"=", "!=", ">", ">=", "<", "<="} {
			condition, err := searchCondition(model.SearchCondition{Field: field, Operator: op, Value: "2.5"})
			if err != nil {
				t.Fatal(err)
			}
			query, _, err := mysqlDialect.From(goqu.T("mtgnissa_card").As("f")).Where(condition).Prepared(true).ToSQL()
			if err != nil || strings.Contains(query, "CAST") || strings.Contains(query, "REGEXP") || field != "mv" && !strings.Contains(query, "_numeric`") {
				t.Fatalf("numeric condition=%s error=%v", query, err)
			}
		}
	}
	condition, err := searchCondition(model.SearchCondition{Field: "pow", Operator: ">", ValueField: "tou"})
	if err != nil {
		t.Fatal(err)
	}
	query, _, err := mysqlDialect.From("mtgnissa_card").Where(condition).Prepared(true).ToSQL()
	if err != nil || !strings.Contains(query, "`f`.`power_numeric` > `f`.`toughness_numeric`") {
		t.Fatalf("same-face comparison=%s error=%v", query, err)
	}
}

func TestGlobalSearchUsesExactKeywordsTagsSetNamesAndFixedIsTypes(t *testing.T) {
	for _, test := range []struct {
		Condition model.SearchCondition
		Fragment  string
	}{
		{model.SearchCondition{Field: "kw", Value: "flying"}, "scryfall_card_keyword"},
		{model.SearchCondition{Field: "otag", Value: `quoted " tag`}, "oracle_tag_mapping"},
		{model.SearchCondition{Field: "set", Value: "Kaladesh"}, "mtgnissa_set"},
		{model.SearchCondition{Field: "is", Value: "historic"}, "scryfall_card_type"},
		{model.SearchCondition{Field: "is", Value: "permanent"}, "scryfall_card_type"},
		{model.SearchCondition{Field: "r", Operator: ">", Value: "rare"}, "CASE"},
		{model.SearchCondition{Field: "c", Operator: "<=", ColorMask: 7}, "&"},
	} {
		condition, err := searchCondition(test.Condition)
		if err != nil {
			t.Fatal(err)
		}
		query, _, err := mysqlDialect.From("mtgnissa_card").Where(condition).Prepared(true).ToSQL()
		if err != nil || !strings.Contains(query, test.Fragment) {
			t.Fatalf("condition=%s error=%v", query, err)
		}
		if test.Condition.Field == "otag" && (strings.Contains(query, "LIKE") || strings.Contains(query, "weight") || strings.Contains(query, "aliases") || strings.Contains(query, "oracle_tag_relation")) {
			t.Fatalf("tag query exceeded exact label matching: %s", query)
		}
	}
}
