package database

import (
	"database/sql"
	"fmt"
	"os"
	"strings"

	_ "github.com/sijms/go-ora/v2"
)

func GetGolfConnection(site_id string) (*sql.DB, error) {
	// Use the GOLF DSN from environment variables
	var golfDsn string
	switch strings.ToUpper(site_id) {
	case "GC":
		golfDsn = os.Getenv("ORACLE_DSN_GC")
	case "TH":
		golfDsn = os.Getenv("ORACLE_DSN_TH")
	case "OS":
		golfDsn = os.Getenv("ORACLE_DSN_OS")
	}

	if golfDsn == "" {
		return nil, fmt.Errorf("GOLF_DSN_XX not found for site_id: %s", strings.ToUpper(site_id))
	}

	// Connect to the ERP database
	db, err := sql.Open("oracle", golfDsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to GOLF database for site_id: %s: %w", strings.ToUpper(site_id), err)
	}

	return db, nil
}
