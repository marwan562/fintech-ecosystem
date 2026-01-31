package main

import (
	"context"
	"encoding/json"
	"log"

	"github.com/marwan562/fintech-ecosystem/internal/ledger/domain"
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

func StartKafkaConsumer(brokers []string, service *domain.LedgerService) {
	consumer := messaging.NewKafkaConsumer(brokers, "payments", "ledger-group")

	log.Println("Ledger Kafka Consumer started on topic 'payments'")

	consumer.Consume(context.Background(), func(key string, value []byte) error {
		var event PaymentEvent
		if err := json.Unmarshal(value, &event); err != nil {
			return err
		}

		log.Printf("Ledger: Received Kafka event type %s for ID %s", event.Type, event.Data.ID)

		var txReq domain.TransactionRequest

		switch event.Type {
		case "payment.succeeded":
			txReq = domain.TransactionRequest{
				ReferenceID: event.Data.ID,
				Description: "Kafka Event: Payment Success",
				Entries: []domain.EntryRequest{
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
		case "payment.refunded":
			// Reversing entries
			txReq = domain.TransactionRequest{
				ReferenceID: "refund_" + event.Data.ID,
				Description: "Kafka Event: Payment Refunded",
				Entries: []domain.EntryRequest{
					{
						AccountID: "user_" + event.Data.UserID,
						Amount:    -event.Data.Amount, // Negative credit is a debit
						Direction: "debit",            // Explicitly set direction
					},
					{
						AccountID: "system_balancing",
						Amount:    event.Data.Amount,
						Direction: "credit",
					},
				},
			}
		default:
			return nil // Ignore other events
		}

		ctx := context.Background()
		if err := service.RecordTransaction(ctx, txReq); err != nil {
			log.Printf("Failed to record transaction for event %s (ID: %s): %v", event.Type, event.Data.ID, err)
			return err
		}

		log.Printf("Ledger: Successfully recorded transaction for event %s (ID: %s)", event.Type, event.Data.ID)
		return nil
	})
}
