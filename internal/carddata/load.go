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

	files, plan, taskErr := m.validateAll(id)
	if taskErr == nil {
		taskErr = m.load(id, files, plan)
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

func (m *Manager) load(id string, results []FileResult, plan *loadPlan) *TaskError {
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
	bridgeName := "card_uuid_bridge__stage_" + suffix
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
	state := newImportState(plan)
	for i, s := range sourceSpecs {
		fileStartedAt := time.Now()
		if s.kind == sourceLegacyCardBridge {
			if err := createCardUUIDBridgeTable(ctx, m.db, bridgeName); err != nil {
				return stageFail(StageImporting, s.file, 0, fmt.Errorf("create card UUID bridge table: %w", err))
			}
			m.log.Info("card UUID bridge staging table created", "task_id", id, "file", s.file, "table", bridgeName)
		}
		m.setProgress(s.file, 0, results[i].ReadRows)
		m.log.Info("card data file import started", "task_id", id, "file", s.file, "expected_rows", results[i].ReadRows, "size_bytes", results[i].Size)
		inserted, taskErr := m.importFile(ctx, s, tempNames, bridgeName, results[i], state)
		if taskErr != nil {
			taskErr.Stage = StageImporting
			return taskErr
		}
		results[i].InsertedRows = inserted
		m.files(id, results)
		m.log.Info("card data file import completed", "task_id", id, "file", s.file, "inserted_rows", inserted, "duration", time.Since(fileStartedAt))
		if s.kind == sourceLegacyCardBridge {
			m.log.Info("legacy card UUID bridge import summarized",
				"task_id", id,
				"file", s.file,
				"read_rows", results[i].ReadRows,
				"mapped_rows", inserted,
				"skipped_alchemy_rows", state.bridgeSkippedAlchemy,
				"skipped_unmatched_rows", state.bridgeSkippedUnmatched,
			)
		}
		if s.table == tableCardTranslation {
			m.log.Info("card translation UUID mapping summarized",
				"task_id", id,
				"file", s.file,
				"read_rows", results[i].ReadRows,
				"inserted_rows", inserted,
				"skipped_unmatched_rows", state.translationSkippedUnmatched,
			)
		}
	}
	if err := insertTranslationMappings(ctx, m.db, tempNames, bridgeName, state); err != nil {
		return stageFail(StageImporting, "", 0, fmt.Errorf("insert translation mappings: %w", err))
	}
	if err := dropCardUUIDBridgeTable(ctx, m.db, bridgeName); err != nil {
		return stageFail(StageImporting, "", 0, fmt.Errorf("drop card UUID bridge table: %w", err))
	}
	m.log.Info("card UUID bridge staging table dropped", "task_id", id, "table", bridgeName)
	m.stageCompleted(id, StageImporting, "file_count", len(sourceSpecs), "duration", time.Since(importStartedAt))
	if taskErr := m.verifyImportedData(ctx, id, results, tempNames, state); taskErr != nil {
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

func (m *Manager) importFile(ctx context.Context, s sourceSpec, tableNames map[tableName]string, bridgeName string, expected FileResult, state *importState) (int64, *TaskError) {
	if s.kind == sourceAllPrintings {
		return m.importAllPrintings(ctx, s, tableNames, expected, state)
	}
	if s.kind == sourceLegacyCardBridge {
		return m.importLegacyBridge(ctx, s, tableNames[tableScryfallCard], bridgeName, expected, state)
	}
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
		if s.table == tableCardTranslation {
			mappedRows, err := remapCardTranslationRows(ctx, m.db, bridgeName, rows, state)
			if err != nil {
				return fail(s.file, rows[0].line, fmt.Errorf("resolve card UUID batch ending at line %d: %w", rows[len(rows)-1].line, err))
			}
			rows = mappedRows
		}
		if err := insertRows(ctx, m.db, tableNames[s.table], mustTableSpec(s.table), rows); err != nil {
			line := int64(0)
			if len(rows) > 0 {
				line = rows[0].line
			}
			return fail(s.file, line, fmt.Errorf("insert batch: %w", err))
		}
		for table, derivedRows := range derivedRowsByTable {
			if len(derivedRows) == 0 {
				continue
			}
			err := insertRows(ctx, m.db, tableNames[table], tableSpecs[table], derivedRows)
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

func createCardUUIDBridgeTable(ctx context.Context, db *sql.DB, table string) error {
	query := "CREATE TABLE " + quote(table) + " (" +
		"`legacy_uuid` CHAR(36) NOT NULL," +
		"`card_uuid` CHAR(36) NOT NULL," +
		"`flavor_id` CHAR(36) NULL," +
		"`face_oracle_id` CHAR(36) NULL," +
		"PRIMARY KEY (`legacy_uuid`)," +
		"KEY `idx_card_uuid_bridge_card` (`card_uuid`)," +
		"KEY `idx_card_uuid_bridge_flavor` (`flavor_id`,`card_uuid`)," +
		"KEY `idx_card_uuid_bridge_oracle` (`face_oracle_id`,`card_uuid`)" +
		") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci"
	_, err := db.ExecContext(ctx, query)
	return err
}

func dropCardUUIDBridgeTable(ctx context.Context, db *sql.DB, table string) error {
	_, err := db.ExecContext(ctx, "DROP TABLE "+quote(table))
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

func (m *Manager) importAllPrintings(ctx context.Context, s sourceSpec, tableNames map[tableName]string, expected FileResult, state *importState) (int64, *TaskError) {
	f, err := os.Open(filepath.Join(m.dir, s.file))
	if err != nil {
		return 0, fail(s.file, 0, err)
	}
	defer f.Close()
	h := sha256.New()
	batches := make(map[tableName][]importRow)
	seenSetCodes := make(map[string]string)
	var inserted int64
	flushTable := func(table tableName) error {
		rows := batches[table]
		if len(rows) == 0 {
			return nil
		}
		if err := insertRows(ctx, m.db, tableNames[table], mustTableSpec(table), rows); err != nil {
			return err
		}
		inserted += int64(len(rows))
		state.insertedRows[table] += int64(len(rows))
		batches[table] = rows[:0]
		return nil
	}
	appendRows := func(table tableName, rows []importRow) error {
		batches[table] = append(batches[table], rows...)
		if len(batches[table]) >= 250 {
			return flushTable(table)
		}
		return nil
	}

	rows, walkErr := walkAllPrintings(ctx, io.TeeReader(f, h), func(key string, raw json.RawMessage, line int64) error {
		if _, excluded := state.plan.excludedSets[strings.ToUpper(key)]; excluded {
			m.setProgress(s.file, line, expected.ReadRows)
			return nil
		}
		var set mtgjsonSet
		if err := json.Unmarshal(raw, &set); err != nil {
			return fmt.Errorf("set %q: %w", key, err)
		}
		if set.Code == "" || set.Name == "" || set.Type == "" {
			return fmt.Errorf("set %q: code, name, and type must be non-empty", key)
		}
		if !strings.EqualFold(key, set.Code) {
			return fmt.Errorf("set key %q does not match code %q", key, set.Code)
		}
		tokenCode := tokenSetCode(set)
		setRows, err := mtgjsonSetRows(set, tokenCode, line)
		if err != nil {
			return fmt.Errorf("set %q: %w", key, err)
		}
		for _, row := range setRows {
			code := strings.ToUpper(fmt.Sprint(row.values[0]))
			kind := fmt.Sprint(row.values[3])
			if previous, exists := seenSetCodes[code]; exists {
				return fmt.Errorf("set code %q is generated more than once (%s and %s)", row.values[0], previous, kind)
			}
			seenSetCodes[code] = kind
		}
		if err := appendRows(tableScryfallSet, setRows); err != nil {
			return fmt.Errorf("insert set %q: %w", key, err)
		}
		for _, cardRaw := range set.Cards {
			derived, err := mtgjsonCardRows(cardRaw, set, line, state)
			if err != nil {
				return fmt.Errorf("set %q card: %w", key, err)
			}
			for table, derivedRows := range derived {
				if err := appendRows(table, derivedRows); err != nil {
					return fmt.Errorf("insert set %q card into %s: %w", key, table, err)
				}
			}
		}
		for _, tokenRaw := range set.Tokens {
			derived, err := mtgjsonTokenRows(tokenRaw, tokenCode, line)
			if err != nil {
				return fmt.Errorf("set %q token: %w", key, err)
			}
			for table, derivedRows := range derived {
				if err := appendRows(table, derivedRows); err != nil {
					return fmt.Errorf("insert set %q token into %s: %w", key, table, err)
				}
			}
		}
		m.setProgress(s.file, line, expected.ReadRows)
		return nil
	})
	if walkErr != nil {
		return inserted, fail(s.file, rows, walkErr)
	}
	if hex.EncodeToString(h.Sum(nil)) != expected.SHA256 || rows != expected.ReadRows {
		return inserted, fail(s.file, rows, errors.New("file changed after validation"))
	}
	legalities := legalityRows(state)
	for start := 0; start < len(legalities); start += 250 {
		end := min(start+250, len(legalities))
		if err := appendRows(tableOracleLegality, legalities[start:end]); err != nil {
			return inserted, fail(s.file, rows, fmt.Errorf("insert oracle legalities: %w", err))
		}
	}
	for table := range batches {
		if err := flushTable(table); err != nil {
			return inserted, fail(s.file, rows, fmt.Errorf("insert final %s batch: %w", table, err))
		}
	}
	return inserted, nil
}

func (m *Manager) importLegacyBridge(ctx context.Context, s sourceSpec, cardTable, bridgeTable string, expected FileResult, state *importState) (int64, *TaskError) {
	f, err := os.Open(filepath.Join(m.dir, s.file))
	if err != nil {
		return 0, fail(s.file, 0, err)
	}
	defer f.Close()
	h := sha256.New()
	scanner := scannerFor(io.TeeReader(f, h))
	const batchSize = 250
	batch := make([]legacyCard, 0, batchSize)
	var line, mapped int64
	flush := func() *TaskError {
		if len(batch) == 0 {
			return nil
		}
		inserted, err := insertLegacyBridgeBatch(ctx, m.db, cardTable, bridgeTable, batch)
		if err != nil {
			return fail(s.file, batch[0].Line, fmt.Errorf("map batch ending at line %d: %w", batch[len(batch)-1].Line, err))
		}
		mapped += inserted
		state.bridgeSkippedUnmatched += int64(len(batch)) - inserted
		batch = batch[:0]
		return nil
	}
	for scanner.Scan() {
		line++
		if err := ctx.Err(); err != nil {
			return mapped, fail(s.file, line, err)
		}
		obj, _, err := parseObject(scanner.Bytes(), s)
		if err != nil {
			return mapped, fail(s.file, line, err)
		}
		card, err := legacyCardFromObject(obj, line)
		if err != nil {
			return mapped, fail(s.file, line, err)
		}
		if strings.HasPrefix(strings.TrimSpace(card.Name), "A-") || strings.HasPrefix(strings.TrimSpace(card.FaceName), "A-") {
			state.bridgeSkippedAlchemy++
			continue
		}
		batch = append(batch, card)
		if len(batch) == batchSize {
			if taskErr := flush(); taskErr != nil {
				return mapped, taskErr
			}
		}
		if line%1000 == 0 {
			m.setProgress(s.file, line, expected.ReadRows)
		}
	}
	if err := scanner.Err(); err != nil {
		return mapped, fail(s.file, line+1, err)
	}
	if taskErr := flush(); taskErr != nil {
		return mapped, taskErr
	}
	if hex.EncodeToString(h.Sum(nil)) != expected.SHA256 || line != expected.ReadRows {
		return mapped, fail(s.file, line, errors.New("file changed after validation"))
	}
	return mapped, nil
}

func insertLegacyBridgeBatch(ctx context.Context, db *sql.DB, cardTable, bridgeTable string, cards []legacyCard) (int64, error) {
	placeholders := make([]string, len(cards))
	args := make([]any, 0, len(cards)*2)
	for i, card := range cards {
		placeholders[i] = "(?,?)"
		args = append(args, card.ScryfallID, card.FaceIndex)
	}
	query := "SELECT `scryfall_id`,`face_index`,`uuid` FROM " + quote(cardTable) +
		" WHERE (`scryfall_id`,`face_index`) IN (" + strings.Join(placeholders, ",") + ")"
	result, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	defer result.Close()
	resolved := make(map[string]string, len(cards))
	for result.Next() {
		var scryfallID, uuid string
		var faceIndex int64
		if err := result.Scan(&scryfallID, &faceIndex, &uuid); err != nil {
			return 0, err
		}
		resolved[lookupKey(scryfallID, faceIndex)] = uuid
	}
	if err := result.Err(); err != nil {
		return 0, err
	}

	valueSQL := make([]string, 0, len(cards))
	insertArgs := make([]any, 0, len(cards)*4)
	for _, card := range cards {
		cardUUID, exists := resolved[lookupKey(card.ScryfallID, card.FaceIndex)]
		if !exists {
			continue
		}
		valueSQL = append(valueSQL, "(?,?,?,?)")
		insertArgs = append(insertArgs, card.UUID, cardUUID, nullableString(card.FlavorID), nullableString(card.FaceOracleID))
	}
	if len(valueSQL) == 0 {
		return 0, nil
	}
	_, err = db.ExecContext(ctx, "INSERT INTO "+quote(bridgeTable)+
		" (`legacy_uuid`,`card_uuid`,`flavor_id`,`face_oracle_id`) VALUES "+strings.Join(valueSQL, ","), insertArgs...)
	if err != nil {
		return 0, err
	}
	return int64(len(valueSQL)), nil
}

func legacyCardFromObject(obj map[string]json.RawMessage, line int64) (legacyCard, error) {
	uuid, err := stringField(obj, "uuid")
	if err != nil {
		return legacyCard{}, err
	}
	scryfallID, err := stringField(obj, "scryfall_id")
	if err != nil {
		return legacyCard{}, err
	}
	var index int64
	if err := json.Unmarshal(obj["face_index"], &index); err != nil {
		return legacyCard{}, fmt.Errorf("field %q: %w", "face_index", err)
	}
	name, err := stringField(obj, "name")
	if err != nil {
		return legacyCard{}, err
	}
	faceName, err := optionalStringField(obj, "face_name")
	if err != nil {
		return legacyCard{}, err
	}
	flavorID, err := optionalStringField(obj, "flavor_id")
	if err != nil {
		return legacyCard{}, err
	}
	faceOracleID, err := optionalStringField(obj, "face_oracle_id")
	if err != nil {
		return legacyCard{}, err
	}
	return legacyCard{
		UUID: uuid, ScryfallID: scryfallID, FaceIndex: index, Name: name,
		FaceName: valueOrEmpty(faceName), FlavorID: valueOrEmpty(flavorID), FaceOracleID: valueOrEmpty(faceOracleID), Line: line,
	}, nil
}

func remapCardTranslationRows(ctx context.Context, db *sql.DB, bridgeTable string, rows []importRow, state *importState) ([]importRow, error) {
	cardIDIndex := -1
	for i, column := range mustTableSpec(tableCardTranslation).columns {
		if column.name == "card_id" {
			cardIDIndex = i
			break
		}
	}
	if cardIDIndex < 0 {
		return nil, errors.New("card_translation card_id column is not defined")
	}
	placeholders := make([]string, len(rows))
	args := make([]any, len(rows))
	for i, row := range rows {
		placeholders[i] = "?"
		args[i] = row.values[cardIDIndex]
	}
	result, err := db.QueryContext(ctx, "SELECT `legacy_uuid`,`card_uuid` FROM "+quote(bridgeTable)+
		" WHERE `legacy_uuid` IN ("+strings.Join(placeholders, ",")+")", args...)
	if err != nil {
		return nil, err
	}
	defer result.Close()
	resolved := make(map[string]string, len(rows))
	for result.Next() {
		var legacyUUID, cardUUID string
		if err := result.Scan(&legacyUUID, &cardUUID); err != nil {
			return nil, err
		}
		resolved[strings.ToLower(legacyUUID)] = cardUUID
	}
	if err := result.Err(); err != nil {
		return nil, err
	}
	mapped := make([]importRow, 0, len(rows))
	for _, row := range rows {
		legacyUUID := fmt.Sprint(row.values[cardIDIndex])
		cardUUID, exists := resolved[strings.ToLower(legacyUUID)]
		if !exists {
			state.translationSkippedUnmatched++
			continue
		}
		row.values[cardIDIndex] = cardUUID
		mapped = append(mapped, row)
	}
	return mapped, nil
}

func insertTranslationMappings(ctx context.Context, db *sql.DB, tableNames map[tableName]string, bridgeTable string, state *importState) error {
	mappings := []struct {
		table  tableName
		column string
		count  *int64
	}{
		{table: tableCardFlavorMapping, column: "flavor_id", count: &state.flavorMappingCount},
		{table: tableCardOracleMapping, column: "face_oracle_id", count: &state.oracleMappingCount},
	}
	for _, mapping := range mappings {
		result, err := db.ExecContext(ctx, "INSERT INTO "+quote(tableNames[mapping.table])+" (`card_uuid`,"+quote(mapping.column)+")"+
			" SELECT DISTINCT `card_uuid`,"+quote(mapping.column)+" FROM "+quote(bridgeTable)+
			" WHERE "+quote(mapping.column)+" IS NOT NULL")
		if err != nil {
			return fmt.Errorf("%s: %w", mapping.table, err)
		}
		rows, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("%s affected rows: %w", mapping.table, err)
		}
		*mapping.count = rows
	}
	return nil
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
