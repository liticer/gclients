package alertmanager

import "time"

// SilenceState represents the state of a silence.
type SilenceState string

const (
	SilenceStateExpired SilenceState = "expired"
	SilenceStateActive  SilenceState = "active"
	SilenceStatePending SilenceState = "pending"
)

// SilenceStatus stores the state of a silence.
type SilenceStatus struct {
	State SilenceState `json:"state"`
}

// SilenceMatcher represents a matcher for a silence rule.
type SilenceMatcher struct {
	Name    string `json:"name"`
	Value   string `json:"value"`
	IsRegex bool   `json:"isRegex"`
	IsEqual bool   `json:"isEqual"`
}

// Silence represents a silence entry in Alertmanager.
type Silence struct {
	ID        string            `json:"id"`
	Matchers  []*SilenceMatcher `json:"matchers"`
	StartsAt  time.Time         `json:"startsAt"`
	EndsAt    time.Time         `json:"endsAt"`
	UpdatedAt time.Time         `json:"updatedAt"`
	CreatedBy string            `json:"createdBy"`
	Comment   string            `json:"comment,omitempty"`
	Status    SilenceStatus     `json:"status"`
}

// AlertState represents the state of an alert.
type AlertState string

// AlertStatus stores the state and silencing/inhibiting info of an alert.
type AlertStatus struct {
	State       AlertState `json:"state"`
	SilencedBy  []string   `json:"silencedBy"`
	InhibitedBy []string   `json:"inhibitedBy"`
}
