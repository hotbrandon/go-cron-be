package database

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/sijms/go-ora/v2"
)

func GetErpConnection() (*sql.DB, error) {
	// Use the ERP DSN from environment variables
	erpDsn := os.Getenv("ERP_DSN")

	// Connect to the ERP database
	db, err := sql.Open("oracle", erpDsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to ERP database: %w", err)
	}

	return db, nil
}
