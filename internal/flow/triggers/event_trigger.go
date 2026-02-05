package triggers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// TriggerType represents the type of flow trigger
type TriggerType string

const (
	TriggerEvent    TriggerType = "event"
	TriggerSchedule TriggerType = "schedule"
	TriggerWebhook  TriggerType = "webhook"
)

// Trigger is the interface for all trigger types
type Trigger interface {
	Type() TriggerType
	ShouldTrigger(ctx context.Context, input interface{}) (bool, error)
	GetConfig() interface{}
}

// EventTrigger triggers flows based on event types
type EventTrigger struct {
	EventType string            `json:"eventType"`
	Filters   map[string]string `json:"filters,omitempty"`
	ZoneID    string            `json:"zoneId"`
	FlowID    string            `json:"flowId"`
}

// NewEventTrigger creates a new event trigger
func NewEventTrigger(eventType, zoneID, flowID string) *EventTrigger {
	return &EventTrigger{
		EventType: eventType,
		ZoneID:    zoneID,
		FlowID:    flowID,
		Filters:   make(map[string]string),
	}
}

// Type returns the trigger type
func (t *EventTrigger) Type() TriggerType {
	return TriggerEvent
}

// ShouldTrigger checks if the event matches this trigger
func (t *EventTrigger) ShouldTrigger(ctx context.Context, input interface{}) (bool, error) {
	event, ok := input.(*Event)
	if !ok {
		return false, fmt.Errorf("expected *Event, got %T", input)
	}

	// Check event type match
	if event.Type != t.EventType {
		// Support wildcard matching (e.g., "payment.*")
		if !matchEventType(t.EventType, event.Type) {
			return false, nil
		}
	}

	// Check zone match
	if t.ZoneID != "" && event.ZoneID != t.ZoneID {
		return false, nil
	}

	// Apply filters if any
	for key, expectedValue := range t.Filters {
		actualValue, err := extractJSONPath(event.Data, key)
		if err != nil || actualValue != expectedValue {
			return false, nil
		}
	}

	return true, nil
}

// GetConfig returns the trigger configuration
func (t *EventTrigger) GetConfig() interface{} {
	return t
}

// Event represents an incoming event
type Event struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	ZoneID    string                 `json:"zoneId"`
	Data      map[string]interface{} `json:"data"`
	CreatedAt time.Time              `json:"createdAt"`
}

// matchEventType checks if a pattern matches an event type
// Supports wildcards like "payment.*" matching "payment.succeeded"
func matchEventType(pattern, eventType string) bool {
	if pattern == "*" {
		return true
	}

	if len(pattern) > 0 && pattern[len(pattern)-1] == '*' {
		prefix := pattern[:len(pattern)-1]
		return len(eventType) >= len(prefix) && eventType[:len(prefix)] == prefix
	}

	return pattern == eventType
}

// extractJSONPath extracts a value from a map using dot notation
func extractJSONPath(data map[string]interface{}, path string) (string, error) {
	// Simple implementation - for complex paths, use a proper JSONPath library
	if val, ok := data[path]; ok {
		switch v := val.(type) {
		case string:
			return v, nil
		case float64:
			return fmt.Sprintf("%v", v), nil
		case bool:
			return fmt.Sprintf("%v", v), nil
		default:
			bytes, err := json.Marshal(v)
			if err != nil {
				return "", err
			}
			return string(bytes), nil
		}
	}
	return "", fmt.Errorf("path not found: %s", path)
}

// EventTriggerService manages event triggers
type EventTriggerService struct {
	triggers map[string][]*EventTrigger // eventType -> triggers
}

// NewEventTriggerService creates a new event trigger service
func NewEventTriggerService() *EventTriggerService {
	return &EventTriggerService{
		triggers: make(map[string][]*EventTrigger),
	}
}

// Register adds a trigger to the service
func (s *EventTriggerService) Register(trigger *EventTrigger) {
	s.triggers[trigger.EventType] = append(s.triggers[trigger.EventType], trigger)
}

// Unregister removes a trigger from the service
func (s *EventTriggerService) Unregister(flowID string) {
	for eventType, triggers := range s.triggers {
		filtered := make([]*EventTrigger, 0)
		for _, t := range triggers {
			if t.FlowID != flowID {
				filtered = append(filtered, t)
			}
		}
		s.triggers[eventType] = filtered
	}
}

// Match finds all triggers that match an event
func (s *EventTriggerService) Match(ctx context.Context, event *Event) ([]*EventTrigger, error) {
	var matched []*EventTrigger

	// Check exact match
	if triggers, ok := s.triggers[event.Type]; ok {
		for _, t := range triggers {
			shouldTrigger, err := t.ShouldTrigger(ctx, event)
			if err != nil {
				return nil, err
			}
			if shouldTrigger {
				matched = append(matched, t)
			}
		}
	}

	// Check wildcard triggers
	for eventType, triggers := range s.triggers {
		if eventType[len(eventType)-1] == '*' && matchEventType(eventType, event.Type) {
			for _, t := range triggers {
				shouldTrigger, err := t.ShouldTrigger(ctx, event)
				if err != nil {
					return nil, err
				}
				if shouldTrigger {
					matched = append(matched, t)
				}
			}
		}
	}

	return matched, nil
}
