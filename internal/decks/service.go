package decks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"mtgnissa/internal/apperror"
	"mtgnissa/internal/idempotency"
	"mtgnissa/internal/model"
	"mtgnissa/internal/store"
)

// Service coordinates saved deck operations, display selection, and editor queries.
type Service struct {
	cards    *store.CardStore
	app      *store.AppTransactions
	intents  *idempotency.Cache
	pageSize int
}

var sectionOrder = []string{"commander", "mainboard", "sideboard", "maybeboard"}

// NewService creates the deck API with shared Card DB access and App DB transactions.
func NewService(cards *store.CardStore, app *store.AppTransactions, intents *idempotency.Cache, pageSize int) *Service {
	return &Service{cards: cards, app: app, intents: intents, pageSize: pageSize}
}

// Detail reads a saved deck and derives its authoritative display and personal statistics.
func (s *Service) Detail(ctx context.Context, id uint64) (model.DeckDetail, error) {
	var deck model.SavedDeck
	var personal model.DeckPersonal
	err := s.app.WithAppRead(ctx, func(tx *store.TxStores) error {
		var err error
		deck, err = tx.Decks.Get(ctx, id, false)
		if err != nil {
			return err
		}
		personal, err = tx.Decks.Personal(ctx, deckOracles([]model.SavedDeck{deck}))
		return err
	})
	if err != nil {
		return model.DeckDetail{}, safeError(err)
	}
	data, err := s.display(ctx, deckOracles([]model.SavedDeck{deck}), personal.Preferred, true)
	if err != nil {
		return model.DeckDetail{}, safeError(err)
	}
	return buildDetail(deck, data, personal), nil
}

// List returns a cursor page using saved deck state only.
func (s *Service) List(ctx context.Context, params model.DeckListParams) (model.DeckPage, error) {
	params.Query = strings.ToLower(strings.TrimSpace(params.Query))
	if params.Sort == "" {
		params.Sort = "updated_desc"
	}
	fingerprint := cursorFingerprint("decks", 0, params.Query, params.Sort)
	var position *cursorPosition
	if params.Cursor != "" {
		decoded, err := decodeCursor(params.Cursor, fingerprint, "decks")
		if err != nil {
			return model.DeckPage{}, err
		}
		position = &decoded
	}
	page := model.DeckPage{Items: []model.DeckListItem{}}
	var selected []model.SavedDeck
	var personal model.DeckPersonal
	err := s.app.WithAppRead(ctx, func(tx *store.TxStores) error {
		all, err := tx.Decks.List(ctx)
		if err != nil {
			return err
		}
		filtered := make([]model.SavedDeck, 0, len(all))
		for _, deck := range all {
			if strings.Contains(strings.ToLower(deck.Name), params.Query) {
				filtered = append(filtered, deck)
			}
		}
		sort.Slice(filtered, func(i, j int) bool {
			return compareDeckPositions(deckPosition(filtered[i], params.Sort), deckPosition(filtered[j], params.Sort), params.Sort) < 0
		})
		page.Total = int64(len(filtered))
		start := 0
		if position != nil {
			start = sort.Search(len(filtered), func(i int) bool {
				return compareDeckPositions(deckPosition(filtered[i], params.Sort), *position, params.Sort) > 0
			})
		}
		end := min(start+s.pageSize, len(filtered))
		selected = filtered[start:end]
		if end < len(filtered) {
			cursor, err := encodeCursor("decks", fingerprint, deckPosition(filtered[end-1], params.Sort))
			if err != nil {
				return err
			}
			page.NextCursor = &cursor
		}
		personal, err = tx.Decks.Personal(ctx, deckOracles(selected))
		return err
	})
	if err != nil {
		return model.DeckPage{}, safeError(err)
	}
	data, err := s.display(ctx, deckOracles(selected), personal.Preferred, false)
	if err != nil {
		return model.DeckPage{}, safeError(err)
	}
	for _, deck := range selected {
		page.Items = append(page.Items, buildListItem(deck, data, personal))
	}
	return page, nil
}

// Create commits one empty definition and creation log, then rereads its current identity.
func (s *Service) Create(ctx context.Context, name, key string) (model.DeckIdentity, bool, error) {
	name, err := normalizeName(name)
	if err != nil {
		return model.DeckIdentity{}, false, err
	}
	body, _ := json.Marshal(struct {
		Name string `json:"name"`
	}{name})
	id, duplicate, err := s.intents.DoResource(ctx, key, "POST /api/v1/decks|"+string(body), func() (uint64, error) {
		var id uint64
		err := s.app.WithAppTx(ctx, func(tx *store.TxStores) error {
			var err error
			id, err = tx.Decks.Create(ctx, model.SavedDeck{Name: name, SourceText: "About\nName " + name + "\n\nDeck"})
			if err != nil {
				return err
			}
			return tx.OperationLogs.AppendDeck(ctx, "create", id, name)
		})
		return id, err
	})
	if err != nil {
		return model.DeckIdentity{}, false, safeError(err)
	}
	identity, err := s.identity(ctx, id)
	return identity, duplicate, safeError(err)
}

// Copy commits a renamed snapshot of the source and retains the new ID for retries.
func (s *Service) Copy(ctx context.Context, id uint64, key string) (model.DeckIdentity, bool, error) {
	newID, duplicate, err := s.intents.DoResource(ctx, key, fmt.Sprintf("POST /api/v1/decks/%d/copies", id), func() (uint64, error) {
		var created uint64
		err := s.app.WithAppTx(ctx, func(tx *store.TxStores) error {
			source, err := tx.Decks.Get(ctx, id, true)
			if err != nil {
				return err
			}
			name, err := normalizeName(copyName(source.Name))
			if err != nil {
				return err
			}
			source.SourceText = strings.Replace(source.SourceText, "Name "+source.Name, "Name "+name, 1)
			source.Name = name
			if err := validateCode(source.SourceText); err != nil {
				return err
			}
			created, err = tx.Decks.Create(ctx, source)
			if err != nil {
				return err
			}
			return tx.OperationLogs.AppendDeck(ctx, "copy", created, name)
		})
		return created, err
	})
	if err != nil {
		return model.DeckIdentity{}, false, safeError(err)
	}
	identity, err := s.identity(ctx, newID)
	return identity, duplicate, safeError(err)
}

// Delete serializes with saves, deletes the definition, and logs its previous name.
func (s *Service) Delete(ctx context.Context, id uint64) error {
	err := s.app.WithAppTx(ctx, func(tx *store.TxStores) error {
		deck, err := tx.Decks.Get(ctx, id, true)
		if err != nil {
			return err
		}
		if err := tx.Decks.Delete(ctx, id); err != nil {
			return err
		}
		return tx.OperationLogs.AppendDeck(ctx, "delete", id, deck.Name)
	})
	return safeError(err)
}

// Save resolves all submitted printings before atomically replacing a locked definition.
func (s *Service) Save(ctx context.Context, id uint64, input model.DeckSave) (model.DeckDetail, error) {
	name, err := normalizeName(input.Name)
	if err != nil {
		return model.DeckDetail{}, err
	}
	ids := []string{}
	for _, section := range input.Sections {
		for _, card := range section.Cards {
			ids = append(ids, card.PrintingUUID)
		}
	}
	if input.CoverPrintingUUID != nil {
		ids = append(ids, *input.CoverPrintingUUID)
	}
	refs, err := s.cards.DeckReferences(ctx, uniqueStrings(ids))
	if err != nil {
		return model.DeckDetail{}, safeError(err)
	}
	next, err := normalizeSave(id, name, input, refs)
	if err != nil {
		return model.DeckDetail{}, err
	}
	personal, _, err := s.personal(ctx, id, deckOracles([]model.SavedDeck{next}))
	if err != nil {
		return model.DeckDetail{}, safeError(err)
	}
	data, err := s.display(ctx, deckOracles([]model.SavedDeck{next}), personal.Preferred, false)
	if err != nil {
		return model.DeckDetail{}, safeError(err)
	}
	next.SourceText = standardCode(next, data)
	if err := validateCode(next.SourceText); err != nil {
		return model.DeckDetail{}, err
	}
	err = s.app.WithAppTx(ctx, func(tx *store.TxStores) error {
		current, err := tx.Decks.Get(ctx, id, true)
		if err != nil {
			return err
		}
		if sameDefinition(current, next) {
			return nil
		}
		if err := tx.Decks.Replace(ctx, next); err != nil {
			return err
		}
		return tx.OperationLogs.AppendDeck(ctx, "save", id, next.Name)
	})
	if err != nil {
		return model.DeckDetail{}, safeError(err)
	}
	return s.Detail(ctx, id)
}

// CardContexts returns request-ordered display contexts with current-deck usage excluded.
func (s *Service) CardContexts(ctx context.Context, id uint64, ids []string) ([]model.DeckEditCardContext, error) {
	if _, err := s.identity(ctx, id); err != nil {
		return nil, safeError(err)
	}
	refs, err := s.cards.DeckReferences(ctx, ids)
	if err != nil {
		return nil, safeError(err)
	}
	oracles := []string{}
	for i, requested := range ids {
		oracle, err := resolveReference(refs, requested, fmt.Sprintf("printing_uuids[%d]", i))
		if err != nil {
			return nil, err
		}
		oracles = append(oracles, oracle)
	}
	personal, current, err := s.personal(ctx, id, uniqueStrings(oracles))
	if err != nil {
		return nil, safeError(err)
	}
	data, err := s.display(ctx, uniqueStrings(oracles), personal.Preferred, true)
	if err != nil {
		return nil, safeError(err)
	}
	result := make([]model.DeckEditCardContext, 0, len(ids))
	for i, oracle := range oracles {
		item := editContext(data[oracle], personal, current, oracle)
		item.RequestedPrintingUUID = ids[i]
		result = append(result, item)
	}
	return result, nil
}

// SearchCards returns logical candidates selected from printed-name contains matches.
func (s *Service) SearchCards(ctx context.Context, id uint64, params model.DeckSearchParams) (model.DeckCandidatePage, error) {
	if _, err := s.identity(ctx, id); err != nil {
		return model.DeckCandidatePage{}, safeError(err)
	}
	params.Query = strings.TrimSpace(params.Query)
	fingerprint := cursorFingerprint("deck_cards", id, strings.ToLower(params.Query), "")
	var position *cursorPosition
	if params.Cursor != "" {
		decoded, err := decodeCursor(params.Cursor, fingerprint, "deck_cards")
		if err != nil {
			return model.DeckCandidatePage{}, err
		}
		position = &decoded
	}
	names, err := s.cards.DeckSearch(ctx, params.Query)
	if err != nil {
		return model.DeckCandidatePage{}, safeError(err)
	}
	sort.Slice(names, func(i, j int) bool {
		return compareNames(names[i], names[j]) < 0
	})
	page := model.DeckCandidatePage{Items: []model.DeckCandidate{}, Total: int64(len(names))}
	start := 0
	if position != nil {
		start = sort.Search(len(names), func(i int) bool {
			return compareNames(names[i], model.DeckName{Name: position.Name, OracleID: &position.OracleID}) > 0
		})
	}
	end := min(start+s.pageSize, len(names))
	if end < len(names) {
		last := names[end-1]
		cursor, err := encodeCursor("deck_cards", fingerprint, cursorPosition{Name: strings.ToLower(last.Name), OracleID: *last.OracleID})
		if err != nil {
			return model.DeckCandidatePage{}, safeError(err)
		}
		page.NextCursor = &cursor
	}
	oracles := []string{}
	for _, name := range names[start:end] {
		oracles = append(oracles, *name.OracleID)
	}
	personal, _, err := s.personal(ctx, id, oracles)
	if err != nil {
		return model.DeckCandidatePage{}, safeError(err)
	}
	data, err := s.display(ctx, oracles, personal.Preferred, false)
	if err != nil {
		return model.DeckCandidatePage{}, safeError(err)
	}
	for _, oracle := range oracles {
		card := data[oracle]
		typeLine, text := displayText(card)
		page.Items = append(page.Items, model.DeckCandidate{Card: card.Card, DisplayTypeLine: typeLine, DisplayOracleText: text})
	}
	return page, nil
}

// NameSuggestions applies the English/Han threshold before issuing any name lookup.
func (s *Service) NameSuggestions(ctx context.Context, id uint64, query string) ([]string, error) {
	if _, err := s.identity(ctx, id); err != nil {
		return nil, safeError(err)
	}
	query = strings.TrimSpace(query)
	if !suggestionEligible(query) {
		return []string{}, nil
	}
	names, err := s.cards.DeckNameSuggestions(ctx, query)
	return names, safeError(err)
}

func (s *Service) identity(ctx context.Context, id uint64) (model.DeckIdentity, error) {
	var result model.DeckIdentity
	err := s.app.WithAppRead(ctx, func(tx *store.TxStores) error {
		deck, err := tx.Decks.Get(ctx, id, false)
		result = model.DeckIdentity{ID: deck.ID, Name: deck.Name}
		return err
	})
	return result, err
}

func (s *Service) personal(ctx context.Context, id uint64, oracles []string) (model.DeckPersonal, map[string]uint64, error) {
	var result model.DeckPersonal
	current := map[string]uint64{}
	err := s.app.WithAppRead(ctx, func(tx *store.TxStores) error {
		deck, err := tx.Decks.Get(ctx, id, false)
		if err != nil {
			return err
		}
		current = countedCards(deck)
		result, err = tx.Decks.Personal(ctx, oracles)
		return err
	})
	return result, current, err
}

func (s *Service) display(ctx context.Context, oracles []string, preferred map[string]int, tags bool) (map[string]model.DeckCardData, error) {
	data, err := s.cards.DeckCards(ctx, oracles, preferred)
	if err != nil || !tags || len(oracles) == 0 {
		return data, err
	}
	labels, err := s.cards.DeckTags(ctx, oracles)
	if err != nil {
		return nil, err
	}
	for oracle, card := range data {
		card.OracleTags = append([]string{}, labels[oracle]...)
		data[oracle] = card
	}
	return data, nil
}

func normalizeSave(id uint64, name string, input model.DeckSave, refs map[string]model.DeckCardData) (model.SavedDeck, error) {
	result := model.SavedDeck{ID: id, Name: name, Cards: []model.SavedDeckCard{}}
	merged := map[[2]string]model.SavedDeckCard{}
	sections := map[string]bool{}
	for i, section := range input.Sections {
		valid := false
		for _, kind := range sectionOrder {
			valid = valid || section.Type == kind
		}
		if !valid || sections[section.Type] {
			return result, fieldError(fmt.Sprintf("sections[%d].type", i), "must_be_a_unique_deck_section")
		}
		sections[section.Type] = true
		for j, card := range section.Cards {
			card.PrintingUUID = strings.ToLower(card.PrintingUUID)
			field := fmt.Sprintf("sections[%d].cards[%d].printing_uuid", i, j)
			oracle, err := resolveReference(refs, card.PrintingUUID, field)
			if err != nil {
				return result, err
			}
			if section.Type == "commander" && !statistics(refs[card.PrintingUUID]).CanBeCommander {
				return result, fieldError(field, "commander_card_not_eligible")
			}
			key := [2]string{section.Type, oracle}
			old := merged[key]
			if card.Quantity == 0 || card.Quantity > math.MaxUint32 || old.Quantity > math.MaxUint32-card.Quantity {
				return result, fieldError(fmt.Sprintf("sections[%d].cards[%d].quantity", i, j), "quantity_out_of_range")
			}
			merged[key] = model.SavedDeckCard{Section: section.Type, OracleID: oracle, Quantity: old.Quantity + card.Quantity, Name: refs[card.PrintingUUID].Card.Name}
		}
	}
	if input.CoverPrintingUUID != nil {
		oracle, err := resolveReference(refs, *input.CoverPrintingUUID, "cover_printing_uuid")
		if err != nil {
			return result, err
		}
		found := false
		for key := range merged {
			found = found || key[1] == oracle
		}
		if !found {
			return result, fieldError("cover_printing_uuid", "cover_card_not_in_deck")
		}
		result.CoverOracleID = &oracle
	}
	for _, card := range merged {
		result.Cards = append(result.Cards, card)
	}
	sort.Slice(result.Cards, func(i, j int) bool {
		a, b := result.Cards[i], result.Cards[j]
		if a.Section != b.Section {
			return a.Section < b.Section
		}
		return a.OracleID < b.OracleID
	})
	return result, nil
}

func sameDefinition(a, b model.SavedDeck) bool {
	if a.Name != b.Name || (a.CoverOracleID == nil) != (b.CoverOracleID == nil) || a.CoverOracleID != nil && *a.CoverOracleID != *b.CoverOracleID || len(a.Cards) != len(b.Cards) {
		return false
	}
	quantities := map[[2]string]uint64{}
	for _, card := range a.Cards {
		quantities[[2]string{card.Section, card.OracleID}] = card.Quantity
	}
	for _, card := range b.Cards {
		if quantities[[2]string{card.Section, card.OracleID}] != card.Quantity {
			return false
		}
	}
	return true
}

func resolveReference(refs map[string]model.DeckCardData, id, field string) (string, error) {
	card, ok := refs[strings.ToLower(id)]
	if !ok {
		return "", apperror.WithDetails(apperror.CodeCardReferenceInvalid, "card printing is unavailable", []apperror.Detail{{Field: field, Reason: "card_reference_invalid"}})
	}
	if card.Card.OracleID == nil || *card.Card.OracleID == "" {
		return "", apperror.WithDetails(apperror.CodeDeckCardNotSupported, "card has no Oracle identity", []apperror.Detail{{Field: field, Reason: "deck_card_not_supported"}})
	}
	return *card.Card.OracleID, nil
}

func normalizeName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if strings.ContainsAny(name, "\r\n\v\f\u0085\u2028\u2029") {
		return "", fieldError("name", "must_not_contain_line_breaks")
	}
	if name == "" || !utf8.ValidString(name) || utf8.RuneCountInString(name) > 255 || strings.ContainsRune(name, 0) {
		return "", fieldError("name", "must_be_1_to_255_characters_without_nul")
	}
	return name, nil
}

func copyName(name string) string {
	prefix := "复制 "
	runes := []rune(name)
	return prefix + string(runes[:min(len(runes), 255-utf8.RuneCountInString(prefix))])
}

func suggestionEligible(query string) bool {
	english, han := 0, 0
	for _, r := range query {
		if r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' {
			english++
		}
		if unicode.Is(unicode.Han, r) {
			han++
		}
	}
	return english >= 3 || han >= 2
}

func deckOracles(decks []model.SavedDeck) []string {
	ids := []string{}
	for _, deck := range decks {
		for _, card := range deck.Cards {
			ids = append(ids, card.OracleID)
		}
		if deck.CoverOracleID != nil {
			ids = append(ids, *deck.CoverOracleID)
		}
	}
	return uniqueStrings(ids)
}

func uniqueStrings(values []string) []string {
	set := make(map[string]bool)
	for _, value := range values {
		set[value] = true
	}
	result := make([]string, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func compareNames(a, b model.DeckName) int {
	if c := strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)); c != 0 {
		return c
	}
	return strings.Compare(*a.OracleID, *b.OracleID)
}

func fieldError(field, reason string) error {
	return apperror.WithDetails(apperror.CodeValidationFailed, "deck input is invalid", []apperror.Detail{{Field: field, Reason: reason}})
}

func safeError(err error) error {
	if err == nil {
		return nil
	}
	var appErr *apperror.Error
	if errors.As(err, &appErr) {
		return err
	}
	return apperror.Wrap(apperror.CodeDependencyUnavailable, "deck or card data is temporarily unavailable", err)
}
