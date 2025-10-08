package main

import (
	"context"
	"database/sql"
	"hotbrandon/go-cron-be/internal/scheduler"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
)

func showEnvironments(logger *slog.Logger) {
	logger = logger.WithGroup("ENV")

	logger.Info("Environment variables",
		"TZ", os.Getenv("TZ"),
		"ERP_DSN", os.Getenv("ERP_DSN"),
		"MYSQL_DSN", os.Getenv("MYSQL_DSN"),
	)
}

func main() {
	// load environment variables
	if err := godotenv.Load(".env"); err != nil {
		// it's OK to continue if .env is absent in some deployments, but log it explicitly
		log.Println("Warning: .env not loaded:", err)
	}

	var logLevel slog.Level
	switch os.Getenv("LOG_LEVEL") {
	case "DEBUG":
		logLevel = slog.LevelDebug
	case "WARN":
		logLevel = slog.LevelWarn
	case "ERROR":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo // Default to INFO
	}
	// Initialize the logger
	handlerOpts := &slog.HandlerOptions{
		// Set the minimum log level. Anything below this level will be discarded.
		Level: logLevel,
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, handlerOpts))
	slog.SetDefault(logger)

	mysqlDsn := os.Getenv("MYSQL_DSN")
	if mysqlDsn == "" {
		slog.Error("MYSQL_DSN environment variable is not set")
		os.Exit(1)
	}

	erpDsn := os.Getenv("ERP_DSN")
	if erpDsn == "" {
		slog.Error("ERP_DSN environment variable is not set")
		os.Exit(1)
	}

	showEnvironments(logger)

	// Connect to the MySQL database
	db, err := sql.Open("mysql", mysqlDsn)
	if err != nil {
		slog.Error("Error opening database", "error", err)
		os.Exit(1)
	}

	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(time.Minute * 60)

	// verify DB is reachable
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		logger.Error("Error pinging DB", "error", err)
		_ = db.Close()
		os.Exit(1)
	}

	// Ensure DB closed on exit
	defer func() {
		if err := db.Close(); err != nil {
			logger.Warn("Failed to close DB on shutdown", "error", err)
		}
	}()

	sched := scheduler.NewScheduler(db, logger)

	// Start the scheduler (this will register jobs and start the cron)
	if err := sched.Start(); err != nil {
		slog.Error("Failed to start scheduler", "error", err)
		os.Exit(1)
	}
	defer sched.Stop()

	// Optional: Show scheduled entries for debugging
	sched.ShowEntries()

	// graceful shutdown on signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	logger.Info("Shutdown signal received, exiting")
}
