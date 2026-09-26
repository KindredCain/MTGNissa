package appdata

import (
	"fmt"
	"io/fs"
	"strings"
	"testing"
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
