package decks

import (
	"context"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"mtgnissa/internal/apperror"
	"mtgnissa/internal/model"
)

type codeLine struct {
	Line         int
	Section      string
	Name         string
	FallbackName string
	Quantity     uint64
}

var cardLinePattern = regexp.MustCompile(`^([0-9]+)\s+(.+)$`)
var printingSuffixPattern = regexp.MustCompile(`\s+\([A-Za-z0-9]{1,16}\)(?:\s+\S+)?$`)
var collectorSuffixPattern = regexp.MustCompile(`\s+[0-9]+[A-Za-z★†]*$`)

// ParseCode resolves every card line before returning a complete, unsaved parsed deck.
func (s *Service) ParseCode(ctx context.Context, id uint64, source string) (model.ParsedDeck, error) {
	if _, err := s.identity(ctx, id); err != nil {
		return model.ParsedDeck{}, safeError(err)
	}
	if err := validateCode(source); err != nil {
		return model.ParsedDeck{}, err
	}
	name, lines, issues := parseLines(source)
	names := []string{}
	for _, line := range lines {
		names = append(names, strings.ToLower(line.Name))
		if line.FallbackName != "" {
			names = append(names, strings.ToLower(line.FallbackName))
		}
	}
	resolved, err := s.cards.DeckNames(ctx, uniqueStrings(names))
	if err != nil {
		return model.ParsedDeck{}, safeError(err)
	}
	byName := map[string]map[string]model.DeckName{}
	for _, card := range resolved {
		key := strings.ToLower(card.Name)
		if byName[key] == nil {
			byName[key] = map[string]model.DeckName{}
		}
		oracle := ""
		if card.OracleID != nil {
			oracle = *card.OracleID
		}
		byName[key][oracle] = card
	}
	entries := []model.SavedDeckCard{}
	entryLines := []codeLine{}
	for _, line := range lines {
		matches := byName[strings.ToLower(line.Name)]
		if len(matches) == 0 && line.FallbackName != "" {
			matches = byName[strings.ToLower(line.FallbackName)]
		}
		reason := ""
		switch len(matches) {
		case 0:
			reason = "card_not_found"
		case 1:
			if _, unsupported := matches[""]; unsupported {
				reason = "deck_card_not_supported"
			}
		default:
			reason = "card_name_not_unique"
		}
		if reason != "" {
			issues = append(issues, apperror.Detail{Line: line.Line, Reason: reason, Input: line.Name})
			continue
		}
		for oracle, match := range matches {
			entries = append(entries, model.SavedDeckCard{Section: line.Section, OracleID: oracle, Name: match.Name, Quantity: line.Quantity})
			entryLines = append(entryLines, line)
		}
	}
	oracles := deckOracles([]model.SavedDeck{{Cards: entries}})
	personal, _, err := s.personal(ctx, id, oracles)
	if err != nil {
		return model.ParsedDeck{}, safeError(err)
	}
	data, err := s.display(ctx, oracles, personal.Preferred, false)
	if err != nil {
		return model.ParsedDeck{}, safeError(err)
	}
	merged := map[[2]string]uint64{}
	for i, card := range entries {
		line := entryLines[i]
		if card.Section == "commander" && !statistics(data[card.OracleID]).CanBeCommander {
			issues = append(issues, apperror.Detail{Line: line.Line, Reason: "commander_card_not_eligible", Input: line.Name})
		}
		key := [2]string{card.Section, card.OracleID}
		if merged[key] > math.MaxUint32-card.Quantity {
			issues = append(issues, apperror.Detail{Line: line.Line, Reason: "quantity_out_of_range", Input: line.Name})
			continue
		}
		merged[key] += card.Quantity
	}
	if len(issues) > 0 {
		sort.SliceStable(issues, func(i, j int) bool {
			return issues[i].Line < issues[j].Line
		})
		return model.ParsedDeck{}, apperror.WithDetails(apperror.CodeDeckParseFailed, "deck code could not be read", issues)
	}
	result := model.ParsedDeck{Name: name, Sections: []model.ParsedDeckSection{}}
	for _, kind := range sectionOrder {
		section := model.ParsedDeckSection{Type: kind, Cards: []model.ParsedDeckCard{}}
		for key, quantity := range merged {
			if key[0] == kind {
				section.Cards = append(section.Cards, model.ParsedDeckCard{Card: data[key[1]].Card, Quantity: quantity})
			}
		}
		sort.Slice(section.Cards, func(i, j int) bool {
			a, b := section.Cards[i].Card, section.Cards[j].Card
			if c := strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)); c != 0 {
				return c < 0
			}
			return a.PrintingUUID < b.PrintingUUID
		})
		result.Sections = append(result.Sections, section)
	}
	return result, nil
}

func validateCode(source string) error {
	if strings.TrimSpace(source) == "" || !utf8.ValidString(source) || strings.ContainsRune(source, 0) {
		return fieldError("source_text", "must_be_nonblank_utf8_without_nul")
	}
	if len(source) > 768*1024 || len(strings.Split(normalizeLines(source), "\n")) > 1000 {
		return fieldError("source_text", "must_be_at_most_768_kib_and_1000_lines")
	}
	return nil
}

func normalizeLines(source string) string {
	return strings.ReplaceAll(strings.ReplaceAll(source, "\r\n", "\n"), "\r", "\n")
}

func parseLines(source string) (*string, []codeLine, []apperror.Detail) {
	lines := strings.Split(normalizeLines(source), "\n")
	headings := map[string]string{"About": "about", "Commander": "commander", "Deck": "mainboard", "Sideboard": "sideboard", "Maybeboard": "maybeboard"}
	hasHeading := false
	for _, line := range lines {
		if _, ok := headings[strings.TrimSpace(line)]; ok {
			hasHeading = true
		}
	}
	section := "mainboard"
	if hasHeading {
		section = ""
	}
	var name *string
	result := []codeLine{}
	issues := []apperror.Detail{}
	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if next, ok := headings[line]; ok {
			section = next
			continue
		}
		if section == "about" && (line == "Name" || strings.HasPrefix(line, "Name ")) {
			value, err := normalizeName(strings.TrimPrefix(line, "Name"))
			if err == nil {
				name = &value
			} else {
				name = nil
			}
			continue
		}
		match := cardLinePattern.FindStringSubmatch(line)
		if section == "" || section == "about" || match == nil {
			issues = append(issues, apperror.Detail{Line: i + 1, Reason: "unrecognized_line", Input: line})
			continue
		}
		quantity, err := strconv.ParseUint(match[1], 10, 32)
		if err != nil || quantity == 0 {
			issues = append(issues, apperror.Detail{Line: i + 1, Reason: "quantity_out_of_range", Input: line})
			continue
		}
		cardName := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(match[2]), "*F*"))
		cardName = printingSuffixPattern.ReplaceAllString(cardName, "")
		cardName = strings.TrimSpace(cardName)
		if cardName == "" {
			issues = append(issues, apperror.Detail{Line: i + 1, Reason: "card_not_found", Input: line})
			continue
		}
		fallback := collectorSuffixPattern.ReplaceAllString(cardName, "")
		if fallback == cardName {
			fallback = ""
		}
		result = append(result, codeLine{Line: i + 1, Section: section, Name: cardName, FallbackName: fallback, Quantity: quantity})
	}
	return name, result, issues
}

func standardCode(deck model.SavedDeck, data map[string]model.DeckCardData) string {
	blocks := []string{"About\nName " + deck.Name}
	for _, section := range sectionOrder {
		cards := []model.SavedDeckCard{}
		for _, card := range deck.Cards {
			if card.Section == section {
				cards = append(cards, card)
			}
		}
		if len(cards) == 0 && section != "mainboard" {
			continue
		}
		sort.Slice(cards, func(i, j int) bool {
			return codeCardLess(data[cards[i].OracleID], data[cards[j].OracleID])
		})
		headings := map[string]string{"commander": "Commander", "mainboard": "Deck", "sideboard": "Sideboard", "maybeboard": "Maybeboard"}
		lines := []string{headings[section]}
		for _, card := range cards {
			lines = append(lines, strconv.FormatUint(card.Quantity, 10)+" "+card.Name)
		}
		blocks = append(blocks, strings.Join(lines, "\n"))
	}
	return strings.Join(blocks, "\n\n")
}

func codeCardLess(a, b model.DeckCardData) bool {
	order := map[string]int{"artifact": 0, "creature": 1, "enchantment": 2, "instant": 3, "sorcery": 4, "planeswalker": 5, "land": 6, "other": 7}
	ka, kb := order[statistics(a).TypeGroups[0]], order[statistics(b).TypeGroups[0]]
	if ka != kb {
		return ka < kb
	}
	if a.Card.CMC != b.Card.CMC {
		return a.Card.CMC < b.Card.CMC
	}
	if ca, cb := codeColor(a.Card.Colors), codeColor(b.Card.Colors); ca != cb {
		return ca < cb
	}
	if c := strings.Compare(strings.ToLower(a.Card.Name), strings.ToLower(b.Card.Name)); c != 0 {
		return c < 0
	}
	return a.Card.PrintingUUID < b.Card.PrintingUUID
}

func codeColor(colors []string) string {
	if len(colors) == 0 {
		return "6"
	}
	key := strconv.Itoa(len(colors))
	for _, color := range []string{"W", "U", "B", "R", "G"} {
		for _, actual := range colors {
			if color == actual {
				key += strconv.Itoa(strings.Index("WUBRG", color))
			}
		}
	}
	return key
}
