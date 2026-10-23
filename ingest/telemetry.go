package main

import (
	"encoding/json"
	"fmt"
	"sync"
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

	if t.Ts.IsZero() {
		return Telemetry{}, fmt.Errorf("missing or invalid timestamp")
	}

	if t.SpeedKph < 0 || t.BatteryPct < 0 || t.BatteryPct > 100 {
		return Telemetry{}, fmt.Errorf("invalid speed or battery percentage")
	}

	if t.OdometerMeters < 0 {
		return Telemetry{}, fmt.Errorf("negative odometer")
	}

	if t.State != "MOVING" && t.SpeedKph != 0 {
		return Telemetry{}, fmt.Errorf("speed must be zero when state is %s", t.State)
	}

	return t, nil
}

type DeviceTracker struct {
	mu        sync.RWMutex
	latest    map[string]Telemetry
	seen      map[string]time.Time
	distanceM map[string]int64
	resets    map[string]int
}

func NewDeviceTracker() *DeviceTracker {
	return &DeviceTracker{
		latest:    make(map[string]Telemetry),
		seen:      make(map[string]time.Time),
		distanceM: make(map[string]int64),
		resets:    make(map[string]int),
	}
}

// Process accepts a new reading unless it is a duplicate or older
// than the latest reading for that device.
func (d *DeviceTracker) Process(t Telemetry) (bool, string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	previous, exists := d.latest[t.DeviceID]
	if exists {
		if t.Ts.Equal(previous.Ts) {
			return false, "duplicate or same-timestamp message"
		}

		if t.Ts.Before(previous.Ts) {
			return false, "out-of-order message"
		}
	}
	if exists {
		if t.OdometerMeters < previous.OdometerMeters {
			d.resets[t.DeviceID]++
		} else {
			d.distanceM[t.DeviceID] += t.OdometerMeters - previous.OdometerMeters
		}
	}
	d.latest[t.DeviceID] = t
	d.seen[t.DeviceID] = time.Now()

	return true, "accepted"
}

func (d *DeviceTracker) Get(deviceID string) (Telemetry, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	t, ok := d.latest[deviceID]
	return t, ok
}
func (d *DeviceTracker) Stats(deviceID string) (int64, int) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.distanceM[deviceID], d.resets[deviceID]
}
