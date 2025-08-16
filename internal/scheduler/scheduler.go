package scheduler

import (
	"database/sql"
	"log/slog"

	"github.com/robfig/cron/v3"
)

type Scheduler struct {
	db     *sql.DB
	logger *slog.Logger
	c      *cron.Cron
}

func NewScheduler(db *sql.DB, logger *slog.Logger) *Scheduler {
	c := cron.New()
	return &Scheduler{
		c:      c,
		db:     db,
		logger: logger,
	}
}

func (s *Scheduler) Stop() {
	s.logger.Info("Scheduler stopped")
	s.c.Stop()
}

func (s *Scheduler) Start() {
	s.logger.Info("Scheduler started")
	s.c.Start()
	s.c.AddFunc("@every 1m", func() {
		s.logger.Info("Running scheduled job every minute")
		s.ShowEntries()
	})
}

func (s *Scheduler) AddJob(spec string, job cron.Job) (cron.EntryID, error) {
	id, err := s.c.AddJob(spec, job)
	if err != nil {
		s.logger.Error("Error adding job to scheduler", "error", err)
		return 0, err
	}
	s.logger.Info("Job added to scheduler", "id", id)
	return id, nil
}

func (s *Scheduler) ShowEntries() {
	entries := s.c.Entries()
	s.logger.Info("Current scheduled entries", "count", len(entries))
	for _, entry := range entries {
		s.logger.Info("Entry", "id", entry.ID, "schedule", entry.Schedule, "next", entry.Next, "prev", entry.Prev)
	}
}
