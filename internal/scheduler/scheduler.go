package scheduler

import (
	"database/sql"
	"log/slog"
)

type Scheduler struct {
	db     *sql.DB
	logger *slog.Logger
}

func NewScheduler(db *sql.DB, logger *slog.Logger) *Scheduler {
	return &Scheduler{
		db:     db,
		logger: logger,
	}
}

func (s *Scheduler) Start() {
	s.logger.Info("Scheduler started")
	// Add logic to start the scheduler, such as initializing tasks or setting up cron jobs.
}
