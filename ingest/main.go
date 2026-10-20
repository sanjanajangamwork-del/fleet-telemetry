package main

import (
	"fmt"
	"os"
	"os/signal"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

func main() {
	opts := mqtt.NewClientOptions()
	opts.AddBroker("tcp://localhost:1883")
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
		fmt.Printf("topic: %s\npayload: %s\n\n", m.Topic(), m.Payload())
	}
	sub := client.Subscribe("fleet/+/+/telemtry", 0, handler)
	sub.Wait()
	fmt.Println("subscribed, waiting for messages (Ctrl+C to stop)")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	<-stop
	client.Disconnect(250)
	fmt.Println("bye")
}
