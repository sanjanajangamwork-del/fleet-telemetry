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
