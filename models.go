package main

import "time"

// AlertType distinguishes horizontal vs channel alerts.
type AlertType string

const (
	AlertHorizontal AlertType = "horizontal"
	AlertChannel    AlertType = "channel"
)

// HorizontalCondition is above or below target price.
type HorizontalCondition string

const (
	ConditionAbove HorizontalCondition = "above"
	ConditionBelow HorizontalCondition = "below"
)

// ChannelSide is which boundary to watch.
type ChannelSide string

const (
	SideUpper ChannelSide = "upper"
	SideLower ChannelSide = "lower"
	SideBoth  ChannelSide = "both"
)

// Alert is stored in alerts.json.
type Alert struct {
	ID          string    `json:"id"`
	Pair        string    `json:"pair"`
	Type        AlertType `json:"type"`
	Enabled     bool      `json:"enabled"`
	Label       string    `json:"label"`
	CreatedAt   time.Time `json:"created_at"`
	LastTrigger time.Time `json:"last_trigger,omitempty"`

	// Horizontal fields
	TargetPrice float64             `json:"target_price,omitempty"`
	Condition   HorizontalCondition `json:"condition,omitempty"`

	// Channel fields (timestamps stored as unix seconds)
	P1Time      int64       `json:"p1_time,omitempty"`
	P1Price     float64     `json:"p1_price,omitempty"`
	P2Time      int64       `json:"p2_time,omitempty"`
	P2Price     float64     `json:"p2_price,omitempty"`
	Offset      float64     `json:"offset,omitempty"`
	TriggerSide ChannelSide `json:"trigger_side,omitempty"`
}

// AlertView is returned by the API with computed channel fields.
type AlertView struct {
	Alert
	P1DateTime   string   `json:"p1_datetime,omitempty"`
	P2DateTime   string   `json:"p2_datetime,omitempty"`
	CurrentUpper *float64 `json:"current_upper,omitempty"`
	CurrentLower *float64 `json:"current_lower,omitempty"`
}

// AlertInput is used for create/update API requests.
type AlertInput struct {
	Pair        string              `json:"pair"`
	Type        AlertType           `json:"type"`
	Label       string              `json:"label"`
	TargetPrice float64             `json:"target_price"`
	Condition   HorizontalCondition `json:"condition"`

	// Channel: datetime strings from UI (YYYY-MM-DDTHH:MM) or unix from API
	P1DateTime string  `json:"p1_datetime"`
	P1Price    float64 `json:"p1_price"`
	P2DateTime string  `json:"p2_datetime"`
	P2Price    float64 `json:"p2_price"`
	Offset     float64 `json:"offset"`
	TriggerSide ChannelSide `json:"trigger_side"`
}
