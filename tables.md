# required tables for SQLite

```sql
-- Table to store funeral invoice data
CREATE TABLE IF NOT EXISTS funeral_invoices (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    invoice_date TEXT NOT NULL,
    c_idno2 TEXT NOT NULL,
    total_amount_dividint10 INTEGER NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(invoice_date, c_idno2) -- Prevent duplicates
);

-- Table to record job execution status
CREATE TABLE IF NOT EXISTS job_executions (
    job_id INTEGER PRIMARY KEY AUTOINCREMENT,
    job_name TEXT NOT NULL,
    job_date TEXT NOT NULL, -- The date parameter for the job (e.g., invoice date to fetch)
    job_params TEXT, -- JSON string of parameters
    job_status TEXT NOT NULL CHECK (job_status IN ('pending', 'running', 'finished', 'failed', 'retrying')) DEFAULT 'pending',
    message TEXT, -- Success message, error message, or API response
    execution_time_ms INTEGER, -- Execution time in milliseconds
    retry_count INTEGER DEFAULT 0,
    max_retries INTEGER DEFAULT 3,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    finished_at DATETIME
);

-- Index for efficient querying
CREATE INDEX IF NOT EXISTS idx_job_executions_status ON job_executions(job_status);
CREATE INDEX IF NOT EXISTS idx_job_executions_job_name_date ON job_executions(job_name, job_date);
CREATE INDEX IF NOT EXISTS idx_funeral_invoices_date ON funeral_invoices(invoice_date);
```