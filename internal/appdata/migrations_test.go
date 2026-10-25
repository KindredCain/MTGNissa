package appdata

import (
	"fmt"
	"io/fs"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestEmbeddedMigrationsAreValid(t *testing.T) {
	paths, err := fs.Glob(migrationFiles, "app/*.sql")
	if err != nil {
		t.Fatalf("list embedded migrations: %v", err)
	}
	if len(paths) != 8 {
		t.Fatalf("migration count = %d, want 8", len(paths))
	}
	for i, path := range paths {
		wantPrefix := fmt.Sprintf("app/%05d_", i+1)
		if !strings.HasPrefix(path, wantPrefix) {
			t.Errorf("migration path %q does not have prefix %q", path, wantPrefix)
		}
		data, err := migrationFiles.ReadFile(path)
		if err != nil {
			t.Fatalf("read embedded migration %q: %v", path, err)
		}
		contents := string(data)
		if strings.Count(contents, "-- +goose Up") != 1 || strings.Count(contents, "-- +goose Down") != 1 {
			t.Errorf("migration %q must contain exactly one Up and one Down annotation", path)
		}
	}
}

func TestPersistedModelsMatchMigrationColumns(t *testing.T) {
	models := map[string]reflect.Type{
		"card_print_ref":        reflect.TypeOf(CardPrintRef{}),
		"collection_item":       reflect.TypeOf(CollectionItem{}),
		"wishlist_item":         reflect.TypeOf(WishlistItem{}),
		"deck":                  reflect.TypeOf(Deck{}),
		"deck_card":             reflect.TypeOf(DeckCard{}),
		"operation_log":         reflect.TypeOf(OperationLog{}),
		"operation_card_detail": reflect.TypeOf(OperationCardDetail{}),
		"operation_deck_detail": reflect.TypeOf(OperationDeckDetail{}),
	}
	paths, err := fs.Glob(migrationFiles, "app/*.sql")
	if err != nil {
		t.Fatalf("list embedded migrations: %v", err)
	}
	tablePattern := regexp.MustCompile("CREATE TABLE `([^`]+)`")
	columnPattern := regexp.MustCompile("(?m)^\\s*`([^`]+)`\\s+([^\\r\\n]+)")
	for _, path := range paths {
		data, err := migrationFiles.ReadFile(path)
		if err != nil {
			t.Fatalf("read migration %q: %v", path, err)
		}
		match := tablePattern.FindStringSubmatch(string(data))
		if len(match) != 2 {
			t.Fatalf("migration %q has no table declaration", path)
		}
		table := match[1]
		model, ok := models[table]
		if !ok {
			t.Fatalf("table %q has no persistence model", table)
		}
		delete(models, table)
		t.Run(table, func(t *testing.T) {
			columns := make(map[string]string)
			for _, column := range columnPattern.FindAllStringSubmatch(string(data), -1) {
				columns[column[1]] = column[2]
			}
			for i := 0; i < model.NumField(); i++ {
				field := model.Field(i)
				column := field.Tag.Get("db")
				definition, ok := columns[column]
				if !ok {
					t.Errorf("model field %s maps to missing column %q", field.Name, column)
					continue
				}
				delete(columns, column)
				nullable := !strings.Contains(definition, "NOT NULL")
				fieldType := field.Type
				if nullable != (fieldType.Kind() == reflect.Pointer) {
					t.Errorf("column %s nullable=%t does not match model type %s", column, nullable, fieldType)
				}
				if fieldType.Kind() == reflect.Pointer {
					fieldType = fieldType.Elem()
				}
				if !matchesSQLColumnType(fieldType, definition) {
					t.Errorf("column %s definition %q does not match model type %s", column, definition, fieldType)
				}
			}
			for column := range columns {
				t.Errorf("column %q has no model field", column)
			}
		})
	}
	for table := range models {
		t.Errorf("persistence model %q has no embedded table", table)
	}
}

func matchesSQLColumnType(fieldType reflect.Type, definition string) bool {
	sqlType := strings.Fields(definition)[0]
	sqlType, _, _ = strings.Cut(sqlType, "(")
	unsigned := strings.Contains(definition, "UNSIGNED")
	switch sqlType {
	case "CHAR", "VARCHAR", "TEXT", "LONGTEXT":
		return fieldType.Kind() == reflect.String
	case "DATETIME":
		return fieldType == reflect.TypeOf(time.Time{})
	case "BOOLEAN":
		return fieldType.Kind() == reflect.Bool
	case "BIGINT":
		return unsigned && fieldType.Kind() == reflect.Uint64 || !unsigned && fieldType.Kind() == reflect.Int64
	case "INT":
		return unsigned && fieldType.Kind() == reflect.Uint32 || !unsigned && fieldType.Kind() == reflect.Int32
	case "TINYINT":
		return unsigned && fieldType.Kind() == reflect.Uint8 || !unsigned && fieldType.Kind() == reflect.Int8
	default:
		return false
	}
}

func TestPersistedEnumValues(t *testing.T) {
	sections := []DeckSection{
		DeckSectionMainboard,
		DeckSectionSideboard,
		DeckSectionCommander,
		DeckSectionMaybeboard,
	}
	for _, section := range sections {
		if !section.Valid() {
			t.Errorf("section %q should be valid", section)
		}
	}
	if DeckSection("other").Valid() {
		t.Error("unexpected valid custom deck section")
	}

	for action := ActionAddCard; action <= ActionModifyDeck; action++ {
		if !action.Valid() {
			t.Errorf("action %d should be valid", action)
		}
	}
	if ActionType(0).Valid() || ActionType(9).Valid() {
		t.Error("action outside persisted range should be invalid")
	}

	if !FinishNormal.Valid() || !FinishFoil.Valid() || FinishKind(0).Valid() || FinishKind(3).Valid() {
		t.Error("finish enum validation does not match persisted values")
	}
}
