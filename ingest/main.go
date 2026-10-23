package main

import (
	"fmt"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"os"
	"os/signal"
	"time"
)

func main() {
	opts := mqtt.NewClientOptions()
	broker := os.Getenv("MQTT_BROKER")
	if broker == "" {
		broker = "tcp://localhost:1883"
	}
	opts.AddBroker(broker)
	opts.SetClientID("ingest-1")

	client := mqtt.NewClient(opts)
	token := client.Connect()
	token.Wait()
	if token.Error() != nil {
		fmt.Println("connection failed:", token.Error())
		return
	}
	fmt.Println("connected to EMQX")

	tracker := NewDeviceTracker()
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				for _, id := range tracker.CheckOffline(time.Now(), 15*time.Second) {
					fmt.Println(id, "is now OFFLINE")
				}
			case <-done:
				return
			}
		}
	}()
	handler := func(c mqtt.Client, m mqtt.Message) {
		t, err := parseTelemetry(m.Payload())
		if err != nil {
			fmt.Println("skipping bad message:", err)
			return
		}

		accepted, reason := tracker.Process(t)
		if !accepted {
			fmt.Println("skipping message:", reason)
			return
		}

		fmt.Printf("%s %s %.1f km/h odo=%d ts=%s\n",
			t.DeviceID, t.State, t.SpeedKph, t.OdometerMeters,
			t.Ts.Format(time.RFC3339Nano))
		w := tracker.Window(t.DeviceID)
		fmt.Printf("  window: avgSpeed=%.1f avgBatt=%.1f moving=%.0f%% dist=%dm n=%d\n",
			w.AvgSpeed, w.AvgBattery, w.MovingPct, w.DistanceM, w.Samples)
	}

	sub := client.Subscribe("fleet/+/+/telemetry", 0, handler)
	sub.Wait()
	fmt.Println("subscribed, waiting for messages (Ctrl+C to stop)")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	<-stop
	client.Disconnect(250)
	close(done)
	fmt.Println("bye")
}
