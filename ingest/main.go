package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

const topicFilter = "fleet/+/+/telemetry"

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	broker := getenv("MQTT_BROKER", "tcp://localhost:1883")
	addr := getenv("HTTP_ADDR", ":8080")
	clientID := getenv("CLIENT_ID", "ingest-1")

	tracker := NewDeviceTracker()
	counters := &Counters{}
	hub := NewHub(counters)

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
			return
		}
		counters.Accepted.Add(1)
		hub.Publish(t.DeviceID, tracker)
	}

	opts := mqtt.NewClientOptions()
	opts.AddBroker(broker)
	opts.SetClientID(clientID)
	opts.SetCleanSession(true)
	opts.SetConnectRetry(true)
	opts.SetConnectRetryInterval(2 * time.Second)
	opts.SetAutoReconnect(true)
	opts.SetMaxReconnectInterval(30 * time.Second)
	opts.SetOnConnectHandler(func(c mqtt.Client) {
		fmt.Println("connected to EMQX, subscribing to", topicFilter)
		tok := c.Subscribe(topicFilter, 0, handler)
		tok.Wait()
		if tok.Error() != nil {
			fmt.Println("subscribe failed:", tok.Error())
		}
	})
	opts.SetConnectionLostHandler(func(c mqtt.Client, err error) {
		counters.Reconnects.Add(1)
		fmt.Println("connection lost:", err)
	})

	client := mqtt.NewClient(opts)
	client.Connect() // keeps retrying in the background until EMQX is up

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

	srv := &http.Server{Addr: addr, Handler: newMux(tracker, counters, hub)}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Println("http server error:", err)
		}
	}()
	fmt.Println("http api on", addr)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	fmt.Println("shutting down")

	close(done)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	srv.Shutdown(ctx)
	cancel()
	client.Disconnect(250)
	fmt.Println("bye")
}
