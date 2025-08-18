package scheduler

import (
	"database/sql"
	"log/slog"
	"time"

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

// withJobLogging is a helper that wraps a job function with logging for its execution time and recovery from panics.
func (s *Scheduler) withJobLogging(jobName string, job func()) func() {
	return func() {
		start := time.Now()
		s.logger.Info("Job started", "job", jobName)

		defer func() {
			duration := time.Since(start)
			if r := recover(); r != nil {
				s.logger.Error("Job panicked", "job", jobName, "duration", duration, "panic", r)
			} else {
				s.logger.Info("Job finished", "job", jobName, "duration", duration)
			}
		}()

		job()
	}
}

func (s *Scheduler) Start() {
	// Add jobs before starting

	// Schedule the job to run daily at 8:00 PM (20:00).
	// The standard cron format is: Minute Hour Day-of-Month Month Day-of-Week
	// "0 20 * * *" means at minute 0 of hour 20 every day.
	s.c.AddFunc("@every 10m", s.withJobLogging("get-funeral-invoice", func() {
		invoices, err := GetFuneralInvoiceByDate(time.Now())
		if err != nil {
			s.logger.Error("Job failed", "job", "get-funeral-invoice", "error", err)
		} else {
			s.logger.Info("Job finished", "job", "get-funeral-invoice", "invoices_fetched", len(invoices))
		}
	}))

	s.logger.Info("Scheduler started")
	s.c.Start()
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
