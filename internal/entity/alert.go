package ent

import (
	"time"
)

const (
	MaxMsgLength  = 4096
	FiringEmoji   = "❗️"
	ResolvedEmoji = "✅"
)

type Alert struct {
	Status      string
	Labels      AlertLabels
	Annotations AlertAnnotations
	StartsAt    time.Time
	EndsAt      time.Time
}

type AlertLabels struct {
	AlertName    string
	Severity     string
	ErrorMessage string
	StrategyName string
	AlertGroup   string
	Name         string
}

type AlertAnnotations struct {
	Summary     string
	Description string
}

type AlertManagerMessage struct {
	Alerts []Alert
}
