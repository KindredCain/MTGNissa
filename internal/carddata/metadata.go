package carddata

var colorBits = map[string]int64{
	"W": 1,
	"U": 2,
	"B": 4,
	"R": 8,
	"G": 16,
	"C": 32,
}

var finishBits = map[string]int64{
	"nonfoil": 1,
	"foil":    2,
	"etched":  4,
}

var gameBits = map[string]int64{
	"paper":  1,
	"arena":  2,
	"mtgo":   4,
	"astral": 8,
	"sega":   16,
}

func dictionaryRows() map[string][]importRow {
	return map[string][]importRow{
		"scryfall_color": rows(
			[]any{"W", int64(1), "White", "White mana color", int64(1)},
			[]any{"U", int64(2), "Blue", "Blue mana color", int64(2)},
			[]any{"B", int64(4), "Black", "Black mana color", int64(3)},
			[]any{"R", int64(8), "Red", "Red mana color", int64(4)},
			[]any{"G", int64(16), "Green", "Green mana color", int64(5)},
			[]any{"C", int64(32), "Colorless", "Colorless; represented as a color code only where the API permits it", int64(6)},
		),
		"scryfall_language": rows(
			[]any{"en", "English"}, []any{"es", "Spanish"}, []any{"fr", "French"},
			[]any{"de", "German"}, []any{"it", "Italian"}, []any{"pt", "Portuguese"},
			[]any{"ja", "Japanese"}, []any{"ko", "Korean"}, []any{"ru", "Russian"},
			[]any{"zhs", "Simplified Chinese"}, []any{"zht", "Traditional Chinese"},
			[]any{"he", "Hebrew"}, []any{"la", "Latin"}, []any{"grc", "Ancient Greek"},
			[]any{"ar", "Arabic"}, []any{"sa", "Sanskrit"}, []any{"ph", "Phyrexian"},
		),
		"scryfall_layout": rows(
			[]any{"normal", "Normal", "A standard Magic card with one face", "single_faced"},
			[]any{"split", "Split", "A split-faced card", "single_sided_multi_face"},
			[]any{"flip", "Flip", "A card that inverts vertically with the flip keyword", "single_sided_multi_face"},
			[]any{"transform", "Transform", "A double-sided card that transforms", "double_sided"},
			[]any{"modal_dfc", "Modal DFC", "A double-sided card playable from either side", "double_sided"},
			[]any{"meld", "Meld", "A meld part with a combined result on the back", "single_faced"},
			[]any{"leveler", "Leveler", "A card with level-up bands", "single_faced"},
			[]any{"class", "Class", "A Class enchantment card", "single_faced"},
			[]any{"case", "Case", "A Case enchantment card", "single_faced"},
			[]any{"saga", "Saga", "A Saga card", "single_faced"},
			[]any{"adventure", "Adventure", "A permanent card with an Adventure spell part", "single_sided_multi_face"},
			[]any{"mutate", "Mutate", "A card with mutate", "single_faced"},
			[]any{"prototype", "Prototype", "A card with prototype", "single_faced"},
			[]any{"battle", "Battle", "A battle card", "single_faced"},
			[]any{"planar", "Planar", "A Plane or Phenomenon card", "single_faced"},
			[]any{"scheme", "Scheme", "A scheme card", "single_faced"},
			[]any{"vanguard", "Vanguard", "A Vanguard card", "single_faced"},
			[]any{"token", "Token", "A token card", "single_faced"},
			[]any{"double_faced_token", "Double-faced token", "A token with another token on the back", "double_sided"},
			[]any{"emblem", "Emblem", "An emblem card", "single_faced"},
			[]any{"augment", "Augment", "An augment card", "single_faced"},
			[]any{"host", "Host", "A host card", "single_faced"},
			[]any{"art_series", "Art Series", "A collectible double-faced Art Series card", "double_sided"},
			[]any{"reversible_card", "Reversible card", "A card with two unrelated sides", "reversible"},
		),
		"scryfall_frame": rows(
			[]any{"1993", "1993 frame", "The original Magic card frame"},
			[]any{"1997", "1997 frame", "The updated original frame introduced in 1997"},
			[]any{"2003", "2003 frame", "The modern frame introduced with Eighth Edition"},
			[]any{"2015", "2015 frame", "The modern frame with the holofoil-stamp era treatment"},
			[]any{"future", "Future frame", "The experimental future-shifted frame"},
		),
		"scryfall_frame_effect": rows(
			[]any{"legendary", "Legendary crown"}, []any{"miracle", "Miracle frame"},
			[]any{"nyxtouched", "Nyx-touched frame"}, []any{"draft", "Draft-matters frame"},
			[]any{"devoid", "Devoid frame"}, []any{"tombstone", "Odyssey tombstone mark"},
			[]any{"colorshifted", "Colorshifted frame"}, []any{"inverted", "Inverted FNM-style frame"},
			[]any{"sunmoondfc", "Sun and moon transform marks"}, []any{"compasslanddfc", "Compass and land transform marks"},
			[]any{"originpwdfc", "Origins planeswalker transform marks"}, []any{"mooneldrazidfc", "Moon and Eldrazi transform marks"},
			[]any{"waxingandwaningmoondfc", "Waxing and waning moon transform marks"},
			[]any{"showcase", "Showcase frame"}, []any{"extendedart", "Extended-art frame"},
			[]any{"companion", "Companion frame"}, []any{"etched", "Etched treatment"},
			[]any{"snow", "Snow frame"}, []any{"lesson", "Lesson frame"},
			[]any{"shatteredglass", "Shattered Glass frame"}, []any{"convertdfc", "More Than Meets the Eye transform marks"},
			[]any{"fandfc", "Fan transform marks"}, []any{"upsidedowndfc", "Upside Down transform marks"},
		),
		"scryfall_finish": rows(
			[]any{"nonfoil", int64(1), "Nonfoil"},
			[]any{"foil", int64(2), "Foil"},
			[]any{"etched", int64(4), "Etched foil"},
		),
		"scryfall_game": rows(
			[]any{"paper", int64(1), "Paper"},
			[]any{"arena", int64(2), "Magic: The Gathering Arena"},
			[]any{"mtgo", int64(4), "Magic Online"},
			[]any{"astral", int64(8), "MicroProse Magic: The Gathering"},
			[]any{"sega", int64(16), "Sega Dreamcast"},
		),
	}
}

func rows(values ...[]any) []importRow {
	result := make([]importRow, len(values))
	for i, value := range values {
		result[i] = importRow{values: value}
	}
	return result
}
