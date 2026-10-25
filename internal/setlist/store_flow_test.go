package setlist

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/jmoiron/sqlx"
)

// This driver exercises the complete store flow without a running Card DB.
type setQueryStep struct {
	contains []string
	absent   []string
	args     []any
	columns  []string
	rows     [][]driver.Value
}

type setTestDatabase struct {
	t         *testing.T
	steps     []setQueryStep
	index     int
	committed bool
	options   driver.TxOptions
}

func (d *setTestDatabase) Connect(context.Context) (driver.Conn, error) {
	return d, nil
}

func (d *setTestDatabase) Driver() driver.Driver {
	return d
}

func (d *setTestDatabase) Open(string) (driver.Conn, error) {
	return d, nil
}

func (d *setTestDatabase) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("unexpected prepared statement")
}

func (d *setTestDatabase) Close() error {
	return nil
}

func (d *setTestDatabase) Begin() (driver.Tx, error) {
	return nil, errors.New("expected BeginTx")
}

func (d *setTestDatabase) BeginTx(_ context.Context, options driver.TxOptions) (driver.Tx, error) {
	d.options = options
	return d, nil
}

func (d *setTestDatabase) Commit() error {
	d.committed = true
	return nil
}

func (d *setTestDatabase) Rollback() error {
	return nil
}

func (d *setTestDatabase) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	d.t.Helper()
	if d.index >= len(d.steps) {
		d.t.Fatalf("unexpected query: %s", query)
	}
	step := d.steps[d.index]
	d.index++
	for _, fragment := range step.contains {
		if !strings.Contains(query, fragment) {
			d.t.Errorf("query %d missing %q: %s", d.index, fragment, query)
		}
	}
	for _, fragment := range step.absent {
		if strings.Contains(query, fragment) {
			d.t.Errorf("query %d contains %q: %s", d.index, fragment, query)
		}
	}
	values := make([]any, len(args))
	for i, arg := range args {
		values[i] = arg.Value
	}
	if step.args != nil && !reflect.DeepEqual(values, step.args) {
		d.t.Errorf("query %d args = %#v, want %#v", d.index, values, step.args)
	}
	return &setTestRows{columns: step.columns, rows: step.rows}, nil
}

type setTestRows struct {
	columns []string
	rows    [][]driver.Value
}

func (r *setTestRows) Columns() []string {
	return r.columns
}

func (r *setTestRows) Close() error {
	return nil
}

func (r *setTestRows) Next(values []driver.Value) error {
	if len(r.rows) == 0 {
		return io.EOF
	}
	copy(values, r.rows[0])
	r.rows = r.rows[1:]
	return nil
}

func TestStoreReturnsCompleteSubtreeWithoutFilteringChildren(t *testing.T) {
	columns := []string{"code", "name", "display_name", "set_type", "card_count", "released_at", "languages", "parent_code"}
	backend := &setTestDatabase{t: t, steps: []setQueryStep{
		{
			contains: []string{"FROM scryfall_language", "ORDER BY sort_order ASC, code ASC", "LIMIT 11"},
			columns:  []string{"code", "display_name", "sort_order"},
			rows:     [][]driver.Value{{"en", "English", int64(1)}, {"ja", "Japanese", int64(2)}},
		},
		{
			contains: []string{"SELECT code, parent_code FROM mtgnissa_set"},
			columns:  []string{"code", "parent_code"},
			rows:     [][]driver.Value{{"A", nil}, {"B", nil}, {"C", "A"}, {"GC", "C"}, {"TA", "A"}, {"BC", "B"}},
		},
		{
			contains: []string{"COUNT(*)", "`s`.`parent_code` IS NULL", "FIND_IN_SET(?, `s`.`languages`)", "`s`.`set_type` IN (?)"},
			args:     []any{"ja", "expansion"}, columns: []string{"count"}, rows: [][]driver.Value{{int64(2)}},
		},
		{
			contains: []string{"`st`.`code` = `s`.`code`", "`pt`.`code` = `s`.`parent_code`", "`s`.`parent_code` IS NULL", "LIMIT ?"},
			args:     []any{"ja", "expansion", int64(2)}, columns: columns,
			rows: [][]driver.Value{
				{"A", "Root A", "Root A", "expansion", int64(100), *testSetDate(2021, 1, 1), "ja,en", nil},
				{"B", "Root B", "Root B", "expansion", int64(200), *testSetDate(2020, 1, 1), "ja", nil},
			},
		},
		{
			contains: []string{"`s`.`code` IN (?, ?, ?)"},
			absent:   []string{"`s`.`parent_code` IS NULL", "FIND_IN_SET", "`s`.`set_type` IN", "LIMIT", "scryfall_card", "set_id"},
			args:     []any{"C", "GC", "TA"}, columns: columns,
			rows: [][]driver.Value{
				{"TA", "Tokens", "Root A", "token", int64(10), nil, nil, "A"},
				{"GC", "Grandchild", "Grandchild", "promo", int64(2), nil, nil, "C"},
				{"C", "Child", "Child", "promo", int64(5), nil, "en", "A"},
			},
		},
	}}
	db := sqlx.NewDb(sql.OpenDB(backend), "mysql")
	t.Cleanup(func() {
		db.Close()
	})
	page, more, err := NewStore(db).List(context.Background(), Params{
		Languages: []string{"ja"}, SetTypes: []string{"expansion"}, Sort: SortReleasedDesc,
	}, nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !more || page.Total != 2 || len(page.Items) != 4 {
		t.Fatalf("page = %#v, more = %v", page, more)
	}
	for i, code := range []string{"A", "C", "GC", "TA"} {
		if page.Items[i].Code != code {
			t.Errorf("item %d code = %s, want %s", i, page.Items[i].Code, code)
		}
	}
	if page.Items[2].Depth != 2 || page.Items[0].DefaultLanguage == nil || *page.Items[0].DefaultLanguage != "en" || page.Items[1].Languages[1].Available || page.Items[2].DefaultLanguage != nil {
		t.Fatalf("unexpected hierarchy or language availability: %#v", page.Items)
	}
	if len(page.Items[3].Languages) != 0 || page.Items[3].DefaultLanguage != nil {
		t.Fatalf("token has languages: %#v", page.Items[3])
	}
	if backend.index != len(backend.steps) || !backend.committed || !backend.options.ReadOnly || backend.options.Isolation != driver.IsolationLevel(sql.LevelRepeatableRead) {
		t.Fatalf("incomplete or inconsistent transaction: %#v", backend)
	}
}
