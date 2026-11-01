use chrono::{SecondsFormat, Utc};
use rand::Rng;
use rumqttc::{Client, MqttOptions, QoS};
use serde_json::json;
use std::{
    env, thread,
    time::{Duration, Instant},
};

fn env_f64(name: &str, default: f64) -> f64 {
    env::var(name)
        .ok()
        .and_then(|v| v.parse().ok())
        .unwrap_or(default)
}

fn main() {
    let host = env::var("MQTT_HOST").unwrap_or_else(|_| "localhost".to_string());
    let port: u16 = env::var("MQTT_PORT")
        .ok()
        .and_then(|p| p.parse().ok())
        .unwrap_or(1883);
    let client_id = env::var("CLIENT_ID").unwrap_or_else(|_| "simulator-1".to_string());

    let dup_rate = env_f64("DUPLICATE_RATE", 0.01);
    let late_rate = env_f64("LATE_RATE", 0.02);
    let reset_rate = env_f64("RESET_RATE", 1.0 / 3000.0);
    let silent_rate = env_f64("SILENT_RATE", 1.0 / 3000.0);

    let mut opts = MqttOptions::new(client_id, host, port);
    opts.set_keep_alive(Duration::from_secs(5));
    let (client, mut connection) = Client::new(opts, 10);

    thread::spawn(move || {
        let mut rng = rand::thread_rng();
        let mut odometer: f64 = 184_920.0;
        let mut battery: f64 = 80.0;
        let mut tick: u64 = 0;
        let mut silent_until: Option<Instant> = None;
        let mut held: Vec<(Instant, String)> = Vec::new();
        let topic = "fleet/west-01/VEH-1042/telemetry";

        loop {
            let moving = (tick / 50) % 2 == 0;
            let speed = if moving { 40.0 } else { 0.0 };
            let state = if moving { "MOVING" } else { "IDLE" };

            if moving {
                odometer += speed / 3.6 * 0.2;
                battery -= 0.01;
            }

            // Fault: counter reset (tracker reboot)
            if rng.gen_bool(reset_rate) {
                odometer = 0.0;
                eprintln!("fault: counter reset");
            }

            let payload = json!({
                "deviceId": "VEH-1042",
                "ts": Utc::now().to_rfc3339_opts(SecondsFormat::Millis, true),
                "state": state,
                "speedKph": speed,
                "batteryPct": (battery * 10.0).round() / 10.0,
                "odometerMeters": odometer as i64,
                "routeId": "R-118",
                "faults": []
            })
            .to_string();

            let now = Instant::now();

            // Fault: silent device
            if let Some(t) = silent_until {
                if now >= t {
                    silent_until = None;
                }
            }
            if silent_until.is_none() && rng.gen_bool(silent_rate) {
                let secs: u64 = rng.gen_range(30..=60);
                silent_until = Some(now + Duration::from_secs(secs));
                eprintln!("fault: silent for {secs}s");
            }

            if silent_until.is_none() {
                // Publish held-back (late) messages whose time has come
                let mut i = 0;
                while i < held.len() {
                    if held[i].0 <= now {
                        let (_, p) = held.remove(i);
                        let _ = client.publish(topic, QoS::AtMostOnce, false, p);
                    } else {
                        i += 1;
                    }
                }

                if rng.gen_bool(late_rate) {
                    // Fault: late / out-of-order (keeps its original ts)
                    let delay: u64 = rng.gen_range(1..=10);
                    held.push((now + Duration::from_secs(delay), payload));
                } else {
                    if let Err(e) = client.publish(topic, QoS::AtMostOnce, false, payload.clone()) {
                        eprintln!("publish failed: {e}");
                    }
                    // Fault: duplicate delivery
                    if rng.gen_bool(dup_rate) {
                        let _ = client.publish(topic, QoS::AtMostOnce, false, payload);
                    }
                }
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