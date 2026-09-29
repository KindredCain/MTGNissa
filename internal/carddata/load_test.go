package carddata

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestManagerStartClearsFinishedTaskHistory(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	manager := &Manager{
		ctx:   ctx,
		dir:   t.TempDir(),
		log:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		tasks: map[string]*Task{"finished-task": {ID: "finished-task", Stage: StageCompleted}},
	}

	started, err := manager.Start()
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if _, ok := manager.Get("finished-task"); ok {
		t.Fatal("finished task history was not cleared")
	}
	if got, ok := manager.Get(started.ID); !ok || got.ID != started.ID {
		t.Fatalf("new task is not queryable: task = %#v, ok = %v", got, ok)
	}

	deadline := time.Now().Add(time.Second)
	for {
		got, ok := manager.Get(started.ID)
		if ok && got.Stage == StageFailed {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("new task did not finish before test timeout")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestValidateFilePreservesRepublishedRulings(t *testing.T) {
	var rulingSource sourceSpec
	for _, candidate := range sourceSpecs {
		if candidate.table == tableScryfallOracleRuling {
			rulingSource = candidate
			break
		}
	}
	rulingSpec := mustTableSpec(rulingSource.table)
	if !rulingSpec.autoIncrementKey || len(rulingSpec.key) != 1 || rulingSpec.key[0] != "id" {
		t.Fatalf("rulings spec must use an auto-increment primary key: %#v", rulingSpec)
	}

	content := "" +
		`{"object":"ruling","oracle_id":"0056fc91-4398-471c-b561-7ff99750ac8a","source":"wotc","published_at":"2024-02-02","comment":"Once blocked, it stays blocked."}` + "\n" +
		`{"object":"ruling","oracle_id":"0056fc91-4398-471c-b561-7ff99750ac8a","source":"wotc","published_at":"2026-01-27","comment":"Once blocked, it stays blocked."}` + "\n"
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, rulingSource.file), []byte(content), 0o600); err != nil {
		t.Fatalf("write rulings fixture: %v", err)
	}
	result, taskErr := validateFile(context.Background(), dir, rulingSource, func(int64) {
	})
	if taskErr != nil {
		t.Fatalf("validateFile() error = %v", taskErr)
	}
	if result.ReadRows != 2 {
		t.Fatalf("validateFile() rows = %d, want 2", result.ReadRows)
	}
}
