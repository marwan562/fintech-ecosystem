package main

import (
	"context"
	"encoding/json"
	"log"

	"github.com/marwan562/fintech-ecosystem/internal/ledger"
	"github.com/marwan562/fintech-ecosystem/pkg/messaging"
)

type PaymentEvent struct {
	Type string `json:"type"`
	Data struct {
		ID       string `json:"id"`
		Amount   int64  `json:"amount"`
		Currency string `json:"currency"`
		UserID   string `json:"user_id"`
	} `json:"data"`
}

func StartKafkaConsumer(brokers []string, repo *ledger.Repository) {
	consumer := messaging.NewKafkaConsumer(brokers, "payments", "ledger-group")

	log.Println("Ledger Kafka Consumer started on topic 'payments'")

	consumer.Consume(context.Background(), func(key string, value []byte) error {
		var event PaymentEvent
		if err := json.Unmarshal(value, &event); err != nil {
			return err
		}

		if event.Type != "payment.succeeded" {
			return nil // Only process successful payments
		}

		log.Printf("Ledger: Processing payment event for ID %s", event.Data.ID)

		// Create transactional entries
		txReq := ledger.TransactionRequest{
			ReferenceID: event.Data.ID,
			Description: "Kafka Event: Professional Payment Success",
			Entries: []ledger.EntryRequest{
				{
					AccountID: "user_" + event.Data.UserID,
					Amount:    event.Data.Amount,
					Direction: "credit",
				},
				{
					AccountID: "system_balancing",
					Amount:    -event.Data.Amount,
					Direction: "debit",
				},
			},
		}

		ctx := context.Background()
		if err := repo.RecordTransaction(ctx, txReq); err != nil {
			log.Printf("Failed to record transaction from Kafka event: %v", err)
			return err
		}

		log.Printf("Ledger: Successfully recorded transaction for payment %s", event.Data.ID)
		return nil
	})
}
