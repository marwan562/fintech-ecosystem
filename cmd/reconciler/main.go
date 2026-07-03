package main

import (
	"context"
	"database/sql"
	"log"
	"os"
	"time"

	_ "github.com/lib/pq"
	"github.com/sapliy/sapliy-core/pkg/database"
	"github.com/sapliy/sapliy-core/pkg/monitoring"
)

func main() {
	paymentsDSN := os.Getenv("PAYMENTS_DB_DSN")
	ledgerDSN := os.Getenv("LEDGER_DB_DSN")

	if paymentsDSN == "" || ledgerDSN == "" {
		log.Println("DSNs not set. Skipping reconciler start.")
		return
	}

	payDB, err := database.Connect(paymentsDSN)
	if err != nil {
		log.Fatalf("Failed to connect to Payments DB: %v", err)
	}
	defer func() {
		if err := payDB.Close(); err != nil {
			log.Printf("Failed to close Payments DB: %v", err)
		}
	}()

	ledgerDB, err := database.Connect(ledgerDSN)
	if err != nil {
		log.Fatalf("Failed to connect to Ledger DB: %v", err)
	}
	defer func() {
		if err := ledgerDB.Close(); err != nil {
			log.Printf("Failed to close Ledger DB: %v", err)
		}
	}()

	// Start Metrics Server
	monitoring.StartMetricsServer(":8081")

	log.Println("Reconciliation Checker started. Auditing every 1 minute...")

	ticker := time.NewTicker(1 * time.Minute)
	for range ticker.C {
		reconcile(payDB, ledgerDB)
	}
}

func reconcile(payDB, ledgerDB *sql.DB) {
	ctx := context.Background()

	// 1. Get all successful/refunded payments from last hour (to keep it small for now)
	rows, err := payDB.QueryContext(ctx, "SELECT id, status, amount FROM payment_intents WHERE status IN ('succeeded', 'refunded') AND created_at > NOW() - INTERVAL '1 hour'")
	if err != nil {
		log.Printf("Reconciler: Error fetching payments: %v", err)
		return
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("Reconciler: Failed to close rows: %v", err)
		}
	}()

	discrepancies := 0
	totalChecked := 0

	for rows.Next() {
		var id, status string
		var amount int64
		if err := rows.Scan(&id, &status, &amount); err != nil {
			log.Printf("Reconciler: Error scanning payment: %v", err)
			continue
		}

		totalChecked++

		// 2. Check Ledger for corresponding transaction
		refID := id
		if status == "refunded" {
			// For simplicity, let's just check if the refund transaction exists
			// This is a basic check.
			refID = "refund_" + id
		}

		var existingID string
		err := ledgerDB.QueryRowContext(ctx, "SELECT id FROM transactions WHERE reference_id = $1", refID).Scan(&existingID)
		if err == sql.ErrNoRows {
			log.Printf("❌ RECONCILIATION FAILURE: Missing transaction in Ledger for Payment %s (Status: %s)", id, status)
			discrepancies++
			continue
		} else if err != nil {
			log.Printf("Reconciler: Error checking Ledger: %v", err)
			continue
		}

		// Optional: Check entry sum for that transaction
		// ...
	}

	log.Printf("Reconciler audit complete. Checked: %d, Discrepancies: %d", totalChecked, discrepancies)
}
