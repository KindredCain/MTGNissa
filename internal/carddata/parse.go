package carddata

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

type sourceSpec struct {
	table         tableName
	file          string
	required      []string
	columnSources map[string]string
	extraFields   []string
	standardJSON  bool
}

var sourceSpecs = []sourceSpec{
	{
		table: tableScryfallCard, file: "scryfall_card.json",
		required: []string{"uuid", "scryfall_id", "name", "set_code", "collector_number", "lang"},
		columnSources: map[string]string{
			"colors_mask":            "colors",
			"color_identity_mask":    "color_identity",
			"color_indicator_mask":   "color_indicator",
			"finishes_mask":          "finishes",
			"games_mask":             "games",
			"attraction_lights_mask": "attraction_lights",
		},
		extraFields: []string{"keywords", "frame_effects", "promo_types", "set_code", "set_name", "set_type"},
	},
	{table: tableCardTranslation, file: "zhs_card.json", required: []string{"card_id"}},
	{table: tableFlavorTranslation, file: "zhs_flavor.json", required: []string{"flavor_id"}},
	{table: tableOracleTranslation, file: "zhs_oracle.json", required: []string{"face_oracle_id"}},
	{
		table: tableRulingTranslation, file: "zhs_ruling.json", required: []string{"ruling", "comment"},
		columnSources: map[string]string{
			"ruling_key":         "comment",
			"ruling_id":          "ruling",
			"translation_source": "source",
			"translation_stage":  "stage",
		},
	},
	{
		table: tableScryfallOracleRuling, file: "rulings.jsonl",
		required:      []string{"object", "oracle_id", "source", "published_at", "comment"},
		columnSources: map[string]string{"ruling_key": "comment"},
		extraFields:   []string{"object"}, standardJSON: true,
	},
	{
		table: tableOracleTag, file: "oracle-tags.jsonl",
		required:      []string{"object", "id", "label", "type", "parent_ids", "child_ids", "aliases", "taggings"},
		columnSources: map[string]string{"tag_id": "id"},
		extraFields:   []string{"object", "slug", "type", "uri", "parent_ids", "child_ids", "taggings"}, standardJSON: true,
	},
	{table: tableSetTranslation, file: "zhs_set.json", required: []string{"set_id"}},
	{table: tableTypeTranslation, file: "zhs_type.json", required: []string{"type_name", "type_type"}},
}

func parseObject(line []byte, s sourceSpec) (map[string]json.RawMessage, bool, error) {
	var obj map[string]json.RawMessage
	if len(strings.TrimSpace(string(line))) == 0 {
		return nil, false, errors.New("empty line")
	}
	normalized, changed := line, false
	if !s.standardJSON {
		normalized, changed = normalizeJSONEscapes(line)
	}
	if err := json.Unmarshal(normalized, &obj); err != nil {
		return nil, changed, fmt.Errorf("invalid JSON object after escape normalization: %w", err)
	}
	if obj == nil {
		return nil, changed, errors.New("JSON value must be an object")
	}
	for _, field := range s.required {
		value, ok := obj[field]
		if !ok || string(value) == "null" {
			return nil, changed, fmt.Errorf("required field %q is missing or null", field)
		}
	}
	return obj, changed, nil
}

// normalizeJSONEscapes removes the single upstream layer that duplicated every
// backslash. This includes backslashes representing real card text, since valid
// JSON already escapes each such backslash once.
func normalizeJSONEscapes(line []byte) ([]byte, bool) {
	var out []byte
	changed := false
	for i := 0; i < len(line); {
		if line[i] != '\\' {
			if out != nil {
				out = append(out, line[i])
			}
			i++
			continue
		}
		start := i
		for i < len(line) && line[i] == '\\' {
			i++
		}
		count := i - start
		if count > 1 {
			if out == nil {
				out = make([]byte, 0, len(line))
				out = append(out, line[:start]...)
			}
			normalizedCount := (count + 1) / 2
			for range normalizedCount {
				out = append(out, '\\')
			}
			changed = true
			continue
		}
		if out != nil {
			out = append(out, line[start:i]...)
		}
	}
	if !changed {
		return line, false
	}
	return out, true
}

func scannerFor(r io.Reader) *bufio.Scanner {
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, 64*1024), 16*1024*1024)
	return s
}
