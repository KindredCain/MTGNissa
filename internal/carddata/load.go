package carddata

import (
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

const Confirmation = "LOAD_CARD_DATA"

const cleanupTimeout = 30 * time.Second

// Stage identifies the current phase of a card-data loading task.
type Stage string

const (
	StageValidating Stage = "validating"
	StagePreparing  Stage = "preparing"
	StageImporting  Stage = "importing"
	StageVerifying  Stage = "verifying"
	StageSwitching  Stage = "switching"
	StageCleaning   Stage = "cleaning"
	StageCompleted  Stage = "completed"
	StageFailed     Stage = "failed"
)

// FileResult contains validation and import metrics for one source data file.
type FileResult struct {
	File           string
	Size           int64
	SHA256         string
	ReadRows       int64
	NormalizedRows int64
	InsertedRows   int64
}

// Task is a snapshot of a card-data loading task and its terminal error, if any.
type Task struct {
	ID         string
	Stage      Stage
	StartedAt  time.Time
	FinishedAt *time.Time
	Files      []FileResult
	Error      *TaskError
}

// TaskError describes the stage and source location at which a loading task failed.
type TaskError struct {
	Stage   Stage
	File    string
	Line    int64
	Message string
}

func (e *TaskError) Error() string {
	return e.Message
}

type importRow struct {
	values []any
	line   int64
}

var ErrRunning = errors.New("a card data load task is already running")

// Manager coordinates card-data loading tasks and exposes their current status.
type Manager struct {
	ctx               context.Context
	db                *sql.DB
	databaseName, dir string
	log               *slog.Logger
	mu                sync.RWMutex
	tasks             map[string]*Task
	taskID            string
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
	// A finished task remains queryable until the next task starts. Once a new
	// task is accepted, older task results are no longer useful and must not
	// accumulate for the lifetime of the process.
	clear(m.tasks)
	t := &Task{ID: randomID(), Stage: StageValidating, StartedAt: time.Now().UTC()}
	m.tasks[t.ID] = t
	m.taskID = t.ID
	m.running = true
	go m.run(t.ID)
	return cloneTask(t), nil
}

// LogCurrentTaskStatus records the last known progress before shutdown closes
// the database. It does nothing when no load task is running.
func (m *Manager) LogCurrentTaskStatus() {
	id, stage, file, rows, total, running := m.progressSnapshot()
	if !running {
		return
	}
	m.log.Info("card data loading interrupted by shutdown", "task_id", id, "stage", stage, "file", file, "processed_rows", rows, "total_rows", total)
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
	m.log.Info("card data loading started", "task_id", id, "data_dir", m.dir)
	go m.logHeartbeat(id, startedAt, heartbeatDone)
	defer close(heartbeatDone)

	files, taskErr := m.validateAll(id)
	if taskErr == nil {
		taskErr = m.load(id, files)
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
		m.log.Error("card data loading failed", "task_id", id, "stage", taskErr.Stage, "file", taskErr.File, "line", taskErr.Line, "duration", time.Since(startedAt), "error", taskErr.Message)
	} else {
		t.Stage = StageCompleted
		m.log.Info("card data loading completed", "task_id", id, "duration", time.Since(startedAt))
	}
}

func (m *Manager) progressSnapshot() (id string, stage Stage, file string, rows, total int64, running bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	id = m.taskID
	if task := m.tasks[id]; task != nil {
		stage = task.Stage
	}
	return id, stage, m.progressFile, m.progressRows, m.progressTotal, m.running
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
				m.log.Info("card data loading still running", "task_id", id, "stage", stage, "file", file, "processed_rows", rows, "total_rows", total, "elapsed", time.Since(startedAt))
			}
		case <-done:
			return
		case <-m.ctx.Done():
			return
		}
	}
}

func (m *Manager) load(id string, results []FileResult) *TaskError {
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
	tempNames := make(map[tableName]string, len(tables))
	backupNames := make(map[tableName]string, len(tables))
	for _, table := range tables {
		tempNames[table.table] = string(table.table) + "__new_" + suffix
		backupNames[table.table] = string(table.table) + "__old_" + suffix
	}
	m.stage(id, StagePreparing)
	prepareStartedAt := time.Now()
	for _, table := range tables {
		tableStartedAt := time.Now()
		q := createTableSQL(tempNames[table.table], table)
		if _, err := m.db.ExecContext(ctx, q); err != nil {
			return stageFail(StagePreparing, "", 0, err)
		}
		m.log.Info("card data temporary table creation completed", "task_id", id, "table", table.table, "duration", time.Since(tableStartedAt))
	}
	seedRows := dictionaryRows()
	for _, table := range dictionarySpecs {
		tableStartedAt := time.Now()
		if err := insertRows(ctx, m.db, tempNames[table.table], table, seedRows[table.table]); err != nil {
			return stageFail(StagePreparing, "", 0, fmt.Errorf("seed dictionary %s: %w", table.table, err))
		}
		m.log.Info("card data dictionary table seeding completed", "task_id", id, "table", table.table, "rows", len(seedRows[table.table]), "duration", time.Since(tableStartedAt))
	}
	m.stageCompleted(id, StagePreparing, "table_count", len(tables), "duration", time.Since(prepareStartedAt))
	m.stage(id, StageImporting)
	importStartedAt := time.Now()
	for i, s := range sourceSpecs {
		fileStartedAt := time.Now()
		m.setProgress(s.file, 0, results[i].ReadRows)
		m.log.Info("card data file import started", "task_id", id, "file", s.file, "expected_rows", results[i].ReadRows, "size_bytes", results[i].Size)
		inserted, taskErr := m.importFile(ctx, s, tempNames, results[i])
		if taskErr != nil {
			taskErr.Stage = StageImporting
			return taskErr
		}
		results[i].InsertedRows = inserted
		m.files(id, results)
		m.log.Info("card data file import completed", "task_id", id, "file", s.file, "inserted_rows", inserted, "duration", time.Since(fileStartedAt))
	}
	m.stageCompleted(id, StageImporting, "file_count", len(sourceSpecs), "duration", time.Since(importStartedAt))
	if taskErr := m.verifyImportedData(ctx, id, results, tempNames); taskErr != nil {
		return taskErr
	}
	m.stage(id, StageSwitching)
	switchStartedAt := time.Now()
	for _, table := range tables {
		q := "CREATE TABLE IF NOT EXISTS " + quote(string(table.table)) + " LIKE " + quote(tempNames[table.table])
		if _, err := m.db.ExecContext(ctx, q); err != nil {
			return stageFail(StageSwitching, "", 0, err)
		}
	}
	parts := make([]string, 0, len(tables)*2)
	for _, table := range tables {
		parts = append(parts, quote(string(table.table))+" TO "+quote(backupNames[table.table]), quote(tempNames[table.table])+" TO "+quote(string(table.table)))
	}
	if _, err := m.db.ExecContext(ctx, "RENAME TABLE "+strings.Join(parts, ", ")); err != nil {
		return stageFail(StageSwitching, "", 0, err)
	}
	m.stageCompleted(id, StageSwitching, "table_count", len(tables), "duration", time.Since(switchStartedAt))
	m.stage(id, StageCleaning)
	cleanupNames := make([]string, 0, len(backupNames))
	for _, table := range tables {
		cleanupNames = append(cleanupNames, backupNames[table.table])
	}
	if err := m.cleanupTables(id, cleanupNames); err != nil {
		return stageFail(StageCleaning, "", 0, fmt.Errorf("card data tables switched but cleanup failed: %w", err))
	}
	return nil
}

func (m *Manager) cleanupTables(id string, names []string) error {
	ctx, cancel := context.WithTimeout(m.ctx, cleanupTimeout)
	defer cancel()
	startedAt := time.Now()
	for _, name := range names {
		tableStartedAt := time.Now()
		if _, err := m.db.ExecContext(ctx, "DROP TABLE IF EXISTS "+quote(name)); err != nil {
			return fmt.Errorf("drop table %s: %w", name, err)
		}
		m.log.Info("card data table cleanup completed", "task_id", id, "table", name, "duration", time.Since(tableStartedAt))
	}
	m.stageCompleted(id, StageCleaning, "table_count", len(names), "duration", time.Since(startedAt))
	return nil
}

func (m *Manager) importFile(ctx context.Context, s sourceSpec, tableNames map[tableName]string, expected FileResult) (int64, *TaskError) {
	f, err := os.Open(filepath.Join(m.dir, s.file))
	if err != nil {
		return 0, fail(s.file, 0, err)
	}
	defer f.Close()
	h := sha256.New()
	scanner := scannerFor(io.TeeReader(f, h))
	const batchSize = 250
	rows := make([]importRow, 0, batchSize)
	derivedRowsByTable := make(map[tableName][]importRow)
	seenSets := make(map[string]string)
	seenTagRelations := make(map[string]struct{})
	tableSpecs := make(map[tableName]spec, len(allTableSpecs()))
	for _, table := range allTableSpecs() {
		tableSpecs[table.table] = table
	}
	var line, inserted int64
	flush := func() *TaskError {
		if len(rows) == 0 {
			return nil
		}
		if err := insertRows(ctx, m.db, tableNames[s.table], mustTableSpec(s.table), rows); err != nil {
			return fail(s.file, rows[0].line, fmt.Errorf("insert batch ending at line %d: %w", rows[len(rows)-1].line, err))
		}
		for table, derivedRows := range derivedRowsByTable {
			if len(derivedRows) == 0 {
				continue
			}
			var err error
			if s.table == tableScryfallOracleRuling && table == tableRulingTranslation {
				err = upsertEnglishRulings(ctx, m.db, tableNames[table], tableSpecs[table], derivedRows)
			} else {
				err = insertRows(ctx, m.db, tableNames[table], tableSpecs[table], derivedRows)
			}
			if err != nil {
				return fail(s.file, derivedRows[0].line, fmt.Errorf("insert derived table %s: %w", table, err))
			}
		}
		inserted += int64(len(rows))
		rows = rows[:0]
		clear(derivedRowsByTable)
		m.setProgress(s.file, inserted, expected.ReadRows)
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
				if table == tableOracleTagRelation {
					relationKey := fmt.Sprint(derivedRow.values[0]) + "\x1f" + fmt.Sprint(derivedRow.values[1])
					if _, exists := seenTagRelations[relationKey]; exists {
						continue
					}
					seenTagRelations[relationKey] = struct{}{}
				}
				if table == tableScryfallSet {
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
	columnIndexes := make([]int, 0, len(s.columns))
	columns := make([]string, 0, len(s.columns))
	for i, column := range s.columns {
		if column.kind == kindAutoIncrement {
			continue
		}
		columnIndexes = append(columnIndexes, i)
		columns = append(columns, column.name)
	}
	args := make([]any, 0, len(rows)*len(columnIndexes))
	values := make([]string, len(rows))
	placeholder := "(" + strings.TrimSuffix(strings.Repeat("?,", len(columnIndexes)), ",") + ")"
	for i, row := range rows {
		values[i] = placeholder
		for _, columnIndex := range columnIndexes {
			args = append(args, row.values[columnIndex])
		}
	}
	_, err := db.ExecContext(ctx, "INSERT INTO "+quote(table)+" ("+quotedList(columns)+") VALUES "+strings.Join(values, ","), args...)
	return err
}

func upsertEnglishRulings(ctx context.Context, db *sql.DB, table string, s spec, rows []importRow) error {
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
	query := "INSERT INTO " + quote(table) + " (" + quotedList(columns) + ") VALUES " + strings.Join(values, ",") +
		" ON DUPLICATE KEY UPDATE `last_published_at` = CASE" +
		" WHEN `last_published_at` IS NULL THEN VALUES(`last_published_at`)" +
		" WHEN VALUES(`last_published_at`) IS NULL THEN `last_published_at`" +
		" ELSE GREATEST(`last_published_at`, VALUES(`last_published_at`)) END"
	_, err := db.ExecContext(ctx, query, args...)
	return err
}

func quote(name string) string {
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}
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
	m.log.Info("card data loading stage started", "task_id", id, "stage", s)
}

func (m *Manager) stageCompleted(id string, s Stage, attributes ...any) {
	fields := []any{"task_id", id, "stage", s}
	fields = append(fields, attributes...)
	m.log.Info("card data loading stage completed", fields...)
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
