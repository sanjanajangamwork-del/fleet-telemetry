package main

import (
	"testing"
	"time"
)

func msg(ts string, odo int64) Telemetry {
	tm, _ := time.Parse(time.RFC3339, ts)
	return Telemetry{DeviceID: "VEH-1", Ts: tm, State: "MOVING", OdometerMeters: odo}
}

func TestCounterReset(t *testing.T) {
	tr := NewDeviceTracker()
	tr.Process(msg("2026-10-23T10:00:00Z", 1000))
	tr.Process(msg("2026-10-23T10:00:01Z", 1010))
	tr.Process(msg("2026-10-23T10:00:02Z", 0))
	tr.Process(msg("2026-10-23T10:00:03Z", 5))

	dist, resets := tr.Stats("VEH-1")
	if dist != 15 {
		t.Errorf("distance = %d, want 15", dist)
	}
	if resets != 1 {
		t.Errorf("resets = %d, want 1", resets)
	}
}

func TestLateMessageDoesNotOverwriteLatest(t *testing.T) {
	tr := NewDeviceTracker()
	tr.Process(msg("2026-10-23T10:00:05Z", 1050))
	ok, _ := tr.Process(msg("2026-10-23T10:00:02Z", 1020))

	if ok {
		t.Error("late message was accepted")
	}
	got, _ := tr.Get("VEH-1")
	if got.OdometerMeters != 1050 {
		t.Errorf("latest odometer = %d, want 1050", got.OdometerMeters)
	}
}

func TestDuplicateIgnored(t *testing.T) {
	tr := NewDeviceTracker()
	tr.Process(msg("2026-10-23T10:00:00Z", 1000))
	tr.Process(msg("2026-10-23T10:00:01Z", 1010))
	ok, _ := tr.Process(msg("2026-10-23T10:00:01Z", 1010))

	dist, _ := tr.Stats("VEH-1")
	if ok || dist != 10 {
		t.Errorf("duplicate counted: ok=%v dist=%d, want false and 10", ok, dist)
	}
}
func TestOfflineAfterSilence(t *testing.T) {
	tr := NewDeviceTracker()
	tr.Process(msg("2026-10-23T10:00:00Z", 1000))
	now := time.Now()

	if got := tr.CheckOffline(now.Add(10*time.Second), 15*time.Second); len(got) != 0 {
		t.Errorf("went offline too early: %v", got)
	}
	if got := tr.CheckOffline(now.Add(20*time.Second), 15*time.Second); len(got) != 1 {
		t.Errorf("expected 1 device offline, got %v", got)
	}
	if got := tr.CheckOffline(now.Add(21*time.Second), 15*time.Second); len(got) != 0 {
		t.Errorf("same device reported twice: %v", got)
	}
	if tr.IsOnline("VEH-1") {
		t.Error("device should be offline")
	}

	tr.Process(msg("2026-10-23T10:00:30Z", 1010))
	if !tr.IsOnline("VEH-1") {
		t.Error("device should be back online after a message")
	}
}
func mk(ts, state string, speed float64, odo int64) Telemetry {
	tm, _ := time.Parse(time.RFC3339, ts)
	return Telemetry{DeviceID: "VEH-1", Ts: tm, State: state, SpeedKph: speed, BatteryPct: 50, OdometerMeters: odo}
}

func TestWindowStats(t *testing.T) {
	tr := NewDeviceTracker()
	tr.Process(mk("2026-10-23T10:00:00Z", "MOVING", 10, 1000))
	tr.Process(mk("2026-10-23T10:00:01Z", "MOVING", 20, 1010))
	tr.Process(mk("2026-10-23T10:00:02Z", "IDLE", 0, 1010))
	tr.Process(mk("2026-10-23T10:00:03Z", "IDLE", 0, 1010))

	w := tr.Window("VEH-1")
	if w.DistanceM != 10 {
		t.Errorf("distance = %d, want 10", w.DistanceM)
	}
	if w.AvgSpeed != 7.5 {
		t.Errorf("avg speed = %v, want 7.5", w.AvgSpeed)
	}
	if w.MovingPct < 66 || w.MovingPct > 67 {
		t.Errorf("moving%% = %v, want about 66.7", w.MovingPct)
	}
}

func TestWindowDropsOldSamples(t *testing.T) {
	tr := NewDeviceTracker()
	tr.Process(mk("2026-10-23T10:00:00Z", "MOVING", 10, 1000))
	tr.Process(mk("2026-10-23T10:02:00Z", "MOVING", 30, 1500))

	if w := tr.Window("VEH-1"); w.Samples != 1 {
		t.Errorf("samples = %d, want 1", w.Samples)
	}
}
func TestImplausibleJumpIsReset(t *testing.T) {
	tr := NewDeviceTracker()
	tr.Process(msg("2026-11-01T10:00:00Z", 1000))
	tr.Process(msg("2026-11-01T10:00:01Z", 1010))
	tr.Process(msg("2026-11-01T10:00:02Z", 187000))

	dist, resets := tr.Stats("VEH-1")
	if dist != 10 || resets != 1 {
		t.Errorf("dist=%d resets=%d, want 10 and 1", dist, resets)
	}
}
