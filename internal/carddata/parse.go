package carddata

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type sourceSpec struct {
	kind          sourceKind
	table         tableName
	file          string
	required      []string
	columnSources map[string]string
	extraFields   []string
	standardJSON  bool
}

type sourceKind uint8

const (
	sourceRows sourceKind = iota
	sourceAllPrintings
	sourceLegacyCardBridge
)

var sourceSpecs = []sourceSpec{
	{
		kind: sourceAllPrintings, file: "AllPrintings.json", standardJSON: true,
	},
	{
		kind: sourceLegacyCardBridge, file: "scryfall_card.json",
		required: []string{"uuid", "scryfall_id", "face_index", "name"},
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

type setHeader struct {
	Code             string  `json:"code"`
	ParentCode       *string `json:"parentCode"`
	IsPartialPreview bool    `json:"isPartialPreview"`
}

type mtgjsonSet struct {
	Code             string            `json:"code"`
	Name             string            `json:"name"`
	ReleaseDate      *string           `json:"releaseDate"`
	Type             string            `json:"type"`
	KeyruneCode      *string           `json:"keyruneCode"`
	Languages        []string          `json:"languages"`
	Block            *string           `json:"block"`
	ParentCode       *string           `json:"parentCode"`
	TokenSetCode     *string           `json:"tokenSetCode"`
	IsPartialPreview bool              `json:"isPartialPreview"`
	Cards            []json.RawMessage `json:"cards"`
	Tokens           []json.RawMessage `json:"tokens"`
}

type legacyCard struct {
	UUID         string
	ScryfallID   string
	FaceIndex    int64
	Name         string
	FaceName     string
	FlavorID     string
	FaceOracleID string
	Line         int64
}

type loadPlan struct {
	excludedSets map[string]struct{}
}

type importState struct {
	plan                        *loadPlan
	flavorMappingCount          int64
	oracleMappingCount          int64
	bridgeSkippedAlchemy        int64
	bridgeSkippedUnmatched      int64
	translationSkippedUnmatched int64
	legalityByOracle            map[string]map[string]string
	rulingKeys                  map[string]struct{}
	insertedRows                map[tableName]int64
}

func newImportState(plan *loadPlan) *importState {
	return &importState{
		plan:             plan,
		legalityByOracle: make(map[string]map[string]string),
		rulingKeys:       make(map[string]struct{}),
		insertedRows:     make(map[tableName]int64),
	}
}

func inspectAllPrintings(ctx context.Context, dir string, progress func(int64)) (FileResult, *loadPlan, *TaskError) {
	file := "AllPrintings.json"
	path := filepath.Join(dir, file)
	f, err := os.Open(path)
	if err != nil {
		return FileResult{}, nil, fail(file, 0, err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return FileResult{}, nil, fail(file, 0, err)
	}
	if !info.Mode().IsRegular() || info.Size() == 0 {
		return FileResult{}, nil, fail(file, 0, errors.New("file must be a non-empty regular file"))
	}

	h := sha256.New()
	headers := make(map[string]setHeader)
	rows, err := walkAllPrintings(ctx, io.TeeReader(f, h), func(key string, raw json.RawMessage, row int64) error {
		var header setHeader
		if err := json.Unmarshal(raw, &header); err != nil {
			return fmt.Errorf("set %q: %w", key, err)
		}
		if header.Code == "" {
			return fmt.Errorf("set %q: required field %q is missing or empty", key, "code")
		}
		if !strings.EqualFold(key, header.Code) {
			return fmt.Errorf("set key %q does not match code %q", key, header.Code)
		}
		code := strings.ToUpper(header.Code)
		if _, exists := headers[code]; exists {
			return fmt.Errorf("duplicate set code %q", header.Code)
		}
		headers[code] = header
		if row%25 == 0 {
			progress(row)
		}
		return nil
	})
	if err != nil {
		return FileResult{}, nil, fail(file, rows+1, err)
	}
	if rows == 0 {
		return FileResult{}, nil, fail(file, 0, errors.New("AllPrintings data contains no sets"))
	}

	excluded := make(map[string]struct{})
	for code, header := range headers {
		if header.IsPartialPreview {
			excluded[code] = struct{}{}
		}
	}
	changed := true
	for changed {
		changed = false
		for code, header := range headers {
			if _, alreadyExcluded := excluded[code]; alreadyExcluded || header.ParentCode == nil {
				continue
			}
			if _, parentExcluded := excluded[strings.ToUpper(*header.ParentCode)]; parentExcluded {
				excluded[code] = struct{}{}
				changed = true
			}
		}
	}
	return FileResult{File: file, Size: info.Size(), SHA256: hex.EncodeToString(h.Sum(nil)), ReadRows: rows}, &loadPlan{excludedSets: excluded}, nil
}

func walkAllPrintings(ctx context.Context, r io.Reader, visit func(string, json.RawMessage, int64) error) (int64, error) {
	decoder := json.NewDecoder(r)
	token, err := decoder.Token()
	if err != nil {
		return 0, err
	}
	if delimiter, ok := token.(json.Delim); !ok || delimiter != '{' {
		return 0, errors.New("top-level JSON value must be an object")
	}
	var rows int64
	foundData := false
	for decoder.More() {
		nameToken, err := decoder.Token()
		if err != nil {
			return rows, err
		}
		name, ok := nameToken.(string)
		if !ok {
			return rows, errors.New("top-level property name must be a string")
		}
		if name != "data" {
			var ignored json.RawMessage
			if err := decoder.Decode(&ignored); err != nil {
				return rows, fmt.Errorf("top-level field %q: %w", name, err)
			}
			continue
		}
		if foundData {
			return rows, errors.New("top-level field \"data\" appears more than once")
		}
		foundData = true
		dataToken, err := decoder.Token()
		if err != nil {
			return rows, err
		}
		if delimiter, ok := dataToken.(json.Delim); !ok || delimiter != '{' {
			return rows, errors.New("top-level field \"data\" must be an object")
		}
		for decoder.More() {
			if err := ctx.Err(); err != nil {
				return rows, err
			}
			keyToken, err := decoder.Token()
			if err != nil {
				return rows, err
			}
			key, ok := keyToken.(string)
			if !ok {
				return rows, errors.New("set key must be a string")
			}
			var raw json.RawMessage
			if err := decoder.Decode(&raw); err != nil {
				return rows, fmt.Errorf("set %q: %w", key, err)
			}
			rows++
			if err := visit(key, raw, rows); err != nil {
				return rows, err
			}
		}
		if _, err := decoder.Token(); err != nil {
			return rows, err
		}
	}
	if _, err := decoder.Token(); err != nil {
		return rows, err
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err == nil {
		return rows, errors.New("multiple top-level JSON values are not allowed")
	} else if !errors.Is(err, io.EOF) {
		return rows, fmt.Errorf("trailing JSON data: %w", err)
	}
	if !foundData {
		return rows, errors.New("required top-level field \"data\" is missing")
	}
	return rows, nil
}

func languageCode(name string) (string, error) {
	code, ok := mtgjsonLanguageCodes[name]
	if !ok {
		return "", fmt.Errorf("unsupported MTGJSON language %q", name)
	}
	return code, nil
}

func faceIndex(obj map[string]json.RawMessage) (int64, error) {
	layout, err := optionalStringField(obj, "layout")
	if err != nil {
		return 0, err
	}
	if layout != nil && strings.EqualFold(*layout, "meld") {
		return -1, nil
	}
	side, err := optionalStringField(obj, "side")
	if err != nil {
		return 0, err
	}
	if side == nil || *side == "" {
		return -1, nil
	}
	if len(*side) == 1 && (*side)[0] >= 'a' && (*side)[0] <= 'e' {
		return int64((*side)[0] - 'a'), nil
	}
	return 0, fmt.Errorf("unsupported side %q", *side)
}

func lookupKey(scryfallID string, index int64) string {
	return strings.ToLower(scryfallID) + "\x1f" + strconv.FormatInt(index, 10)
}

func tokenSetCode(set mtgjsonSet) string {
	if set.TokenSetCode != nil && *set.TokenSetCode != "" && !strings.EqualFold(*set.TokenSetCode, set.Code) {
		return *set.TokenSetCode
	}
	return "T" + set.Code
}
