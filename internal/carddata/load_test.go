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

func TestInspectAllPrintingsRecursivelyExcludesPartialPreviewSets(t *testing.T) {
	content := `{"meta":{"version":"test"},"data":{` +
		`"AAA":{"code":"AAA","isPartialPreview":true},` +
		`"AAB":{"code":"AAB","parentCode":"AAA"},` +
		`"BBB":{"code":"BBB"}}}`
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "AllPrintings.json"), []byte(content), 0o600); err != nil {
		t.Fatalf("write AllPrintings fixture: %v", err)
	}
	result, plan, taskErr := inspectAllPrintings(context.Background(), dir, func(int64) {
	})
	if taskErr != nil {
		t.Fatalf("inspectAllPrintings() error = %v", taskErr)
	}
	if result.ReadRows != 3 {
		t.Fatalf("inspectAllPrintings() rows = %d, want 3", result.ReadRows)
	}
	for _, code := range []string{"AAA", "AAB"} {
		if _, excluded := plan.excludedSets[code]; !excluded {
			t.Errorf("set %s was not excluded", code)
		}
	}
	if _, excluded := plan.excludedSets["BBB"]; excluded {
		t.Fatal("complete set BBB was excluded")
	}
}
