package carddata

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (m *Manager) validateAll(id string) ([]FileResult, *loadPlan, *TaskError) {
	m.stage(id, StageValidating)
	stageStartedAt := time.Now()
	results := make([]FileResult, 0, len(sourceSpecs))
	plan := &loadPlan{excludedSets: make(map[string]struct{})}
	for _, s := range sourceSpecs {
		fileStartedAt := time.Now()
		m.setProgress(s.file, 0, 0)
		var result FileResult
		var err *TaskError
		if s.kind == sourceAllPrintings {
			var inspectedPlan *loadPlan
			result, inspectedPlan, err = inspectAllPrintings(m.ctx, m.dir, func(rows int64) {
				m.setProgress(s.file, rows, 0)
			})
			if inspectedPlan != nil {
				plan = inspectedPlan
			}
		} else {
			result, err = validateFile(m.ctx, m.dir, s, func(rows int64) {
				m.setProgress(s.file, rows, 0)
			})
		}
		if err != nil {
			err.Stage = StageValidating
			return nil, nil, err
		}
		results = append(results, result)
		m.setProgress(s.file, result.ReadRows, result.ReadRows)
		m.files(id, results)
		m.log.Info("card data file validation completed", "task_id", id, "file", s.file, "rows", result.ReadRows, "normalized_rows", result.NormalizedRows, "size_bytes", result.Size, "duration", time.Since(fileStartedAt))
	}
	m.stageCompleted(id, StageValidating, "file_count", len(results), "duration", time.Since(stageStartedAt))
	return results, plan, nil
}

func validateFile(ctx context.Context, dir string, s sourceSpec, progress func(int64)) (FileResult, *TaskError) {
	path := filepath.Join(dir, s.file)
	f, err := os.Open(path)
	if err != nil {
		return FileResult{}, fail(s.file, 0, err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return FileResult{}, fail(s.file, 0, err)
	}
	if !info.Mode().IsRegular() || info.Size() == 0 {
		return FileResult{}, fail(s.file, 0, errors.New("file must be a non-empty regular file"))
	}
	h := sha256.New()
	scanner := scannerFor(io.TeeReader(f, h))
	seen := make(map[string]struct{})
	var line, normalizedRows int64
	for scanner.Scan() {
		line++
		if line%1000 == 0 {
			progress(line)
		}
		if err := ctx.Err(); err != nil {
			return FileResult{}, fail(s.file, line, err)
		}
		obj, normalized, err := parseObject(scanner.Bytes(), s)
		if err != nil {
			return FileResult{}, fail(s.file, line, err)
		}
		var values []any
		if s.kind == sourceLegacyCardBridge {
			uuid, err := stringField(obj, "uuid")
			if err != nil {
				return FileResult{}, fail(s.file, line, err)
			}
			values = []any{uuid}
		} else {
			values, err = rowValues(obj, s)
			if err != nil {
				return FileResult{}, fail(s.file, line, err)
			}
			if _, err := deriveRows(s, obj, line); err != nil {
				return FileResult{}, fail(s.file, line, err)
			}
		}
		if normalized {
			normalizedRows++
		}
		tableSpec := spec{key: []string{"uuid"}, columns: []columnSpec{col("uuid", "CHAR(36) NOT NULL", kindString)}}
		if s.kind != sourceLegacyCardBridge {
			tableSpec = mustTableSpec(s.table)
		}
		if !tableSpec.autoIncrementKey {
			key, err := relationalKey(values, tableSpec)
			if err != nil {
				return FileResult{}, fail(s.file, line, err)
			}
			if _, exists := seen[key]; exists {
				return FileResult{}, fail(s.file, line, fmt.Errorf("duplicate unique key %q", key))
			}
			seen[key] = struct{}{}
		}
	}
	if err := scanner.Err(); err != nil {
		return FileResult{}, fail(s.file, line+1, err)
	}
	if line == 0 {
		return FileResult{}, fail(s.file, 0, errors.New("file contains no records"))
	}
	return FileResult{File: s.file, Size: info.Size(), SHA256: hex.EncodeToString(h.Sum(nil)), ReadRows: line, NormalizedRows: normalizedRows}, nil
}

func relationalKey(values []any, s spec) (string, error) {
	columnIndexes := make(map[string]int, len(s.columns))
	for i, column := range s.columns {
		columnIndexes[column.name] = i
	}
	parts := make([]any, len(s.key))
	for i, field := range s.key {
		columnIndex, ok := columnIndexes[field]
		if !ok {
			return "", fmt.Errorf("primary key column %q is not defined", field)
		}
		if values[columnIndex] == nil {
			return "", fmt.Errorf("primary key column %q is null", field)
		}
		value := values[columnIndex]
		if text, ok := value.(string); ok {
			text = strings.TrimSpace(text)
			if text == "" {
				return "", fmt.Errorf("primary key column %q is empty", field)
			}
			value = strings.ToLower(text)
		}
		parts[i] = value
	}
	encoded, err := json.Marshal(parts)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}
