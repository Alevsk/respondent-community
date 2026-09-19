package tasks

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog"
)

// Scheduler runs schedule-triggered task pipelines on their configured
// intervals or cron expressions. It mirrors the analysis engine's scheduling
// pattern using robfig/cron for cron expressions and time.Ticker for intervals.
type Scheduler struct {
	engine      *Engine
	definitions map[string]*TaskDefinition
	cronRunner  *cron.Cron
	tickers     []*time.Ticker
	logger      zerolog.Logger
	cancelFn    context.CancelFunc
	wg          sync.WaitGroup
}

// NewScheduler creates a Scheduler wired to the given engine and definitions.
func NewScheduler(engine *Engine, definitions map[string]*TaskDefinition, logger zerolog.Logger) *Scheduler {
	return &Scheduler{
		engine:      engine,
		definitions: definitions,
		logger:      logger,
	}
}

// Start begins running all schedule-triggered task definitions. It blocks
// until the context is cancelled or Stop() is called.
func (s *Scheduler) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	s.cancelFn = cancel

	s.cronRunner = cron.New(cron.WithSeconds())

	enabledCount := 0
	for name, def := range s.definitions {
		if !def.Enabled {
			continue
		}
		if def.Trigger.Type != "schedule" {
			continue
		}

		if err := s.scheduleDefinition(ctx, def); err != nil {
			cancel()
			return fmt.Errorf("schedule task %q: %w", name, err)
		}
		enabledCount++
	}

	s.cronRunner.Start()

	s.logger.Info().
		Int("total", len(s.definitions)).
		Int("scheduled", enabledCount).
		Msg("task scheduler started")

	<-ctx.Done()
	return nil
}

// Stop gracefully shuts down the scheduler.
func (s *Scheduler) Stop() {
	if s.cronRunner != nil {
		s.cronRunner.Stop()
	}
	for _, t := range s.tickers {
		t.Stop()
	}
	if s.cancelFn != nil {
		s.cancelFn()
	}
	s.wg.Wait()
	s.logger.Info().Msg("task scheduler stopped")
}

// scheduleDefinition sets up scheduling for a single task definition.
func (s *Scheduler) scheduleDefinition(ctx context.Context, def *TaskDefinition) error {
	if def.Trigger.Cron != "" {
		return s.scheduleCron(ctx, def)
	}
	return s.scheduleInterval(ctx, def)
}

// scheduleCron uses robfig/cron for cron-based scheduling.
func (s *Scheduler) scheduleCron(ctx context.Context, def *TaskDefinition) error {
	cronExpr := def.Trigger.Cron
	defCopy := def // capture for closure

	_, err := s.cronRunner.AddFunc(cronExpr, func() {
		if ctx.Err() != nil {
			return
		}
		s.wg.Add(1)
		defer s.wg.Done()
		if _, execErr := s.engine.ExecuteTask(ctx, defCopy, nil); execErr != nil {
			s.logger.Error().Err(execErr).Str("task", defCopy.Name).Msg("scheduled task execution failed")
		}
	})
	if err != nil {
		return fmt.Errorf("add cron job %q: %w", cronExpr, err)
	}

	s.logger.Info().
		Str("task", def.Name).
		Str("cron", cronExpr).
		Msg("scheduled task (cron)")
	return nil
}

// scheduleInterval uses time.Ticker for interval-based scheduling.
func (s *Scheduler) scheduleInterval(ctx context.Context, def *TaskDefinition) error {
	interval, err := time.ParseDuration(def.Trigger.Interval)
	if err != nil {
		return fmt.Errorf("parse interval %q: %w", def.Trigger.Interval, err)
	}

	ticker := time.NewTicker(interval)
	s.tickers = append(s.tickers, ticker)

	defCopy := def // capture for closure
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.wg.Add(1)
				func() {
					defer s.wg.Done()
					if _, execErr := s.engine.ExecuteTask(ctx, defCopy, nil); execErr != nil {
						s.logger.Error().Err(execErr).Str("task", defCopy.Name).Msg("scheduled task execution failed")
					}
				}()
			}
		}
	}()

	s.logger.Info().
		Str("task", def.Name).
		Str("interval", def.Trigger.Interval).
		Msg("scheduled task (interval)")
	return nil
}
