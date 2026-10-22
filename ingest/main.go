package main

import (
	"fmt"
	"os"
	"os/signal"
    "time"
	mqtt "github.com/eclipse/paho.mqtt.golang"
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

	handler := func(c mqtt.Client, m mqtt.Message) {
    t, err := parseTelemetry(m.Payload())
    if err != nil {
        fmt.Println("skipping bad message:", err)
        return
    }
    fmt.Printf("%s %s %.1f km/h odo=%d ts=%s\n",
        t.DeviceID, t.State, t.SpeedKph, t.OdometerMeters,
        t.Ts.Format(time.RFC3339Nano))
} 
	sub := client.Subscribe("fleet/+/+/telemetry", 0, handler)
	sub.Wait()
	fmt.Println("subscribed, waiting for messages (Ctrl+C to stop)")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	<-stop
	client.Disconnect(250)
	fmt.Println("bye")
}
