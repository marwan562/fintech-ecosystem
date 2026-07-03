package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// SIEMExporter exports audit logs to external SIEM systems.
type SIEMExporter interface {
	Export(ctx context.Context, logs []SignedAuditLog) error
}

// HTTPWebhookExporter exports logs via HTTP POST (e.g., to Splunk HEC).
type HTTPWebhookExporter struct {
	url        string
	token      string
	httpClient *http.Client
}

// NewHTTPWebhookExporter creates a new HTTP webhook exporter.
func NewHTTPWebhookExporter(url, token string) *HTTPWebhookExporter {
	return &HTTPWebhookExporter{
		url:   url,
		token: token,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Export sends logs to the configured HTTP endpoint.
func (e *HTTPWebhookExporter) Export(ctx context.Context, logs []SignedAuditLog) error {
	payload, err := json.Marshal(logs)
	if err != nil {
		return fmt.Errorf("failed to marshal logs: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", e.url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if e.token != "" {
		req.Header.Set("Authorization", "Splunk "+e.token)
	}

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("export request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("export failed with status %d", resp.StatusCode)
	}

	return nil
}

// DataDogExporter exports logs to DataDog.
type DataDogExporter struct {
	apiKey     string
	httpClient *http.Client
	site       string // e.g., "datadoghq.com" or "datadoghq.eu"
}

// NewDataDogExporter creates a new DataDog exporter.
func NewDataDogExporter(apiKey, site string) *DataDogExporter {
	if site == "" {
		site = "datadoghq.com"
	}
	return &DataDogExporter{
		apiKey: apiKey,
		site:   site,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Export sends logs to DataDog Logs API.
func (e *DataDogExporter) Export(ctx context.Context, logs []SignedAuditLog) error {
	url := fmt.Sprintf("https://http-intake.logs.%s/api/v2/logs", e.site)

	// Transform logs to DataDog format
	ddLogs := make([]map[string]interface{}, len(logs))
	for i, log := range logs {
		ddLogs[i] = map[string]interface{}{
			"ddsource":  "sapliy",
			"service":   "sapliy-core",
			"hostname":  log.ZoneID,
			"message":   log.Action,
			"status":    "info",
			"timestamp": log.Timestamp.UnixMilli(),
			"audit":     log,
		}
	}

	payload, err := json.Marshal(ddLogs)
	if err != nil {
		return fmt.Errorf("failed to marshal dd logs: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create dd request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("DD-API-KEY", e.apiKey)

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("dd request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("dd export failed with status %d", resp.StatusCode)
	}

	return nil
}

// SIEMManager manages background export of audit logs.
type SIEMManager struct {
	exporter  SIEMExporter
	logBuffer []SignedAuditLog
	batchSize int
	interval  time.Duration
	stopChan  chan struct{}
}

// NewSIEMManager creates a new SIEM manager.
func NewSIEMManager(exporter SIEMExporter) *SIEMManager {
	return &SIEMManager{
		exporter:  exporter,
		batchSize: 100,
		interval:  5 * time.Second,
		stopChan:  make(chan struct{}),
	}
}

// Enqueue adds a log to the export buffer.
func (m *SIEMManager) Enqueue(log SignedAuditLog) {
	m.logBuffer = append(m.logBuffer, log)
	if len(m.logBuffer) >= m.batchSize {
		m.flush()
	}
}

// Start starts the background flush loop.
func (m *SIEMManager) Start(ctx context.Context) {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			m.flush()
			return
		case <-m.stopChan:
			m.flush()
			return
		case <-ticker.C:
			m.flush()
		}
	}
}

func (m *SIEMManager) flush() {
	if len(m.logBuffer) == 0 {
		return
	}

	// Create a copy to export
	logs := make([]SignedAuditLog, len(m.logBuffer))
	copy(logs, m.logBuffer)

	// Clear buffer immediately
	m.logBuffer = m.logBuffer[:0]

	// Async export
	go func() {
		// Use background context as main ctx might be cancelled
		if err := m.exporter.Export(context.Background(), logs); err != nil {
			fmt.Printf("SIEM export failed: %v\n", err)
			// In production: retry logic or backup to disk
		}
	}()
}
