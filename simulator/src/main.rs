use chrono::{SecondsFormat, Utc};
use rumqttc::{Client, MqttOptions, QoS};
use serde_json::json;
use std::{env, thread, time::Duration};

fn main() {
    let host = env::var("MQTT_HOST").unwrap_or_else(|_| "localhost".to_string());
    let port: u16 = env::var("MQTT_PORT")
        .ok()
        .and_then(|p| p.parse().ok())
        .unwrap_or(1883);

    let mut opts = MqttOptions::new("simulator-1", host, port);
    opts.set_keep_alive(Duration::from_secs(5));
    let (client, mut connection) = Client::new(opts, 10);

    thread::spawn(move || {
        let mut odometer: f64 = 184_920.0;
        let mut battery: f64 = 80.0;
        let mut tick: u64 = 0;
        loop {
            let moving = (tick / 50) % 2 == 0;
            let speed = if moving { 40.0 } else { 0.0 };
            let state = if moving { "MOVING" } else { "IDLE" };

            if moving {
                odometer += speed / 3.6 * 0.2;
                battery -= 0.01;
            }

            let payload = json!({
                "deviceId": "VEH-1042",
                "ts": Utc::now().to_rfc3339_opts(SecondsFormat::Millis, true),
                "state": state,
                "speedKph": speed,
                "batteryPct": battery,
                "odometerMeters": odometer as i64,
                "routeId": "R-118",
                "faults": []
            });

            let topic = "fleet/west-01/VEH-1042/telemetry";
            if let Err(e) = client.publish(topic, QoS::AtMostOnce, false, payload.to_string()) {
                eprintln!("publish failed: {e}");
            }

            tick += 1;
            thread::sleep(Duration::from_millis(200));
        }
    });

    for event in connection.iter() {
        if let Err(e) = event {
            eprintln!("connection error: {e}");
            thread::sleep(Duration::from_secs(1));
        }
    }
}