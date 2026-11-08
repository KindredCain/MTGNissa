package decks

import (
	"strings"

	"mtgnissa/internal/model"
)

var typePriority = []string{"land", "creature", "planeswalker", "instant", "sorcery", "enchantment", "artifact", "other"}

func statistics(data model.DeckCardData) model.DeckCardStatistics {
	result := model.DeckCardStatistics{CMC: data.Card.CMC, ManaCost: data.Card.ManaCost, Colors: append([]string{}, data.Card.Colors...), ProducedMana: append([]string{}, data.ProducedMana...), OracleTags: append([]string{}, data.OracleTags...), TypeGroups: []string{}}
	types := map[string]bool{}
	for _, face := range data.Faces {
		if face.TypeLine == nil {
			continue
		}
		main := strings.SplitN(*face.TypeLine, "—", 2)[0]
		for _, token := range strings.Fields(main) {
			types[strings.ToLower(token)] = true
		}
		words := map[string]bool{}
		for _, token := range strings.Fields(*face.TypeLine) {
			words[token] = true
		}
		if words["Legendary"] && (words["Creature"] || words["Planeswalker"] || words["Vehicle"]) {
			result.CanBeCommander = true
		}
	}
	for _, kind := range typePriority[:len(typePriority)-1] {
		if types[kind] {
			result.TypeGroups = append(result.TypeGroups, kind)
		}
	}
	if len(result.TypeGroups) == 0 {
		result.TypeGroups = []string{"other"}
	}
	result.IsLand = types["land"]
	return result
}

func countedCards(deck model.SavedDeck) map[string]uint64 {
	result := map[string]uint64{}
	for _, card := range deck.Cards {
		if card.Section == "commander" || card.Section == "mainboard" || card.Section == "sideboard" {
			result[card.OracleID] += card.Quantity
		}
	}
	return result
}

func cardException(current, owned, all, using uint64) *model.DeckCardException {
	if current > owned {
		return &model.DeckCardException{Type: "shortage", MissingQuantity: current - owned}
	}
	if using > 1 && all > owned {
		return &model.DeckCardException{Type: "deck_usage", MissingQuantity: all - owned}
	}
	return nil
}

func hasColorlessCost(cost *string) bool {
	if cost == nil {
		return false
	}
	for _, part := range strings.Split(*cost, "{")[1:] {
		end := strings.IndexByte(part, '}')
		if end < 0 {
			continue
		}
		for _, symbol := range strings.Split(part[:end], "/") {
			if symbol == "C" {
				return true
			}
		}
	}
	return false
}

func displayText(data model.DeckCardData) (string, *string) {
	if len(data.Faces) == 0 {
		return "", nil
	}
	face := data.Faces[0]
	typeLine := ""
	if face.DisplayTypeLine != nil {
		typeLine = *face.DisplayTypeLine
	} else if face.TypeLine != nil {
		typeLine = *face.TypeLine
	}
	text := face.DisplayOracleText
	if text == nil {
		text = face.OracleText
	}
	return typeLine, text
}

func editContext(data model.DeckCardData, personal model.DeckPersonal, current map[string]uint64, oracle string) model.DeckEditCardContext {
	stats := statistics(data)
	typeLine, text := displayText(data)
	otherQuantity := personal.AllQuantity[oracle]
	otherCount := personal.UsingDecks[oracle]
	if current[oracle] > 0 {
		otherQuantity -= min(otherQuantity, current[oracle])
		if otherCount > 0 {
			otherCount--
		}
	}
	return model.DeckEditCardContext{Card: data.Card, TypeGroup: stats.TypeGroups[0], TranslatedTypeLine: typeLine, TranslatedOracleText: text, OwnedQuantity: personal.Owned[oracle], OtherDecksQuantity: otherQuantity, OtherDecksCount: otherCount, Statistics: stats}
}

func buildDetail(deck model.SavedDeck, data map[string]model.DeckCardData, personal model.DeckPersonal) model.DeckDetail {
	result := model.DeckDetail{ID: deck.ID, Name: deck.Name, SourceText: deck.SourceText, CreatedAt: deck.CreatedAt, UpdatedAt: deck.UpdatedAt, Sections: []model.DeckSection{}}
	current := countedCards(deck)
	collected := uint64(0)
	for oracle, quantity := range current {
		result.Summary.CountedTotal += quantity
		collected += min(quantity, personal.Owned[oracle])
		exception := cardException(quantity, personal.Owned[oracle], personal.AllQuantity[oracle], personal.UsingDecks[oracle])
		if exception != nil {
			target := &result.Summary.Shortage
			if exception.Type == "deck_usage" {
				target = &result.Summary.DeckUsage
			}
			target.DistinctCards++
			target.MissingQuantity += exception.MissingQuantity
		}
	}
	if result.Summary.CountedTotal > 0 {
		result.Summary.CollectionRate = float64(collected) / float64(result.Summary.CountedTotal)
	}
	for _, kind := range sectionOrder {
		section := model.DeckSection{Type: kind, Cards: []model.DeckCard{}}
		for _, saved := range deck.Cards {
			if saved.Section != kind {
				continue
			}
			oracle := saved.OracleID
			card := model.DeckCard{Context: editContext(data[oracle], personal, current, oracle), Quantity: saved.Quantity, AllDecksQuantity: personal.AllQuantity[oracle], UsingDecks: personal.UsingDecks[oracle]}
			if kind != "maybeboard" {
				card.Exception = cardException(current[oracle], personal.Owned[oracle], card.AllDecksQuantity, card.UsingDecks)
			}
			section.Quantity += saved.Quantity
			section.Cards = append(section.Cards, card)
		}
		result.Sections = append(result.Sections, section)
	}
	if deck.CoverOracleID != nil {
		id := data[*deck.CoverOracleID].Card.PrintingUUID
		result.CoverPrintingUUID = &id
	}
	return result
}

func buildListItem(deck model.SavedDeck, data map[string]model.DeckCardData, personal model.DeckPersonal) model.DeckListItem {
	detail := buildDetail(deck, data, personal)
	result := model.DeckListItem{ID: deck.ID, Name: deck.Name, CreatedAt: deck.CreatedAt, UpdatedAt: deck.UpdatedAt, CountedTotal: detail.Summary.CountedTotal,
		SectionCounts: map[string]uint64{}, ColorDistribution: map[string]uint64{"W": 0, "U": 0, "B": 0, "R": 0, "G": 0, "C": 0}}
	if deck.CoverOracleID != nil {
		card := data[*deck.CoverOracleID].Card
		result.CoverCard = &card
	}
	for _, section := range detail.Sections {
		result.SectionCounts[section.Type] = section.Quantity
		if section.Type != "commander" && section.Type != "mainboard" {
			continue
		}
		for _, card := range section.Cards {
			for _, color := range card.Context.Statistics.Colors {
				result.ColorDistribution[color] += card.Quantity
			}
			if hasColorlessCost(card.Context.Statistics.ManaCost) {
				result.ColorDistribution["C"] += card.Quantity
			}
		}
	}
	if detail.Summary.Shortage.DistinctCards > 0 {
		result.Exception = &model.DeckListException{Type: "shortage", DeckExceptionTotals: detail.Summary.Shortage}
	} else if detail.Summary.DeckUsage.DistinctCards > 0 {
		result.Exception = &model.DeckListException{Type: "deck_usage", DeckExceptionTotals: detail.Summary.DeckUsage}
	}
	return result
}
