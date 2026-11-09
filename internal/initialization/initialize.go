package initialization

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// Step defines one ordered startup operation that finishes before HTTP traffic is accepted.
type Step struct {
	Name string
	Run  func(context.Context) error
}

// Run executes startup steps in order and stops on cancellation or a step-level failure.
func Run(ctx context.Context, log *slog.Logger, steps ...Step) error {
	started := time.Now()
	log.Info("application initialization started", "steps", len(steps))
	for _, step := range steps {
		if err := ctx.Err(); err != nil {
			log.Error("application initialization interrupted", "error", err)
			return err
		}
		stepStarted := time.Now()
		log.Info("initialization step started", "step", step.Name)
		err := step.Run(ctx)
		if err == nil {
			err = ctx.Err()
		}
		if err != nil {
			log.Error("initialization step failed", "step", step.Name, "duration_ms", time.Since(stepStarted).Milliseconds(), "error", err)
			return fmt.Errorf("initialize %s: %w", step.Name, err)
		}
		log.Info("initialization step completed", "step", step.Name, "duration_ms", time.Since(stepStarted).Milliseconds())
	}
	log.Info("application initialization completed", "steps", len(steps), "duration_ms", time.Since(started).Milliseconds())
	return nil
}
