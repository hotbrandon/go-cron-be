package main

import (
	"database/sql"
	"hotbrandon/go-cron-be/internal/scheduler"
	"log"
	"log/slog"
	"os"

	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	// load environment variables
	godotenv.Load(".env")
	SQLITE_PATH := os.Getenv("SQLITE_PATH")
	if SQLITE_PATH == "" {
		log.Fatal("SQLITE_PATH environment variable is not set")
	}

	// Initialize the logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	db, err := sql.Open("sqlite3", SQLITE_PATH)
	if err != nil {
		slog.Error("Error opening database:", "error", err)
	}
	// Defer the closing of the connection until the surrounding function returns.
	defer db.Close()

	scheduler := scheduler.NewScheduler(db, logger)
	scheduler.Start()
	defer scheduler.Stop()

	select {}
}
