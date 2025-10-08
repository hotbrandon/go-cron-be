package scheduler

import (
	"database/sql"
	"hotbrandon/go-cron-be/internal/database"
	"time"
)

type ReservationSummary struct {
	DataName string
	AmtD     int
	AmtM     int
	AmtY     int
}

func GetReservationSummary(site_id string, resvDate time.Time) (ReservationSummary, error) {
	db, err := database.GetGolfConnection(site_id)
	if err != nil {
		return ReservationSummary{}, err
	}
	defer db.Close()

	// Calculate date ranges based on the input resvDate
	year, month, _ := resvDate.Date()
	loc := resvDate.Location()

	firstOfMonth := time.Date(year, month, 1, 0, 0, 0, 0, loc)
	lastOfMonth := firstOfMonth.AddDate(0, 1, -1)

	firstOfYear := time.Date(year, time.January, 1, 0, 0, 0, 0, loc)
	lastOfYear := time.Date(year, time.December, 31, 0, 0, 0, 0, loc)

	query := `
	SELECT '預約組數',
            (
                SELECT sum(b.est_cnt) 
                FROM glf_stk_mn a, glf_rev_mn b
                WHERE a.rev_no = b.rev_no
                AND a.ple_date = :resv_date
                AND b.stat <> 'X'
            ) AmtD,
            (
                SELECT sum(b.est_cnt) 
                FROM glf_stk_mn a, glf_rev_mn b
                WHERE a.rev_no = b.rev_no
                AND a.ple_date BETWEEN :resv_date_mb AND :resv_date_me
                AND b.stat <> 'X'
            ) AmtM,
            (
                SELECT sum(b.est_cnt) 
                FROM glf_stk_mn a, glf_rev_mn b
                WHERE a.rev_no = b.rev_no
                AND a.ple_date BETWEEN :resv_date_yb AND :resv_date_ye
                AND b.stat <> 'X'
            ) AmtY
            FROM dual
			`

	var summary ReservationSummary
	// Use sql.Named to pass parameters by name, which is supported by the Oracle driver.
	// The driver will handle the time.Time to Oracle DATE conversion.
	err = db.QueryRow(query,
		sql.Named("resv_date", resvDate),
		sql.Named("resv_date_mb", firstOfMonth),
		sql.Named("resv_date_me", lastOfMonth),
		sql.Named("resv_date_yb", firstOfYear),
		sql.Named("resv_date_ye", lastOfYear),
	).Scan(&summary.DataName, &summary.AmtD, &summary.AmtM, &summary.AmtY)

	if err != nil {
		return ReservationSummary{}, err
	}

	return summary, nil
}
