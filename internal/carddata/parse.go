package carddata

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

func parseObject(line []byte, s spec) (map[string]json.RawMessage, bool, error) {
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
