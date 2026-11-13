package initialization

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"mtgnissa/internal/appdata"
)

// PrintingRefSource provides persistence operations for personal query metadata initialization.
type PrintingRefSource interface {
	CountPending(context.Context) (int64, error)
	Pending(context.Context, string, int) ([]string, error)
	Load(context.Context, []string) (map[string]appdata.CardPrintRef, error)
	Update(context.Context, appdata.CardPrintRef) error
}

type failureLog struct {
	dir  string
	path string
	file *os.File
	err  error
}

// PrintingRefs defines the personal query metadata initialization step.
func PrintingRefs(source PrintingRefSource, logDir string, log *slog.Logger) Step {
	return Step{Name: "card_print_ref", Run: func(ctx context.Context) error {
		return refreshPrintingRefs(ctx, source, logDir, log)
	}}
}

func refreshPrintingRefs(ctx context.Context, source PrintingRefSource, logDir string, log *slog.Logger) (resultErr error) {
	started := time.Now()
	var total, synced, failed int64
	journal := failureLog{dir: logDir}
	defer func() {
		resultErr = errors.Join(resultErr, journal.close())
		attributes := []any{"total", total, "synced", synced, "failed", failed, "failure_file", journal.path, "duration_ms", time.Since(started).Milliseconds()}
		if resultErr != nil {
			log.Error("card printing reference initialization incomplete", append(attributes, "error", resultErr)...)
		} else if failed > 0 {
			log.Warn("card printing reference initialization completed", attributes...)
		} else {
			log.Info("card printing reference initialization completed", attributes...)
		}
	}()
	queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	total, err := source.CountPending(queryCtx)
	err = errors.Join(err, queryCtx.Err())
	cancel()
	if err != nil {
		return err
	}
	after := ""
	for total > 0 {
		if err := ctx.Err(); err != nil {
			return err
		}
		queryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		ids, err := source.Pending(queryCtx, after, 500)
		err = errors.Join(err, queryCtx.Err())
		cancel()
		if err != nil {
			return err
		}
		if len(ids) == 0 {
			break
		}
		queryCtx, cancel = context.WithTimeout(ctx, 30*time.Second)
		refs, readErr := source.Load(queryCtx, ids)
		readErr = errors.Join(readErr, queryCtx.Err())
		cancel()
		for _, id := range ids {
			refreshErr := readErr
			if refreshErr == nil {
				ref, ok := refs[strings.ToLower(id)]
				if !ok {
					refreshErr = errors.New("data not found")
				} else {
					ref.PrintingUUID = id
					updateCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
					refreshErr = source.Update(updateCtx, ref)
					refreshErr = errors.Join(refreshErr, updateCtx.Err())
					cancel()
				}
			}
			if refreshErr == nil {
				synced++
				continue
			}
			failed++
			if err := journal.write(id, refreshErr); err != nil {
				log.Error("initialization failure file unavailable", "printing_uuid", id, "refresh_error", refreshErr, "error", err)
			}
			if errors.Is(refreshErr, context.DeadlineExceeded) || errors.Is(refreshErr, context.Canceled) {
				return refreshErr
			}
		}
		after = ids[len(ids)-1]
	}
	return ctx.Err()
}

func (l *failureLog) write(uuid string, cause error) error {
	if l.err != nil {
		return l.err
	}
	if l.file == nil {
		l.err = os.MkdirAll(l.dir, 0750)
		if l.err != nil {
			return l.err
		}
		now := time.Now().UTC()
		name := fmt.Sprintf("card_print_ref_init_failures_%s%09dZ.log", now.Format("20060102T150405"), now.Nanosecond())
		l.path = filepath.Join(l.dir, name)
		l.file, l.err = os.OpenFile(l.path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0640)
		if l.err != nil {
			return l.err
		}
	}
	_, l.err = fmt.Fprintf(l.file, "%s printing_uuid=%s error=%q\n", time.Now().UTC().Format(time.RFC3339Nano), uuid, cause.Error())
	return l.err
}

func (l *failureLog) close() error {
	if l.file != nil {
		return errors.Join(l.err, l.file.Close())
	}
	return l.err
}
