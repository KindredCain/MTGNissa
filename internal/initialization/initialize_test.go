package initialization

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"mtgnissa/internal/appdata"
)

type printingRefStub struct {
	pending  map[string]bool
	fail     map[string]error
	visited  []string
	readErr  error
	missing  map[string]bool
	loadErr  error
	contexts []context.Context
	updated  []string
}

func (s *printingRefStub) CountPending(ctx context.Context) (int64, error) {
	s.contexts = append(s.contexts, ctx)
	var count int64
	for _, pending := range s.pending {
		if pending {
			count++
		}
	}
	return count, nil
}

func (s *printingRefStub) Pending(ctx context.Context, after string, _ int) ([]string, error) {
	s.contexts = append(s.contexts, ctx)
	if s.readErr != nil {
		return nil, s.readErr
	}
	ids := []string{}
	for id, pending := range s.pending {
		if pending && id > after {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids[:min(2, len(ids))], nil
}

func (s *printingRefStub) Load(ctx context.Context, ids []string) (map[string]appdata.CardPrintRef, error) {
	s.contexts = append(s.contexts, ctx)
	refs := make(map[string]appdata.CardPrintRef)
	for _, id := range ids {
		s.visited = append(s.visited, id)
		if !s.missing[id] {
			refs[strings.ToLower(id)] = appdata.CardPrintRef{PrintingUUID: id}
		}
	}
	return refs, s.loadErr
}

func (s *printingRefStub) Update(ctx context.Context, ref appdata.CardPrintRef) error {
	s.contexts = append(s.contexts, ctx)
	s.updated = append(s.updated, ref.PrintingUUID)
	if err := s.fail[ref.PrintingUUID]; err != nil {
		return err
	}
	s.pending[ref.PrintingUUID] = false
	return nil
}

func TestPrintingRefOperationsHaveIndependentThirtySecondDeadlines(t *testing.T) {
	source := &printingRefStub{pending: map[string]bool{"one": true, "two": true}}
	started := time.Now()
	log := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))
	if err := refreshPrintingRefs(context.Background(), source, t.TempDir(), log); err != nil {
		t.Fatal(err)
	}
	seen := map[context.Context]bool{}
	if len(source.contexts) != 6 {
		t.Fatalf("operation contexts=%d", len(source.contexts))
	}
	for _, ctx := range source.contexts {
		deadline, ok := ctx.Deadline()
		if !ok || deadline.Before(started.Add(30*time.Second)) || deadline.After(time.Now().Add(30*time.Second)) || seen[ctx] {
			t.Fatalf("missing independent 30-second deadline: %v", deadline)
		}
		seen[ctx] = true
	}
}

func TestPrintingRefTimeoutLogsAndStopsStartup(t *testing.T) {
	for _, operation := range []string{"load", "update"} {
		t.Run(operation, func(t *testing.T) {
			var output bytes.Buffer
			log := slog.New(slog.NewJSONHandler(&output, nil))
			cause := fmt.Errorf("database operation: %w", context.DeadlineExceeded)
			source := &printingRefStub{pending: map[string]bool{"one": true, "two": true, "three": true}}
			if operation == "load" {
				source.loadErr = cause
			} else {
				source.fail = map[string]error{"one": cause}
			}
			next := false
			dir := t.TempDir()
			err := Run(context.Background(), log, PrintingRefs(source, dir, log), Step{Name: "next", Run: func(context.Context) error {
				next = true
				return nil
			}})
			if !errors.Is(err, context.DeadlineExceeded) || next || !source.pending["two"] || !source.pending["three"] {
				t.Fatalf("next=%v pending=%v error=%v", next, source.pending, err)
			}
			if operation == "load" && len(source.updated) != 0 || operation == "update" && !reflect.DeepEqual(source.updated, []string{"one"}) {
				t.Fatalf("unexpected later updates: %v", source.updated)
			}
			files, err := os.ReadDir(dir)
			if err != nil || len(files) != 1 {
				t.Fatalf("failure files=%v error=%v", files, err)
			}
			content, err := os.ReadFile(filepath.Join(dir, files[0].Name()))
			if err != nil || !strings.Contains(string(content), "printing_uuid=one") || bytes.Count(content, []byte("\n")) != 1 {
				t.Fatalf("failure content=%q error=%v", content, err)
			}
			if !strings.Contains(output.String(), "initialization incomplete") || !strings.Contains(output.String(), `"failed":1`) || strings.Contains(output.String(), "application initialization completed") {
				t.Fatalf("logs=%s", output.String())
			}
		})
	}
}

func TestPrintingRefMissingDataAndBatchReadFailureRemainPending(t *testing.T) {
	for _, batchFailure := range []bool{false, true} {
		t.Run(fmt.Sprint(batchFailure), func(t *testing.T) {
			var output bytes.Buffer
			log := slog.New(slog.NewJSONHandler(&output, nil))
			source := &printingRefStub{
				pending: map[string]bool{"one": true, "two": true, "three": true},
				missing: map[string]bool{"one": true},
			}
			if batchFailure {
				source.loadErr = errors.New("card database unavailable")
			}
			dir := t.TempDir()
			if err := refreshPrintingRefs(context.Background(), source, dir, log); err != nil {
				t.Fatal(err)
			}
			if len(source.visited) != 3 || !source.pending["one"] || source.pending["two"] != batchFailure || source.pending["three"] != batchFailure {
				t.Fatalf("visited=%v pending=%v", source.visited, source.pending)
			}
			files, err := os.ReadDir(dir)
			if err != nil || len(files) != 1 {
				t.Fatalf("files=%v error=%v", files, err)
			}
			content, err := os.ReadFile(filepath.Join(dir, files[0].Name()))
			want := "data not found"
			if batchFailure {
				want = source.loadErr.Error()
			}
			if err != nil || !strings.Contains(string(content), want) {
				t.Fatalf("failure content=%q error=%v", content, err)
			}
		})
	}
}

func TestInitializationStepsRunInOrderAndStopOnFailure(t *testing.T) {
	var output bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&output, nil))
	visited := []string{}
	cause := errors.New("step failed")
	err := Run(context.Background(), log,
		Step{Name: "first", Run: func(context.Context) error {
			visited = append(visited, "first")
			return nil
		}},
		Step{Name: "second", Run: func(context.Context) error {
			visited = append(visited, "second")
			return cause
		}},
		Step{Name: "third", Run: func(context.Context) error {
			visited = append(visited, "third")
			return nil
		}},
	)
	if !errors.Is(err, cause) || !reflect.DeepEqual(visited, []string{"first", "second"}) || !strings.Contains(output.String(), "initialization step failed") || strings.Contains(output.String(), "application initialization completed") {
		t.Fatalf("visited=%v error=%v logs=%s", visited, err, output.String())
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := Run(ctx, log, Step{Name: "cancelled", Run: func(context.Context) error {
		t.Fatal("cancelled initialization executed a step")
		return nil
	}}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation=%v", err)
	}
}

func TestPrintingRefFailuresContinueAndWriteNamedFile(t *testing.T) {
	var output bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&output, nil))
	missing := "00000000-0000-4000-8000-000000000002"
	first := "00000000-0000-4000-8000-000000000001"
	last := "00000000-0000-4000-8000-000000000003"
	source := &printingRefStub{pending: map[string]bool{first: true, missing: true, last: true, "already-real": false}, fail: map[string]error{missing: errors.New("printing not found\nsource unavailable")}}
	dir := filepath.Join(t.TempDir(), "logs")
	nextStep := false
	err := Run(context.Background(), log, PrintingRefs(source, dir, log), Step{Name: "next", Run: func(context.Context) error {
		nextStep = true
		return nil
	}})
	if err != nil || !nextStep || !reflect.DeepEqual(source.visited, []string{first, missing, last}) || !source.pending[missing] || source.pending[first] || source.pending[last] {
		t.Fatalf("visited=%v pending=%v next=%v error=%v", source.visited, source.pending, nextStep, err)
	}
	files, err := os.ReadDir(dir)
	if err != nil || len(files) != 1 {
		t.Fatalf("files=%v error=%v", files, err)
	}
	if !regexp.MustCompile(`^card_print_ref_init_failures_[0-9]{8}T[0-9]{15}Z\.log$`).MatchString(files[0].Name()) {
		t.Fatalf("failure filename=%s", files[0].Name())
	}
	content, err := os.ReadFile(filepath.Join(dir, files[0].Name()))
	if err != nil || !strings.Contains(string(content), missing) || !strings.Contains(string(content), `printing not found\nsource unavailable`) || bytes.Count(content, []byte("\n")) != 1 {
		t.Fatalf("failure content=%q error=%v", content, err)
	}
	for _, field := range []string{`"total":3`, `"synced":2`, `"failed":1`, `"failure_file":`, `"application initialization completed"`} {
		if !strings.Contains(output.String(), field) {
			t.Fatalf("missing %s in logs: %s", field, output.String())
		}
	}
}

func TestPrintingRefSuccessDoesNotCreateFailureFileAndReadErrorsSurface(t *testing.T) {
	var output bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&output, nil))
	dir := filepath.Join(t.TempDir(), "unused")
	source := &printingRefStub{pending: map[string]bool{"one": true}, fail: map[string]error{}}
	if err := Run(context.Background(), log, PrintingRefs(source, dir, log)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("successful initialization created a log directory: %v", err)
	}
	source.readErr = errors.New("unused query must not run")
	if err := Run(context.Background(), log, PrintingRefs(source, dir, log)); err != nil {
		t.Fatalf("empty pending set queried the source: %v", err)
	}
	source.pending["one"] = true
	source.readErr = errors.New("pending list unavailable")
	if err := Run(context.Background(), log, PrintingRefs(source, dir, log)); !errors.Is(err, source.readErr) {
		t.Fatalf("pending-list error=%v", err)
	}
}

func TestFailureFileErrorDoesNotInterruptRemainingRecords(t *testing.T) {
	var output bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&output, nil))
	path := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(path, []byte("file"), 0600); err != nil {
		t.Fatal(err)
	}
	source := &printingRefStub{pending: map[string]bool{"one": true, "two": true, "three": true}, fail: map[string]error{"one": errors.New("missing"), "three": errors.New("missing")}}
	if err := Run(context.Background(), log, PrintingRefs(source, path, log)); err == nil {
		t.Fatal("failure file error was silently ignored")
	}
	if len(source.visited) != 3 || source.pending["two"] || !strings.Contains(output.String(), "initialization failure file unavailable") {
		t.Fatalf("visited=%v pending=%v logs=%s", source.visited, source.pending, output.String())
	}
}
