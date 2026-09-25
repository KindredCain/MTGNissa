package carddata

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const Confirmation = "REBUILD_CARD_DATABASE"

type Stage string

const (
	StageValidating Stage = "validating"
	StagePreparing  Stage = "preparing"
	StageImporting  Stage = "importing"
	StageVerifying  Stage = "verifying"
	StageSwitching  Stage = "switching"
	StageCompleted  Stage = "completed"
	StageFailed     Stage = "failed"
)

type FileResult struct {
	File           string `json:"file"`
	Size           int64  `json:"size"`
	SHA256         string `json:"sha256"`
	ReadRows       int64  `json:"read_rows"`
	NormalizedRows int64  `json:"normalized_rows"`
	InsertedRows   int64  `json:"inserted_rows"`
}

type Task struct {
	ID         string       `json:"id"`
	Stage      Stage        `json:"stage"`
	StartedAt  time.Time    `json:"started_at"`
	FinishedAt *time.Time   `json:"finished_at,omitempty"`
	Files      []FileResult `json:"files,omitempty"`
	Error      *TaskError   `json:"error,omitempty"`
}

type TaskError struct {
	Stage   Stage  `json:"stage"`
	File    string `json:"file,omitempty"`
	Line    int64  `json:"line,omitempty"`
	Message string `json:"message"`
}

func (e *TaskError) Error() string { return e.Message }

type importRow struct {
	values []any
	line   int64
}

var ErrRunning = errors.New("a rebuild task is already running")

type Manager struct {
	ctx               context.Context
	db                *sql.DB
	databaseName, dir string
	log               *slog.Logger
	mu                sync.RWMutex
	tasks             map[string]*Task
	running           bool
	progressFile      string
	progressRows      int64
	progressTotal     int64
}

func NewManager(ctx context.Context, db *sql.DB, databaseName, dir string, log *slog.Logger) *Manager {
	return &Manager{ctx: ctx, db: db, databaseName: databaseName, dir: dir, log: log, tasks: make(map[string]*Task)}
}

func (m *Manager) Start() (Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.running {
		return Task{}, ErrRunning
	}
	t := &Task{ID: randomID(), Stage: StageValidating, StartedAt: time.Now().UTC()}
	m.tasks[t.ID] = t
	m.running = true
	go m.run(t.ID)
	return cloneTask(t), nil
}

func (m *Manager) Get(id string) (Task, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	t, ok := m.tasks[id]
	if !ok {
		return Task{}, false
	}
	return cloneTask(t), true
}

func (m *Manager) run(id string) {
	startedAt := time.Now()
	heartbeatDone := make(chan struct{})
	m.setProgress("", 0, 0)
	m.log.Info("card data rebuild started", "task_id", id, "data_dir", m.dir)
	go m.logHeartbeat(id, startedAt, heartbeatDone)
	defer close(heartbeatDone)

	files, taskErr := m.validateAll(id)
	if taskErr == nil {
		taskErr = m.rebuild(id, files)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	t := m.tasks[id]
	now := time.Now().UTC()
	t.FinishedAt = &now
	m.running = false
	if taskErr != nil {
		t.Stage = StageFailed
		t.Error = taskErr
		m.log.Error("card data rebuild failed", "task_id", id, "stage", taskErr.Stage, "file", taskErr.File, "line", taskErr.Line, "duration", time.Since(startedAt), "error", taskErr.Message)
	} else {
		t.Stage = StageCompleted
		m.log.Info("card data rebuild completed", "task_id", id, "duration", time.Since(startedAt))
	}
}

func (m *Manager) logHeartbeat(id string, startedAt time.Time, done <-chan struct{}) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			m.mu.RLock()
			task, ok := m.tasks[id]
			stage := Stage("")
			if ok {
				stage = task.Stage
			}
			file, rows, total := m.progressFile, m.progressRows, m.progressTotal
			m.mu.RUnlock()
			if ok {
				m.log.Info("card data rebuild still running", "task_id", id, "stage", stage, "file", file, "processed_rows", rows, "total_rows", total, "elapsed", time.Since(startedAt))
			}
		case <-done:
			return
		case <-m.ctx.Done():
			return
		}
	}
}

func (m *Manager) validateAll(id string) ([]FileResult, *TaskError) {
	m.stage(id, StageValidating)
	results := make([]FileResult, 0, len(specs))
	for _, s := range specs {
		startedAt := time.Now()
		m.setProgress(s.file, 0, 0)
		m.log.Info("card data file validation started", "task_id", id, "file", s.file)
		result, err := validateFile(m.ctx, m.dir, s, func(rows int64) {
			m.setProgress(s.file, rows, 0)
		})
		if err != nil {
			err.Stage = StageValidating
			return nil, err
		}
		results = append(results, result)
		m.setProgress(s.file, result.ReadRows, result.ReadRows)
		m.files(id, results)
		m.log.Info("card data file validation completed", "task_id", id, "file", s.file, "rows", result.ReadRows, "normalized_rows", result.NormalizedRows, "size_bytes", result.Size, "duration", time.Since(startedAt))
	}
	return results, nil
}

func validateFile(ctx context.Context, dir string, s spec, progress func(int64)) (FileResult, *TaskError) {
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
		if _, err := rowValues(obj, s); err != nil {
			return FileResult{}, fail(s.file, line, err)
		}
		if _, err := deriveRows(s, obj, line); err != nil {
			return FileResult{}, fail(s.file, line, err)
		}
		if normalized {
			normalizedRows++
		}
		key, err := objectKey(obj, s.key)
		if err != nil {
			return FileResult{}, fail(s.file, line, err)
		}
		if _, exists := seen[key]; exists {
			return FileResult{}, fail(s.file, line, fmt.Errorf("duplicate unique key %q", key))
		}
		seen[key] = struct{}{}
	}
	if err := scanner.Err(); err != nil {
		return FileResult{}, fail(s.file, line+1, err)
	}
	if line == 0 {
		return FileResult{}, fail(s.file, 0, errors.New("file contains no records"))
	}
	return FileResult{File: s.file, Size: info.Size(), SHA256: hex.EncodeToString(h.Sum(nil)), ReadRows: line, NormalizedRows: normalizedRows}, nil
}

func (m *Manager) rebuild(id string, results []FileResult) *TaskError {
	ctx := m.ctx
	var selected string
	if err := m.db.QueryRowContext(ctx, "SELECT DATABASE()").Scan(&selected); err != nil {
		return stageFail(StagePreparing, "", 0, err)
	}
	if selected != m.databaseName {
		return stageFail(StagePreparing, "", 0, fmt.Errorf("connected database %q does not match configured card database %q", selected, m.databaseName))
	}
	suffix := strings.ToLower(id)
	if len(suffix) > 12 {
		suffix = suffix[:12]
	}
	tables := allTableSpecs()
	tempNames := make(map[string]string, len(tables))
	backupNames := make(map[string]string, len(tables))
	cleanupNames := make([]string, 0, len(tables)*2)
	for _, table := range tables {
		tempNames[table.table] = table.table + "__new_" + suffix
		backupNames[table.table] = table.table + "__old_" + suffix
		cleanupNames = append(cleanupNames, tempNames[table.table], backupNames[table.table])
	}
	cleanup := func() {
		for _, name := range cleanupNames {
			_, _ = m.db.ExecContext(context.Background(), "DROP TABLE IF EXISTS "+quote(name))
		}
	}
	defer cleanup()
	m.stage(id, StagePreparing)
	prepareStartedAt := time.Now()
	for _, table := range tables {
		q := createTableSQL(tempNames[table.table], table)
		if _, err := m.db.ExecContext(ctx, q); err != nil {
			return stageFail(StagePreparing, table.file, 0, err)
		}
	}
	seedRows := dictionaryRows()
	for _, table := range dictionarySpecs {
		if err := insertRows(ctx, m.db, tempNames[table.table], table, seedRows[table.table]); err != nil {
			return stageFail(StagePreparing, "", 0, fmt.Errorf("seed dictionary %s: %w", table.table, err))
		}
	}
	m.log.Info("card data temporary tables prepared", "task_id", id, "table_count", len(tables), "duration", time.Since(prepareStartedAt))
	m.stage(id, StageImporting)
	for i, s := range specs {
		startedAt := time.Now()
		m.setProgress(s.file, 0, results[i].ReadRows)
		m.log.Info("card data file import started", "task_id", id, "file", s.file, "expected_rows", results[i].ReadRows, "size_bytes", results[i].Size)
		inserted, taskErr := m.importFile(ctx, id, s, tempNames, results[i])
		if taskErr != nil {
			taskErr.Stage = StageImporting
			return taskErr
		}
		results[i].InsertedRows = inserted
		m.files(id, results)
		m.log.Info("card data file import completed", "task_id", id, "file", s.file, "inserted_rows", inserted, "duration", time.Since(startedAt))
	}
	m.stage(id, StageVerifying)
	for i, s := range specs {
		m.setProgress(s.file, 0, results[i].ReadRows)
		var count int64
		if err := m.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+quote(tempNames[s.table])).Scan(&count); err != nil {
			return stageFail(StageVerifying, s.file, 0, err)
		}
		if count != results[i].ReadRows {
			return stageFail(StageVerifying, s.file, 0, fmt.Errorf("row count mismatch: read %d, inserted %d", results[i].ReadRows, count))
		}
		m.setProgress(s.file, count, results[i].ReadRows)
		m.log.Info("card data table row count verified", "task_id", id, "table", s.table, "rows", count)
	}
	m.stage(id, StageSwitching)
	switchStartedAt := time.Now()
	for _, table := range tables {
		q := "CREATE TABLE IF NOT EXISTS " + quote(table.table) + " LIKE " + quote(tempNames[table.table])
		if _, err := m.db.ExecContext(ctx, q); err != nil {
			return stageFail(StageSwitching, table.file, 0, err)
		}
	}
	parts := make([]string, 0, len(tables)*2)
	for _, table := range tables {
		parts = append(parts, quote(table.table)+" TO "+quote(backupNames[table.table]), quote(tempNames[table.table])+" TO "+quote(table.table))
	}
	if _, err := m.db.ExecContext(ctx, "RENAME TABLE "+strings.Join(parts, ", ")); err != nil {
		return stageFail(StageSwitching, "", 0, err)
	}
	m.log.Info("card data tables switched", "task_id", id, "table_count", len(tables), "duration", time.Since(switchStartedAt))
	for _, obsolete := range []string{
		"scryfall_card_color",
		"scryfall_card_attraction_light",
		"scryfall_card_finish",
		"scryfall_card_game",
		"scryfall_card_preview",
	} {
		if _, err := m.db.ExecContext(ctx, "DROP TABLE IF EXISTS "+quote(obsolete)); err != nil {
			m.log.Warn("obsolete card data table cleanup failed", "table", obsolete, "error", err)
		}
	}
	return nil
}

func (m *Manager) importFile(ctx context.Context, taskID string, s spec, tableNames map[string]string, expected FileResult) (int64, *TaskError) {
	f, err := os.Open(filepath.Join(m.dir, s.file))
	if err != nil {
		return 0, fail(s.file, 0, err)
	}
	defer f.Close()
	h := sha256.New()
	scanner := scannerFor(io.TeeReader(f, h))
	const batchSize = 250
	rows := make([]importRow, 0, batchSize)
	derivedRowsByTable := make(map[string][]importRow)
	seenSets := make(map[string]string)
	tableSpecs := make(map[string]spec, len(derivedSpecs))
	for _, table := range derivedSpecs {
		tableSpecs[table.table] = table
	}
	var line, inserted int64
	nextProgressLog := int64(10000)
	flush := func() *TaskError {
		if len(rows) == 0 {
			return nil
		}
		if err := insertRows(ctx, m.db, tableNames[s.table], s, rows); err != nil {
			return fail(s.file, rows[0].line, fmt.Errorf("insert batch ending at line %d: %w", rows[len(rows)-1].line, err))
		}
		for table, derivedRows := range derivedRowsByTable {
			if len(derivedRows) == 0 {
				continue
			}
			if err := insertRows(ctx, m.db, tableNames[table], tableSpecs[table], derivedRows); err != nil {
				return fail(s.file, derivedRows[0].line, fmt.Errorf("insert derived table %s: %w", table, err))
			}
		}
		inserted += int64(len(rows))
		rows = rows[:0]
		clear(derivedRowsByTable)
		m.setProgress(s.file, inserted, expected.ReadRows)
		if inserted >= nextProgressLog {
			m.log.Info("card data file import progress", "task_id", taskID, "file", s.file, "inserted_rows", inserted, "expected_rows", expected.ReadRows)
			for nextProgressLog <= inserted {
				nextProgressLog += 10000
			}
		}
		return nil
	}
	for scanner.Scan() {
		line++
		if err := ctx.Err(); err != nil {
			return inserted, fail(s.file, line, err)
		}
		obj, _, err := parseObject(scanner.Bytes(), s)
		if err != nil {
			return inserted, fail(s.file, line, err)
		}
		values, err := rowValues(obj, s)
		if err != nil {
			return inserted, fail(s.file, line, err)
		}
		rows = append(rows, importRow{values, line})
		derived, err := deriveRows(s, obj, line)
		if err != nil {
			return inserted, fail(s.file, line, err)
		}
		for table, derivedRows := range derived {
			for _, derivedRow := range derivedRows {
				if table == "scryfall_set" {
					setID := fmt.Sprint(derivedRow.values[0])
					signatureJSON, _ := json.Marshal(derivedRow.values[1:])
					signature := string(signatureJSON)
					if previous, exists := seenSets[setID]; exists {
						if previous != signature {
							return inserted, fail(s.file, line, fmt.Errorf("inconsistent metadata for set %s", setID))
						}
						continue
					}
					seenSets[setID] = signature
				}
				derivedRowsByTable[table] = append(derivedRowsByTable[table], derivedRow)
			}
		}
		if len(rows) == batchSize {
			if e := flush(); e != nil {
				return inserted, e
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return inserted, fail(s.file, line+1, err)
	}
	if e := flush(); e != nil {
		return inserted, e
	}
	actualHash := hex.EncodeToString(h.Sum(nil))
	if actualHash != expected.SHA256 || line != expected.ReadRows {
		return inserted, fail(s.file, line, errors.New("file changed after validation"))
	}
	return inserted, nil
}

func insertRows(ctx context.Context, db *sql.DB, table string, s spec, rows []importRow) error {
	if len(rows) == 0 {
		return nil
	}
	args := make([]any, 0, len(rows)*len(s.columns))
	values := make([]string, len(rows))
	placeholder := "(" + strings.TrimSuffix(strings.Repeat("?,", len(s.columns)), ",") + ")"
	for i, row := range rows {
		values[i] = placeholder
		args = append(args, row.values...)
	}
	columns := make([]string, len(s.columns))
	for i, column := range s.columns {
		columns[i] = column.name
	}
	_, err := db.ExecContext(ctx, "INSERT INTO "+quote(table)+" ("+quotedList(columns)+") VALUES "+strings.Join(values, ","), args...)
	return err
}

func parseObject(line []byte, s spec) (map[string]json.RawMessage, bool, error) {
	var obj map[string]json.RawMessage
	if len(strings.TrimSpace(string(line))) == 0 {
		return nil, false, errors.New("empty line")
	}
	normalized, changed := normalizeJSONEscapes(line)
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

func objectKey(obj map[string]json.RawMessage, fields []string) (string, error) {
	parts := make([]string, len(fields))
	for i, field := range fields {
		raw, ok := obj[field]
		if !ok || string(raw) == "null" {
			return "", fmt.Errorf("unique field %q is missing or null", field)
		}
		parts[i] = string(raw)
	}
	return strings.Join(parts, "\x1f"), nil
}

func scannerFor(r io.Reader) *bufio.Scanner {
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, 64*1024), 16*1024*1024)
	return s
}
func quote(name string) string { return "`" + strings.ReplaceAll(name, "`", "``") + "`" }
func randomID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}
func fail(file string, line int64, err error) *TaskError {
	return &TaskError{File: file, Line: line, Message: err.Error()}
}
func stageFail(stage Stage, file string, line int64, err error) *TaskError {
	e := fail(file, line, err)
	e.Stage = stage
	return e
}
func (m *Manager) stage(id string, s Stage) {
	m.mu.Lock()
	m.tasks[id].Stage = s
	m.progressFile = ""
	m.progressRows = 0
	m.progressTotal = 0
	m.mu.Unlock()
	m.log.Info("card data rebuild stage started", "task_id", id, "stage", s)
}

func (m *Manager) setProgress(file string, rows, total int64) {
	m.mu.Lock()
	m.progressFile = file
	m.progressRows = rows
	m.progressTotal = total
	m.mu.Unlock()
}
func (m *Manager) files(id string, f []FileResult) {
	m.mu.Lock()
	m.tasks[id].Files = append([]FileResult(nil), f...)
	m.mu.Unlock()
}
func cloneTask(t *Task) Task {
	c := *t
	c.Files = append([]FileResult(nil), t.Files...)
	if t.Error != nil {
		e := *t.Error
		c.Error = &e
	}
	return c
}
