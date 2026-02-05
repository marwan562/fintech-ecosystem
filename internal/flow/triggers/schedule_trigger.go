package triggers

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// ScheduleTrigger triggers flows based on time intervals
type ScheduleTrigger struct {
	Interval time.Duration `json:"interval"`
	CronExpr string        `json:"cronExpression,omitempty"` // For documentation only
	Timezone string        `json:"timezone,omitempty"`
	FlowID   string        `json:"flowId"`
	ZoneID   string        `json:"zoneId"`
	LastRun  time.Time     `json:"lastRun,omitempty"`
	NextRun  time.Time     `json:"nextRun,omitempty"`
	location *time.Location
}

// NewScheduleTrigger creates a new schedule trigger
func NewScheduleTrigger(interval time.Duration, timezone, flowID, zoneID string) (*ScheduleTrigger, error) {
	loc := time.UTC
	var err error
	if timezone != "" {
		loc, err = time.LoadLocation(timezone)
		if err != nil {
			return nil, fmt.Errorf("invalid timezone: %w", err)
		}
	}

	t := &ScheduleTrigger{
		Interval: interval,
		Timezone: timezone,
		FlowID:   flowID,
		ZoneID:   zoneID,
		location: loc,
	}
	t.NextRun = time.Now().In(loc).Add(interval)

	return t, nil
}

// NewScheduleTriggerFromCron creates a trigger from a cron expression (simplified)
func NewScheduleTriggerFromCron(cronExpr, timezone, flowID, zoneID string) (*ScheduleTrigger, error) {
	interval := parseCronToInterval(cronExpr)
	trigger, err := NewScheduleTrigger(interval, timezone, flowID, zoneID)
	if err != nil {
		return nil, err
	}
	trigger.CronExpr = cronExpr
	return trigger, nil
}

// parseCronToInterval converts simple cron expressions to intervals
func parseCronToInterval(cron string) time.Duration {
	cron = strings.TrimSpace(cron)
	switch {
	case strings.Contains(cron, "*/1 ") || cron == "* * * * *":
		return time.Minute
	case strings.HasPrefix(cron, "*/5"):
		return 5 * time.Minute
	case strings.HasPrefix(cron, "*/15"):
		return 15 * time.Minute
	case cron == "0 * * * *":
		return time.Hour
	case cron == "0 9 * * *", cron == "0 0 * * *":
		return 24 * time.Hour
	case strings.HasSuffix(cron, "* * 1"):
		return 7 * 24 * time.Hour // Weekly
	default:
		return time.Hour // Default to hourly
	}
}

// Type returns the trigger type
func (t *ScheduleTrigger) Type() TriggerType {
	return TriggerSchedule
}

// ShouldTrigger checks if it's time to trigger based on the schedule
func (t *ScheduleTrigger) ShouldTrigger(ctx context.Context, input interface{}) (bool, error) {
	now, ok := input.(time.Time)
	if !ok {
		now = time.Now()
	}

	now = now.In(t.location)

	// Check if we've passed the next run time
	if now.After(t.NextRun) || now.Equal(t.NextRun) {
		return true, nil
	}

	return false, nil
}

// GetConfig returns the trigger configuration
func (t *ScheduleTrigger) GetConfig() interface{} {
	return t
}

// UpdateAfterRun updates the trigger state after a successful run
func (t *ScheduleTrigger) UpdateAfterRun() {
	t.LastRun = time.Now().In(t.location)
	t.NextRun = t.LastRun.Add(t.Interval)
}

// GetNextRunTime returns the next scheduled run time
func (t *ScheduleTrigger) GetNextRunTime() time.Time {
	return t.NextRun
}

// ScheduleTriggerService manages scheduled triggers
type ScheduleTriggerService struct {
	triggers map[string]*ScheduleTrigger // flowID -> trigger
	handler  func(ctx context.Context, trigger *ScheduleTrigger) error
	stopCh   chan struct{}
	mu       sync.RWMutex
	running  bool
}

// NewScheduleTriggerService creates a new schedule trigger service
func NewScheduleTriggerService() *ScheduleTriggerService {
	return &ScheduleTriggerService{
		triggers: make(map[string]*ScheduleTrigger),
		stopCh:   make(chan struct{}),
	}
}

// SetHandler sets the handler function for triggered schedules
func (s *ScheduleTriggerService) SetHandler(handler func(ctx context.Context, trigger *ScheduleTrigger) error) {
	s.handler = handler
}

// Register adds a schedule trigger
func (s *ScheduleTriggerService) Register(trigger *ScheduleTrigger) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.triggers[trigger.FlowID] = trigger
	return nil
}

// Unregister removes a schedule trigger
func (s *ScheduleTriggerService) Unregister(flowID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.triggers, flowID)
}

// Start begins the scheduler loop
func (s *ScheduleTriggerService) Start() {
	s.running = true
	go s.loop()
}

// loop is the main scheduler loop
func (s *ScheduleTriggerService) loop() {
	ticker := time.NewTicker(time.Second * 10) // Check every 10 seconds
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case now := <-ticker.C:
			s.checkTriggers(now)
		}
	}
}

// checkTriggers checks all triggers and fires any that are due
func (s *ScheduleTriggerService) checkTriggers(now time.Time) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, trigger := range s.triggers {
		shouldFire, _ := trigger.ShouldTrigger(context.Background(), now)
		if shouldFire && s.handler != nil {
			go func(t *ScheduleTrigger) {
				ctx := context.Background()
				if err := s.handler(ctx, t); err != nil {
					fmt.Printf("Schedule trigger error for flow %s: %v\n", t.FlowID, err)
				}
				t.UpdateAfterRun()
			}(trigger)
		}
	}
}

// Stop halts the scheduler
func (s *ScheduleTriggerService) Stop() {
	if s.running {
		close(s.stopCh)
		s.running = false
	}
}

// GetAllTriggers returns all registered triggers
func (s *ScheduleTriggerService) GetAllTriggers() []*ScheduleTrigger {
	s.mu.RLock()
	defer s.mu.RUnlock()

	triggers := make([]*ScheduleTrigger, 0, len(s.triggers))
	for _, t := range s.triggers {
		triggers = append(triggers, t)
	}
	return triggers
}

// Common schedule presets
var SchedulePresets = map[string]string{
	"every-minute":     "* * * * *",
	"every-5-minutes":  "*/5 * * * *",
	"every-15-minutes": "*/15 * * * *",
	"every-hour":       "0 * * * *",
	"daily-9am":        "0 9 * * *",
	"daily-midnight":   "0 0 * * *",
	"weekly-monday":    "0 9 * * 1",
	"monthly-first":    "0 9 1 * *",
}
