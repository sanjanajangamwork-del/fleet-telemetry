package main

import (
	"encoding/json"
	"fmt"
	"sort"
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

type sample struct {
	ts      time.Time
	speed   float64
	battery float64
	moving  bool
	distM   int64
}

type WindowStats struct {
	AvgSpeed   float64
	AvgBattery float64
	MovingPct  float64
	DistanceM  int64
	Samples    int
}
type DeviceView struct {
	DeviceID  string      `json:"deviceId"`
	Online    bool        `json:"online"`
	LastSeen  time.Time   `json:"lastSeen"`
	Latest    Telemetry   `json:"latest"`
	DistanceM int64       `json:"totalDistanceM"`
	Resets    int         `json:"resets"`
	Window    WindowStats `json:"window60s"`
}
type DeviceTracker struct {
	mu        sync.RWMutex
	latest    map[string]Telemetry
	seen      map[string]time.Time
	distanceM map[string]int64
	resets    map[string]int
	online    map[string]bool
	windows   map[string][]sample
}

func NewDeviceTracker() *DeviceTracker {
	return &DeviceTracker{
		latest:    make(map[string]Telemetry),
		seen:      make(map[string]time.Time),
		distanceM: make(map[string]int64),
		resets:    make(map[string]int),
		online:    make(map[string]bool),
		windows:   make(map[string][]sample),
	}
}

// Process accepts a new reading unless it is a duplicate or older
// than the latest reading for that device.
func (d *DeviceTracker) Process(t Telemetry) (bool, string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.seen[t.DeviceID] = time.Now()
	d.online[t.DeviceID] = true

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
	var dist int64
	if exists && t.OdometerMeters >= previous.OdometerMeters {
		dist = t.OdometerMeters - previous.OdometerMeters
	}
	w := append(d.windows[t.DeviceID], sample{
		ts: t.Ts, speed: t.SpeedKph, battery: t.BatteryPct,
		moving: t.State == "MOVING", distM: dist,
	})
	cutoff := t.Ts.Add(-60 * time.Second)
	i := 0
	for i < len(w) && w[i].ts.Before(cutoff) {
		i++
	}
	d.windows[t.DeviceID] = w[i:]

	d.latest[t.DeviceID] = t

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

// CheckOffline marks devices silent for longer than timeout as offline
// and returns the IDs that just changed.
func (d *DeviceTracker) CheckOffline(now time.Time, timeout time.Duration) []string {
	d.mu.Lock()
	defer d.mu.Unlock()

	var changed []string
	for id, last := range d.seen {
		if d.online[id] && now.Sub(last) > timeout {
			d.online[id] = false
			changed = append(changed, id)
		}
	}
	return changed
}

func (d *DeviceTracker) IsOnline(deviceID string) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.online[deviceID]
}
func (d *DeviceTracker) Window(deviceID string) WindowStats {
	d.mu.RLock()
	defer d.mu.RUnlock()

	w := d.windows[deviceID]
	var s WindowStats
	if len(w) == 0 {
		return s
	}
	var sumSpeed, sumBat float64
	var movingDur, total time.Duration
	for i, x := range w {
		sumSpeed += x.speed
		sumBat += x.battery
		if i > 0 {
			s.DistanceM += x.distM
			dt := x.ts.Sub(w[i-1].ts)
			total += dt
			if w[i-1].moving {
				movingDur += dt
			}
		}
	}
	n := float64(len(w))
	s.AvgSpeed = sumSpeed / n
	s.AvgBattery = sumBat / n
	s.Samples = len(w)
	if total > 0 {
		s.MovingPct = 100 * float64(movingDur) / float64(total)
	}
	return s
}
func (d *DeviceTracker) View(id string) (DeviceView, bool) {
	d.mu.RLock()
	latest, ok := d.latest[id]
	if !ok {
		d.mu.RUnlock()
		return DeviceView{}, false
	}
	v := DeviceView{
		DeviceID:  id,
		Online:    d.online[id],
		LastSeen:  d.seen[id],
		Latest:    latest,
		DistanceM: d.distanceM[id],
		Resets:    d.resets[id],
	}
	d.mu.RUnlock()

	v.Window = d.Window(id)
	return v, true
}

func (d *DeviceTracker) IDs() []string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	ids := make([]string, 0, len(d.latest))
	for id := range d.latest {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
