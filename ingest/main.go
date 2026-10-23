package main

import (
	"context"
	"fmt"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"net/http"
	"os"
	"os/signal"
	"strings"
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
	counters := &Counters{}
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
		counters.Received.Add(1)
		t, err := parseTelemetry(m.Payload())
		if err != nil {
			counters.BadMessage.Add(1)
			fmt.Println("skipping bad message:", err)
			return
		}

		accepted, reason := tracker.Process(t)
		if !accepted {
			if strings.Contains(reason, "duplicate") {
				counters.Duplicates.Add(1)
			} else {
				counters.Late.Add(1)
			}
			fmt.Println("skipping message:", reason)
			return
		}
		counters.Accepted.Add(1)

		fmt.Printf("%s %s %.1f km/h odo=%d ts=%s\n",
			t.DeviceID, t.State, t.SpeedKph, t.OdometerMeters,
			t.Ts.Format(time.RFC3339Nano))
	}

	sub := client.Subscribe("fleet/+/+/telemetry", 0, handler)
	sub.Wait()
	fmt.Println("subscribed, waiting for messages (Ctrl+C to stop)")
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	srv := &http.Server{Addr: addr, Handler: newMux(tracker, counters)}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Println("http server error:", err)
		}
	}()
	fmt.Println("http api on", addr)
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	<-stop
	close(done)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	srv.Shutdown(ctx)
	cancel()
	client.Disconnect(250)

	fmt.Println("bye")
}
