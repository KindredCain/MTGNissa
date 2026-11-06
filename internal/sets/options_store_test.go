package sets

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"reflect"
	"testing"

	"github.com/jmoiron/sqlx"
)

func TestStoreOptionsReadsConsistentFilters(t *testing.T) {
	for _, empty := range []bool{false, true} {
		t.Run(map[bool]string{false: "populated", true: "empty"}[empty], func(t *testing.T) {
			languageRows := [][]driver.Value{{"en", "English", int64(1)}, {"ja", "Japanese", int64(2)}}
			typeRows := [][]driver.Value{{"expansion"}, {"core"}}
			want := Options{
				Languages: []Language{{Code: "en", DisplayName: "English", SortOrder: 1}, {Code: "ja", DisplayName: "Japanese", SortOrder: 2}},
				SetTypes:  []string{"expansion", "core"},
			}
			if empty {
				languageRows, typeRows = nil, nil
				want = Options{Languages: []Language{}, SetTypes: []string{}}
			}
			backend := &setTestDatabase{t: t, steps: []setQueryStep{
				{
					contains: []string{"FROM scryfall_language", "ORDER BY sort_order ASC, code ASC", "LIMIT 11"},
					columns:  []string{"code", "display_name", "sort_order"}, rows: languageRows,
				},
				{
					contains: []string{"FROM mtgnissa_set", "parent_code IS NULL", "set_type <> ''", "set_type <> 'token'", "GROUP BY set_type", "ORDER BY MIN(code) ASC, set_type ASC"},
					columns:  []string{"set_type"}, rows: typeRows,
				},
			}}
			db := sqlx.NewDb(sql.OpenDB(backend), "mysql")
			t.Cleanup(func() {
				db.Close()
			})
			got, err := NewService(NewStore(db), 50).Options(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("options = %#v, want %#v", got, want)
			}
			if backend.index != len(backend.steps) || !backend.committed || !backend.options.ReadOnly || backend.options.Isolation != driver.IsolationLevel(sql.LevelRepeatableRead) {
				t.Fatalf("incomplete or inconsistent transaction: %#v", backend)
			}
		})
	}
}
