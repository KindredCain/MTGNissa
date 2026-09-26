package carddata

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestValidateFilePreservesRepublishedRulings(t *testing.T) {
	var rulingSpec spec
	for _, candidate := range specs {
		if candidate.table == "scryfall_oracle_ruling" {
			rulingSpec = candidate
			break
		}
	}
	if !rulingSpec.autoIncrementKey || len(rulingSpec.key) != 1 || rulingSpec.key[0] != "id" {
		t.Fatalf("rulings spec must use an auto-increment primary key: %#v", rulingSpec)
	}

	content := "" +
		`{"object":"ruling","oracle_id":"0056fc91-4398-471c-b561-7ff99750ac8a","source":"wotc","published_at":"2024-02-02","comment":"Once blocked, it stays blocked."}` + "\n" +
		`{"object":"ruling","oracle_id":"0056fc91-4398-471c-b561-7ff99750ac8a","source":"wotc","published_at":"2026-01-27","comment":"Once blocked, it stays blocked."}` + "\n"
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, rulingSpec.file), []byte(content), 0o600); err != nil {
		t.Fatalf("write rulings fixture: %v", err)
	}
	result, taskErr := validateFile(context.Background(), dir, rulingSpec, func(int64) {})
	if taskErr != nil {
		t.Fatalf("validateFile() error = %v", taskErr)
	}
	if result.ReadRows != 2 {
		t.Fatalf("validateFile() rows = %d, want 2", result.ReadRows)
	}
}
