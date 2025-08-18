package scheduler

import (
	"fmt"
	"hotbrandon/go-cron-be/internal/database"
	"time"
)

type FuneralInvoiceRow struct {
	// 發票日期
	InvoiceDate string `json:"invoice_date"`
	// 主事者ID
	CustomerID string `json:"c_idno2"`
	// 含稅額(除以10)
	TotalAmount int `json:"total_amount_dividint10"`
}

func GetFuneralInvoiceByDate(invoiceDate time.Time) ([]FuneralInvoiceRow, error) {
	// Get the ERP database connection
	db, err := database.GetErpConnection()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	// Pass the time.Time object directly. The driver will handle the conversion to Oracle's DATE type.
	_, err = db.Exec("BEGIN ARGOERP.GOBO_P_UIBF062_V(:1); END;", invoiceDate)
	if err != nil {
		return nil, fmt.Errorf("calling ARGOERP.GOBO_P_UIBF062_V: %w", err)
	}

	query := `
		SELECT 
			invoice_date,
			c_idno2,
			total_amount_dividint10
		FROM GOBO_UIBF062_V2
	`
	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("querying GOBO_UIBF062_V2: %w", err)
	}
	defer rows.Close()

	var invoices []FuneralInvoiceRow
	for rows.Next() {
		var invoice FuneralInvoiceRow
		if err := rows.Scan(&invoice.InvoiceDate, &invoice.CustomerID, &invoice.TotalAmount); err != nil {
			return nil, fmt.Errorf("scanning row: %w", err)
		}
		invoices = append(invoices, invoice)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return invoices, nil
}
