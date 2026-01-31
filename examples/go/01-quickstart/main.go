package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	fintech "github.com/marwan562/fintech-ecosystem/sdks/go"
)

func main() {
	fmt.Println("🚀 Starting Sapliy Fintech Quickstart (Go)...")

	apiKey := os.Getenv("API_KEY")
	if apiKey == "" {
		apiKey = "sk_test_123"
	}

	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	// Initialize the client
	client := fintech.NewClient(apiKey, fintech.WithBaseURL(baseURL))

	ctx := context.Background()

	// 1. Create a Ledger Account
	// Simulating a unique account ID
	accountID := fmt.Sprintf("acc_%d", time.Now().Unix())
	fmt.Printf("\n1. Creating Ledger Account: %s\n", accountID)

	// Note: In a real flow, you'd use client.Payments.CreateIntent(...)
	// For now, we simulate using the ledger directly.

	fmt.Println("   (Simulating payment confirmation via Ledger Transaction)")
	description := "Quickstart Payment"
	refID := fmt.Sprintf("ref_%d", time.Now().Unix())

	txReq := &fintech.RecordTransactionRequest{
		AccountID:   accountID,
		Amount:      1000, // $10.00
		Currency:    "USD",
		Description: description,
		ReferenceID: refID,
	}

	tx, err := client.Ledger.RecordTransaction(ctx, txReq)
	if err != nil {
		log.Fatalf("❌ Failed to record transaction: %v", err)
	}

	fmt.Printf("✅ Transaction Recorded: %+v\n", tx)

	// 2. Check Balance
	fmt.Printf("\n2. Checking Balance for Account: %s\n", accountID)
	// Small delay for consistency if running against a real async system
	time.Sleep(1 * time.Second)

	account, err := client.Ledger.GetAccount(ctx, accountID)
	if err != nil {
		log.Fatalf("❌ Failed to get account: %v", err)
	}

	fmt.Printf("💰 Account Details: %+v\n", account)
	fmt.Printf("   Balance: %.2f %s\n", float64(account.Balance)/100.0, account.Currency)
}
