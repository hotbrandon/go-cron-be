package scheduler

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

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
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		invoice_date TEXT NOT NULL,
		c_idno2 TEXT NOT NULL,
		total_amount_dividint10 INTEGER NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(invoice_date, c_idno2)
	);`

	jobExecutionsTable := `
	CREATE TABLE IF NOT EXISTS job_executions (
		job_id INTEGER PRIMARY KEY AUTOINCREMENT,
		job_name TEXT NOT NULL,
		job_date TEXT NOT NULL,
		job_params TEXT,
		job_status TEXT NOT NULL CHECK (job_status IN ('pending', 'running', 'finished', 'failed', 'retrying')) DEFAULT 'pending',
		message TEXT,
		execution_time_ms INTEGER,
		retry_count INTEGER DEFAULT 0,
		max_retries INTEGER DEFAULT 3,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		finished_at DATETIME
	);`

	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_job_executions_status ON job_executions(job_status);",
		"CREATE INDEX IF NOT EXISTS idx_job_executions_job_name_date ON job_executions(job_name, job_date);",
		"CREATE INDEX IF NOT EXISTS idx_funeral_invoices_date ON funeral_invoices(invoice_date);",
	}

	if _, err := s.db.Exec(funeralInvoicesTable); err != nil {
		return fmt.Errorf("creating funeral_invoices table: %w", err)
	}

	if _, err := s.db.Exec(jobExecutionsTable); err != nil {
		return fmt.Errorf("creating job_executions table: %w", err)
	}

	for _, idx := range indexes {
		if _, err := s.db.Exec(idx); err != nil {
			return fmt.Errorf("creating index: %w", err)
		}
	}

	return nil
}

// createJobExecution creates a new job execution record
func (s *Scheduler) createJobExecution(jobName, jobDate string, params interface{}, maxRetries int) (int64, error) {
	paramsJSON, _ := json.Marshal(params)

	query := `
		INSERT INTO job_executions (job_name, job_date, job_params, max_retries)
		VALUES (?, ?, ?, ?)
	`
	result, err := s.db.Exec(query, jobName, jobDate, string(paramsJSON), maxRetries)
	if err != nil {
		return 0, fmt.Errorf("creating job execution: %w", err)
	}

	return result.LastInsertId()
}

// updateJobExecution updates job execution status and details
func (s *Scheduler) updateJobExecution(jobID int64, status, message string, executionTimeMs int64, retryCount int) error {
	query := `
		UPDATE job_executions 
		SET job_status = ?, message = ?, execution_time_ms = ?, retry_count = ?, updated_at = CURRENT_TIMESTAMP,
		    finished_at = CASE WHEN ? IN ('finished', 'failed') THEN CURRENT_TIMESTAMP ELSE finished_at END
		WHERE job_id = ?
	`
	_, err := s.db.Exec(query, status, message, executionTimeMs, retryCount, status, jobID)
	return err
}

// storeFuneralInvoices stores the fetched invoice data
func (s *Scheduler) storeFuneralInvoices(invoices []FuneralInvoiceRow) error {
	if len(invoices) == 0 {
		return nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT OR IGNORE INTO funeral_invoices (invoice_date, c_idno2, total_amount_dividint10) 
		VALUES (?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("preparing statement: %w", err)
	}
	defer stmt.Close()

	insertedCount := 0
	for _, invoice := range invoices {
		result, err := stmt.Exec(invoice.InvoiceDate, invoice.CustomerID, invoice.TotalAmount)
		if err != nil {
			return fmt.Errorf("inserting invoice: %w", err)
		}
		if rowsAffected, _ := result.RowsAffected(); rowsAffected > 0 {
			insertedCount++
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	s.logger.Info("Stored funeral invoices", "total_fetched", len(invoices), "newly_inserted", insertedCount)
	return nil
}

// executeFuneralInvoiceJob executes the funeral invoice fetching job with retry logic
func (s *Scheduler) executeFuneralInvoiceJob(targetDate time.Time) {
	jobName := "get-funeral-invoice"
	jobDate := targetDate.Format("2006-01-02")
	maxRetries := 3

	// Create job execution record
	jobID, err := s.createJobExecution(jobName, jobDate, JobParams{InvoiceDate: jobDate}, maxRetries)
	if err != nil {
		s.logger.Error("Failed to create job execution record", "error", err)
		return
	}

	s.executeJobWithRetry(jobID, jobName, targetDate)
}

// executeJobWithRetry handles the actual job execution with retry logic
func (s *Scheduler) executeJobWithRetry(jobID int64, jobName string, targetDate time.Time) {
	start := time.Now()
	retryCount := 0
	maxRetries := 3

	for retryCount <= maxRetries {
		// Update status to running/retrying
		status := "running"
		if retryCount > 0 {
			status = "retrying"
		}
		s.updateJobExecution(jobID, status, "", 0, retryCount)

		s.logger.Info("Executing job", "job_id", jobID, "job", jobName, "retry_count", retryCount)

		// Execute the actual job
		invoices, err := GetFuneralInvoiceByDate(targetDate)
		executionTime := time.Since(start).Milliseconds()

		if err != nil {
			retryCount++
			message := fmt.Sprintf("Job failed (attempt %d/%d): %v", retryCount, maxRetries+1, err)
			s.logger.Error("Job execution failed", "job_id", jobID, "job", jobName, "error", err, "retry_count", retryCount)

			if retryCount > maxRetries {
				// Max retries exceeded, mark as failed
				s.updateJobExecution(jobID, "failed", message, executionTime, retryCount-1)
				return
			}

			// Update with retry status and wait before retrying
			s.updateJobExecution(jobID, "retrying", message, executionTime, retryCount-1)

			// Exponential backoff: 1min, 2min, 4min
			backoffDuration := time.Duration(1<<uint(retryCount-1)) * time.Minute
			s.logger.Info("Retrying job after backoff", "job_id", jobID, "backoff_duration", backoffDuration)
			time.Sleep(backoffDuration)
			continue
		}

		// Job succeeded, store the data
		if err := s.storeFuneralInvoices(invoices); err != nil {
			message := fmt.Sprintf("Job completed but failed to store data: %v", err)
			s.updateJobExecution(jobID, "failed", message, executionTime, retryCount)
			s.logger.Error("Failed to store job results", "job_id", jobID, "error", err)
			return
		}

		// Success
		message := fmt.Sprintf("Successfully fetched and stored %d invoices", len(invoices))
		s.updateJobExecution(jobID, "finished", message, executionTime, retryCount)
		s.logger.Info("Job completed successfully", "job_id", jobID, "job", jobName, "invoices_count", len(invoices), "execution_time_ms", executionTime)
		return
	}
}

// RegisterJobs registers all scheduled jobs
func (s *Scheduler) RegisterJobs() error {
	// Initialize database tables
	if err := s.initializeTables(); err != nil {
		return fmt.Errorf("initializing database tables: %w", err)
	}

	// Register the funeral invoice job to run daily at 8:00 PM
	// Cron format: "0 20 * * *" = At minute 0 of hour 20 (8:00 PM) every day
	_, err := s.c.AddFunc("0 20 * * *", func() {
		s.executeFuneralInvoiceJob(time.Now())
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

// GetJobExecutions retrieves job execution history
func (s *Scheduler) GetJobExecutions(limit int) ([]JobExecution, error) {
	query := `
		SELECT job_id, job_name, job_date, job_params, job_status, message, 
		       execution_time_ms, retry_count, max_retries, created_at, updated_at, finished_at
		FROM job_executions 
		ORDER BY created_at DESC
		LIMIT ?
	`
	rows, err := s.db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("querying job executions: %w", err)
	}
	defer rows.Close()

	var executions []JobExecution
	for rows.Next() {
		var exec JobExecution
		err := rows.Scan(&exec.JobID, &exec.JobName, &exec.JobDate, &exec.JobParams,
			&exec.JobStatus, &exec.Message, &exec.ExecutionTimeMs, &exec.RetryCount,
			&exec.MaxRetries, &exec.CreatedAt, &exec.UpdatedAt, &exec.FinishedAt)
		if err != nil {
			return nil, fmt.Errorf("scanning job execution: %w", err)
		}
		executions = append(executions, exec)
	}

	return executions, nil
}
