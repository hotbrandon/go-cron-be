package scheduler

import (
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/robfig/cron/v3"
)

type Scheduler struct {
	db     *sql.DB
	logger *slog.Logger
	c      *cron.Cron
}

type JobExecution struct {
	JobID           int64      `json:"job_id"`
	JobName         string     `json:"job_name"`
	JobDate         string     `json:"job_date"`
	JobParams       string     `json:"job_params"`
	JobStatus       string     `json:"job_status"`
	Message         string     `json:"message"`
	ExecutionTimeMs int64      `json:"execution_time_ms"`
	RetryCount      int        `json:"retry_count"`
	MaxRetries      int        `json:"max_retries"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	FinishedAt      *time.Time `json:"finished_at"`
}

type JobParams struct {
	InvoiceDate string `json:"invoice_date"`
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

// initializeTables creates the required database tables if they don't exist
func (s *Scheduler) initializeTables() error {
	funeralInvoicesTable := `
	CREATE TABLE IF NOT EXISTS funeral_invoices (
		id INT PRIMARY KEY AUTO_INCREMENT,
		invoice_date VARCHAR(10) NOT NULL,
		c_idno2 VARCHAR(50) NOT NULL,
		total_amount_dividint10 INT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(invoice_date, c_idno2)
	);`

	jobExecutionsTable := `
	CREATE TABLE IF NOT EXISTS job_executions (
		job_id INT PRIMARY KEY AUTO_INCREMENT,
		job_name VARCHAR(255) NOT NULL,
		job_date VARCHAR(10) NOT NULL,
		job_params TEXT,
		job_status VARCHAR(10) NOT NULL DEFAULT 'pending',
		message TEXT,
		execution_time_ms BIGINT,
		retry_count INT DEFAULT 0,
		max_retries INT DEFAULT 3,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
		finished_at DATETIME
	);`

	indexes := []string{
		"CREATE INDEX idx_job_executions_status ON job_executions(job_status);",
		"CREATE INDEX idx_job_executions_job_name_date ON job_executions(job_name, job_date);",
		"CREATE INDEX idx_funeral_invoices_date ON funeral_invoices(invoice_date);",
	}

	if _, err := s.db.Exec(funeralInvoicesTable); err != nil {
		return fmt.Errorf("creating funeral_invoices table: %w", err)
	}

	if _, err := s.db.Exec(jobExecutionsTable); err != nil {
		return fmt.Errorf("creating job_executions table: %w", err)
	}

	for _, idx := range indexes {
		if _, err := s.db.Exec(idx); err != nil {
			// Check if the error is a MySQL-specific "duplicate key name" error (code 1061)
			if mysqlErr, ok := err.(*mysql.MySQLError); ok && mysqlErr.Number == 1061 {
				s.logger.Debug("Index already exists, skipping creation.", "query", idx)
			} else {
				// For any other error, log it as a warning.
				// A proper migration tool is better, but this is a good compromise.
				s.logger.Warn("Could not create index.", "query", idx, "error", err)
			}
		}
	}

	return nil
}

// RegisterJobs registers all scheduled jobs
func (s *Scheduler) RegisterJobs() error {
	// Initialize database tables
	if err := s.initializeTables(); err != nil {
		return fmt.Errorf("initializing database tables: %w", err)
	}

	_, err := s.c.AddFunc("*/10 * * * *", func() {
		result, err := GetReservationSummary("GC", time.Now())
		if err != nil {
			s.logger.Error("Error getting reservation summary", "error", err)
			return
		}
		s.logger.Info("Reservation summary", "result", result)
	})
	if err != nil {
		return fmt.Errorf("registering funeral invoice job: %w", err)
	}

	s.logger.Info("Jobs registered successfully")
	return nil
}

// Start initializes and starts the scheduler
func (s *Scheduler) Start() error {
	// Register jobs before starting
	if err := s.RegisterJobs(); err != nil {
		return fmt.Errorf("registering jobs: %w", err)
	}

	s.logger.Info("Scheduler started")
	s.c.Start()
	return nil
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
