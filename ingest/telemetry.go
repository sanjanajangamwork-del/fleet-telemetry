package main

import (
	"encoding/json"
	"fmt"
	"time"
)

type Telemetry struct {
	DeviceID       string    `json:"deviceId"`
	Ts             time.Time `json:"ts"`
	State          string    `json:"state"`
	SpeedKph       float64   `json:"speedKph"`
	BatteryPct     float64   `json:"batteryPct"`
	OdometerMeters int64     `json:"odometerMeters"`
	RouteID        string    `json:"routeId"`
	Faults         []string  `json:"faults"`
}

func parseTelemetry(data []byte) (Telemetry, error) {
	var t Telemetry
	if err := json.Unmarshal(data, &t); err != nil {
		return Telemetry{}, fmt.Errorf("bad json: %w", err)
	}
	if t.DeviceID == "" {
		return Telemetry{}, fmt.Errorf("missing deviceId")
	}
	switch t.State {
	case "MOVING", "IDLE", "CHARGING", "OFF":
	default:
		return Telemetry{}, fmt.Errorf("unknown state %q", t.State)
	}
	return t, nil
}
