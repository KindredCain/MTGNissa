package cards

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"

	"mtgnissa/internal/apperror"
)

type dbStep struct {
	query   string
	absent  []string
	args    []any
	exec    bool
	columns []string
	rows    [][]driver.Value
	err     error
}

type scriptedDB struct {
	t          *testing.T
	steps      []dbStep
	index      int
	committed  bool
	rolledBack bool
	options    driver.TxOptions
}

type scriptedRows struct {
	columns []string
	rows    [][]driver.Value
}

type result struct{}

func (result) LastInsertId() (int64, error) {
	return 1, nil
}

func (result) RowsAffected() (int64, error) {
	return 1, nil
}

func (d *scriptedDB) Connect(context.Context) (driver.Conn, error) {
	return d, nil
}

func (d *scriptedDB) Driver() driver.Driver {
	return d
}

func (d *scriptedDB) Open(string) (driver.Conn, error) {
	return d, nil
}

func (d *scriptedDB) Close() error {
	return nil
}

func (d *scriptedDB) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("unexpected prepare")
}

func (d *scriptedDB) Begin() (driver.Tx, error) {
	return nil, errors.New("expected BeginTx")
}

func (d *scriptedDB) BeginTx(_ context.Context, options driver.TxOptions) (driver.Tx, error) {
	d.options = options
	return d, nil
}

func (d *scriptedDB) Commit() error {
	d.committed = true
	return nil
}

func (d *scriptedDB) Rollback() error {
	d.rolledBack = true
	return nil
}

func (d *scriptedDB) next(query string, exec bool) dbStep {
	d.t.Helper()
	if d.index >= len(d.steps) {
		d.t.Fatalf("unexpected query: %s", query)
	}
	step := d.steps[d.index]
	d.index++
	if !strings.Contains(query, step.query) || exec != step.exec {
		d.t.Fatalf("step %d: exec=%v query=%s; expected exec=%v containing %s", d.index, exec, query, step.exec, step.query)
	}
	for _, fragment := range step.absent {
		if strings.Contains(query, fragment) {
			d.t.Fatalf("step %d contains expensive or unnecessary query fragment %q: %s", d.index, fragment, query)
		}
	}
	return step
}

func (d *scriptedDB) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	step := d.next(query, false)
	if step.args != nil {
		values := make([]any, len(args))
		for i, arg := range args {
			values[i] = arg.Value
		}
		if !reflect.DeepEqual(values, step.args) {
			d.t.Fatalf("step %d args=%#v, expected %#v", d.index, values, step.args)
		}
	}
	return &scriptedRows{columns: step.columns, rows: step.rows}, step.err
}

func (d *scriptedDB) ExecContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Result, error) {
	step := d.next(query, true)
	return result{}, step.err
}

func (r *scriptedRows) Columns() []string {
	return r.columns
}

func (r *scriptedRows) Close() error {
	return nil
}

func (r *scriptedRows) Next(values []driver.Value) error {
	if len(r.rows) == 0 {
		return io.EOF
	}
	copy(values, r.rows[0])
	r.rows = r.rows[1:]
	return nil
}

func testDatabase(t *testing.T, steps []dbStep) (*sqlx.DB, *scriptedDB) {
	t.Helper()
	backend := &scriptedDB{t: t, steps: steps}
	db := sqlx.NewDb(sql.OpenDB(backend), "mysql")
	t.Cleanup(func() {
		db.Close()
		if backend.index != len(steps) {
			t.Errorf("only consumed %d of %d database steps", backend.index, len(steps))
		}
	})
	return db, backend
}

func referenceStep(card Summary) dbStep {
	return dbStep{query: "FOR UPDATE", columns: []string{"printing_uuid", "oracle_id", "set_code", "collector_number", "lang", "created_at", "last_verified_at"},
		rows: [][]driver.Value{{card.PrintingUUID, nil, card.Set.Code, card.CollectorNumber, card.Language.Code, time.Now(), time.Now()}}}
}

func testCard() Summary {
	return Summary{PrintingUUID: "print", Set: Set{Code: "KLD"}, CollectorNumber: "1", Language: Language{Code: "en"}}
}

func TestCollectionAndLogCommitTogetherOrRollback(t *testing.T) {
	for _, logFails := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "log failure"}[logFails], func(t *testing.T) {
			var failure error
			if logFails {
				failure = errors.New("snapshot write failed")
			}
			steps := []dbStep{
				{query: "INSERT INTO card_print_ref", exec: true}, referenceStep(testCard()),
				{query: "FOR UPDATE", columns: []string{"normal_quantity", "foil_quantity"}},
				{query: "INSERT INTO collection_item", exec: true},
				{query: "INSERT INTO operation_log", exec: true},
				{query: "INSERT INTO operation_card_detail", exec: true, err: failure},
			}
			db, backend := testDatabase(t, steps)
			err := NewAppStore(db).mutate(context.Background(), testCard(), "foil", 1, nil)
			if !errors.Is(err, failure) || backend.committed == logFails || backend.rolledBack != logFails {
				t.Fatalf("error=%v committed=%v rollback=%v", err, backend.committed, backend.rolledBack)
			}
			if backend.options.Isolation != driver.IsolationLevel(sql.LevelReadCommitted) {
				t.Fatal("mutation isolation changed")
			}
		})
	}
}

func TestCollectionUnderflowRollsBackAndZeroDeletes(t *testing.T) {
	for _, initial := range []int64{0, 1} {
		steps := []dbStep{{query: "INSERT INTO card_print_ref", exec: true}, referenceStep(testCard()),
			{query: "FOR UPDATE", columns: []string{"normal_quantity", "foil_quantity"}, rows: [][]driver.Value{{initial, int64(0)}}}}
		if initial == 1 {
			steps = append(steps, dbStep{query: "DELETE FROM collection_item", exec: true}, dbStep{query: "INSERT INTO operation_log", exec: true}, dbStep{query: "INSERT INTO operation_card_detail", exec: true})
		}
		db, backend := testDatabase(t, steps)
		err := NewAppStore(db).mutate(context.Background(), testCard(), "normal", -1, nil)
		if initial == 0 {
			var appErr *apperror.Error
			if !errors.As(err, &appErr) || appErr.Code != apperror.CodeCollectionUnderflow || backend.committed {
				t.Fatalf("underflow=%v committed=%v", err, backend.committed)
			}
		} else if err != nil || !backend.committed {
			t.Fatalf("zero quantity deletion failed: %v", err)
		}
	}
}

func TestWishlistLogsOnlyActualChanges(t *testing.T) {
	for _, alreadyIncluded := range []bool{false, true} {
		var count int64
		if alreadyIncluded {
			count = 1
		}
		steps := []dbStep{{query: "INSERT INTO card_print_ref", exec: true}, referenceStep(testCard()),
			{query: "FROM wishlist_item", columns: []string{"count"}, rows: [][]driver.Value{{count}}}}
		if !alreadyIncluded {
			steps = append(steps, dbStep{query: "INSERT INTO wishlist_item", exec: true}, dbStep{query: "INSERT INTO operation_log", exec: true}, dbStep{query: "INSERT INTO operation_card_detail", exec: true})
		}
		db, backend := testDatabase(t, steps)
		included := true
		if err := NewAppStore(db).mutate(context.Background(), testCard(), "", 0, &included); err != nil || !backend.committed {
			t.Fatalf("wishlist error=%v committed=%v", err, backend.committed)
		}
	}
}

func TestStoredReferenceCannotSilentlyChangeIdentity(t *testing.T) {
	step := referenceStep(testCard())
	step.rows[0][2] = "OTHER"
	db, backend := testDatabase(t, []dbStep{{query: "INSERT INTO card_print_ref", exec: true}, step})
	err := NewAppStore(db).mutate(context.Background(), testCard(), "normal", 1, nil)
	if err == nil || !backend.rolledBack || backend.committed {
		t.Fatalf("identity mismatch accepted: %v", err)
	}
}

func TestListUsesOrdinarySetAndKnownLanguage(t *testing.T) {
	set := dbStep{query: "WHERE s.code=?", columns: []string{"code", "name", "display_name", "set_type", "released_at"}, rows: [][]driver.Value{{"KLD", "Kaladesh", "Kaladesh", "expansion", nil}}}
	language := dbStep{query: "FROM scryfall_language", columns: []string{"code", "display_name", "sort_order"}, rows: [][]driver.Value{{"en", "English", int64(1)}}}
	faceColumns := []string{"printing_uuid", "source_id", "image_id", "face_index", "name", "display_name", "collector_number", "language_code"}
	steps := []dbStep{set, language, {query: "WHERE s.code=? AND l.code=?", columns: faceColumns,
		absent: []string{"GROUP BY", "oracle_text", "flavor_text", "translated_type", "card_flavor_translation_mapping"},
		rows:   [][]driver.Value{{"p10", "id10", "", int64(-1), "Ten", "Ten", "10", "en"}, {"p2", "id2", "", int64(-1), "Two", "Two", "2", "en"}}},
		{query: "printed_name LIKE ? ESCAPE '='", columns: []string{"uuid"}, rows: [][]driver.Value{{"p2"}}}}
	db, backend := testDatabase(t, steps)
	list, err := NewCardStore(db).list(context.Background(), "KLD", "en", "Two%_")
	if err != nil || len(list.Items) != 1 || list.Items[0].PrintingUUID != "p2" || !backend.committed || !backend.options.ReadOnly {
		t.Fatalf("list=%#v error=%v", list, err)
	}
	for _, kind := range []string{"missing", "token", "unknown language", "empty"} {
		setStep := set
		setStep.rows = [][]driver.Value{{"KLD", "Kaladesh", "Kaladesh", "expansion", nil}}
		steps := []dbStep{setStep}
		want := apperror.CodeSetNotFound
		switch kind {
		case "missing":
			steps[0].rows = nil
		case "token":
			steps[0].rows[0][3] = "token"
			want = apperror.CodeInvalidRequest
		case "unknown language":
			steps = append(steps, dbStep{query: "FROM scryfall_language", columns: language.columns})
			want = apperror.CodeValidationFailed
		case "empty":
			steps = append(steps, language, dbStep{query: "WHERE s.code=? AND l.code=?", columns: faceColumns})
		}
		db, _ := testDatabase(t, steps)
		list, err := NewCardStore(db).list(context.Background(), "KLD", "en", "")
		if kind == "empty" {
			if err != nil || !reflect.DeepEqual(list.Items, []Summary{}) {
				t.Fatalf("empty list=%#v error=%v", list, err)
			}
		} else {
			var appErr *apperror.Error
			if !errors.As(err, &appErr) || appErr.Code != want {
				t.Fatalf("%s error=%v", kind, err)
			}
		}
	}
}

func TestBatchSummariesKeepMainPrintingAndDistinctBack(t *testing.T) {
	columns := []string{"printing_uuid", "source_id", "image_id", "face_index", "name", "display_name", "language_code", "face_category"}
	id := "00000000-0000-4000-8000-000000000001"
	source := "00000000-0000-4000-8000-000000000010"
	db, backend := testDatabase(t, []dbStep{{query: "requested.uuid IN (?)", columns: columns,
		args: []any{id}, absent: []string{"GROUP BY", "c.oracle_text", "c.flavor_text"},
		rows: [][]driver.Value{
			{"back", source, source, int64(1), "Front // Back", "Back translation", "en", "double_sided"},
			{id, source, source, int64(0), "Front // Back", "Front translation", "en", "double_sided"},
		}}})
	result, err := NewCardStore(db).Summaries(context.Background(), []string{id})
	if err != nil {
		t.Fatal(err)
	}
	card := result[id]
	if len(result) != 1 || card.PrintingUUID != id || card.DisplayName != "Front translation // Back translation" || card.Images.BackURL == nil || !backend.committed || !backend.options.ReadOnly {
		t.Fatalf("summary=%#v backend=%#v", card, backend)
	}
}

func TestBatchSummariesRejectMissingPrintingAndSkipEmptyQuery(t *testing.T) {
	db, _ := testDatabase(t, []dbStep{{query: "requested.uuid IN (?)", columns: []string{"printing_uuid"}}})
	if _, err := NewCardStore(db).Summaries(context.Background(), []string{"missing"}); err == nil {
		t.Fatal("missing canonical printing was silently omitted")
	}
	db, _ = testDatabase(t, nil)
	result, err := NewCardStore(db).Summaries(context.Background(), nil)
	if err != nil || result == nil || len(result) != 0 {
		t.Fatalf("empty summaries=%#v error=%v", result, err)
	}
}
