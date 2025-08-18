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

	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	// load environment variables
	if err := godotenv.Load(".env"); err != nil {
		// it's OK to continue if .env is absent in some deployments, but log it explicitly
		log.Println("Warning: .env not loaded:", err)
	}

	SQLITE_PATH := os.Getenv("SQLITE_PATH")
	if SQLITE_PATH == "" {
		log.Fatal("SQLITE_PATH environment variable is not set")
	}

	erpDsn := os.Getenv("ERP_DSN")
	if erpDsn == "" {
		log.Fatal("ERP_DSN environment variable not set")
	}

	// Initialize the logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	db, err := sql.Open("sqlite3", SQLITE_PATH)
	if err != nil {
		slog.Error("Error opening database", "error", err)
		log.Fatal(err)
	}

	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(time.Minute * 60)

	// verify DB is reachable
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		logger.Error("Error pinging DB", "error", err)
		db.Close()
		log.Fatal(err)
	}

	// Ensure DB closed on exit
	defer func() {
		if err := db.Close(); err != nil {
			logger.Warn("Failed to close DB on shutdown", "error", err, "sqlite_path", SQLITE_PATH)
		}
	}()

	sched := scheduler.NewScheduler(db, logger)

	// Start the scheduler (this will register jobs and start the cron)
	if err := sched.Start(); err != nil {
		logger.Error("Failed to start scheduler", "error", err)
		log.Fatal(err)
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
