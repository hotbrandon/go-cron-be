package scheduler

import (
	"database/sql"
	"encoding/json"
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

type CronJob struct {
	JobID           int64      `json:"job_id"`
	JobName         string     `json:"job_name"`
	JobDate         string     `json:"job_date"`
	JobParams       string     `json:"job_params"`
	JobStatus       string     `json:"job_status"`
	Message         string     `json:"message"`
	ExecutionTimeMs int64      `json:"execution_time_ms"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	FinishedAt      *time.Time `json:"finished_at"`
}

type JobParams struct {
	DbID    string `json:"db_id"`
	JobDate string `json:"job_date"`
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

	CronJobsTable := `
	CREATE TABLE IF NOT EXISTS cron_jobs (
		job_id INT PRIMARY KEY AUTO_INCREMENT,
		job_name VARCHAR(255) NOT NULL,
		job_date VARCHAR(10) NOT NULL,
		job_params JSON,
		job_params_hash VARCHAR(64) AS (SHA2(job_params, 256)) STORED,
		job_status VARCHAR(10) NOT NULL DEFAULT 'pending',
		message TEXT,
		execution_time_ms BIGINT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
		finished_at DATETIME,
		UNIQUE KEY unique_job (job_name, job_date, job_params_hash)
	);`

	indexes := []string{
		"CREATE INDEX idx_cron_jobs_status ON cron_jobs(job_status);",
		"CREATE INDEX idx_cron_jobs_job_name_date ON cron_jobs(job_name, job_date);",
	}

	if _, err := s.db.Exec(funeralInvoicesTable); err != nil {
		return fmt.Errorf("creating funeral_invoices table: %w", err)
	}

	if _, err := s.db.Exec(CronJobsTable); err != nil {
		return fmt.Errorf("creating cron_jobs table: %w", err)
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

	_, err := s.c.AddFunc("* 12 * * *", func() {
		s.CreateGolfJob()
	})
	if err != nil {
		return fmt.Errorf("error registering golf jobs: %w", err)
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

func (s *Scheduler) CreateGolfJob() {

	jobDate := time.Now().Format("2006-01-02")
	for _, db_id := range []string{"GC", "TH", "OS"} {
		paramsJSON, _ := json.Marshal(JobParams{DbID: db_id, JobDate: jobDate})

		query := `
			INSERT INTO cron_jobs (job_name, job_date, job_params)
			VALUES (?, ?, ?)
		`
		result, err := s.db.Exec(query, "golf", jobDate, string(paramsJSON))
		if err != nil {
			s.logger.Error("failed creating golf jobs", "error", err)
			return
		} else {
			insertedId, _ := result.LastInsertId()
			s.logger.Info("golf job created", "job_id", insertedId)
		}
	}
}

func (s *Scheduler) RunGolfJob() {
	var job CronJob
	var jobs []CronJob
	query := `
		SELECT 
			job_id, job_name, job_date, job_params
		FROM cron_jobs
		WHERE job_name = 'golf' AND job_status <> 'finished'
	`
	rows, err := s.db.Query(query)
	if err != nil {
		s.logger.Error("querying cron_jobs:", "error", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		if err := rows.Scan(&job.JobID, &job.JobName, &job.JobDate, &job.JobParams); err != nil {
			s.logger.Error("scanning row:", "error", err)
			return
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		s.logger.Error("rows error:", "error", err)
		return
	}

	var jobParam JobParams
	for _, job := range jobs {
		if err := json.Unmarshal([]byte(job.JobParams), &jobParam); err != nil {
			s.logger.Error("failed to unmarshal job_params:", "error", err)
			return
		}

		// The layout must match the format used when creating the date string.
		const layout = "2006-01-02"
		jobDate, err := time.Parse(layout, jobParam.JobDate)
		if err != nil {
			// If parsing fails, log the error and continue to the next job.
			s.logger.Error("Failed to parse job_date for job", "job_id", job.JobID, "date_string", jobParam.JobDate, "error", err)
			continue
		}

		summary, err := GetReservationSummary(jobParam.DbID, jobDate)
		if err != nil {
			// If the job execution fails, log the error and continue to the next job.
			s.logger.Error("Failed to get reservation summary for job", "job_id", job.JobID, "db_id", jobParam.DbID, "error", err)
			continue
		}
		s.logger.Info("Successfully ran golf job", "job_id", job.JobID, "db_id", jobParam.DbID, "summary", summary)
	}
}
