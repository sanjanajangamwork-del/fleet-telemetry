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

fn prob(name: &str, default: f64) -> f64 {
    env_f64(name, default).clamp(0.0, 1.0)
}

fn run_device(
    client: Client,
    id: String,
    hz: f64,
    dup_rate: f64,
    late_rate: f64,
    reset_rate: f64,
    silent_rate: f64,
) {
    let mut rng = rand::thread_rng();
    let topic = format!("fleet/west-01/{id}/telemetry");
    let period = Duration::from_secs_f64(1.0 / hz);
    let dt = 1.0 / hz;

    let mut odometer: f64 = rng.gen_range(1000.0..200_000.0);
    let mut battery: f64 = rng.gen_range(40.0..100.0);
    let mut state = "IDLE";
    let mut speed = 0.0_f64;
    let mut state_until = Instant::now() + Duration::from_secs(rng.gen_range(2..=10));
    let mut silent_until: Option<Instant> = None;
    let mut held: Vec<(Instant, String)> = Vec::new();

    // stagger devices so they do not all publish at the same instant
    thread::sleep(Duration::from_secs_f64(rng.gen_range(0.0..(1.0 / hz))));
    let mut next = Instant::now();

    loop {
        let now = Instant::now();

        // State machine with dwell times: change state only when the dwell ends
        let charged = state == "CHARGING" && battery >= 95.0;
        if now >= state_until || charged {
            state = match state {
                "MOVING" => {
                    if battery < 30.0 {
                        "CHARGING"
                    } else {
                        "IDLE"
                    }
                }
                "CHARGING" => "IDLE",
                _ => {
                    if battery < 20.0 {
                        "CHARGING"
                    } else {
                        "MOVING"
                    }
                }
            };
            let dwell: u64 = match state {
                "MOVING" => rng.gen_range(15..=45),
                "CHARGING" => rng.gen_range(20..=40),
                _ => rng.gen_range(5..=15),
            };
            state_until = now + Duration::from_secs(dwell);
        }

        match state {
            "MOVING" => {
                speed = 40.0 + rng.gen_range(-5.0..5.0);
                odometer += speed / 3.6 * dt; // km/h to m/s, times the tick length
                battery -= 0.1 * dt;
            }
            "CHARGING" => {
                speed = 0.0;
                battery += 0.5 * dt;
            }
            _ => {
                speed = 0.0;
            }
        }
        battery = battery.clamp(0.0, 100.0);

        // Fault: counter reset (tracker reboot)
        if rng.gen_bool(reset_rate) {
            odometer = 0.0;
            eprintln!("{id}: fault counter reset");
        }

        let payload = json!({
            "deviceId": id,
            "ts": Utc::now().to_rfc3339_opts(SecondsFormat::Millis, true),
            "state": state,
            "speedKph": (speed * 10.0).round() / 10.0,
            "batteryPct": (battery * 10.0).round() / 10.0,
            "odometerMeters": odometer as i64,
            "routeId": "R-118",
            "faults": []
        })
        .to_string();

        // Fault: silent device
        if let Some(t) = silent_until {
            if now >= t {
                silent_until = None;
            }
        }
        if silent_until.is_none() && rng.gen_bool(silent_rate) {
            let secs: u64 = rng.gen_range(30..=60);
            silent_until = Some(now + Duration::from_secs(secs));
            eprintln!("{id}: fault silent for {secs}s");
        }

        if silent_until.is_none() {
            // release held-back (late) messages whose time has come
            let mut i = 0;
            while i < held.len() {
                if held[i].0 <= now {
                    let (_, p) = held.remove(i);
                    let _ = client.publish(topic.clone(), QoS::AtMostOnce, false, p);
                } else {
                    i += 1;
                }
            }

            if rng.gen_bool(late_rate) {
                // Fault: late / out-of-order. Keeps its original ts.
                let delay: u64 = rng.gen_range(1..=10);
                held.push((now + Duration::from_secs(delay), payload));
            } else {
                let _ = client.publish(topic.clone(), QoS::AtMostOnce, false, payload.clone());
                // Fault: duplicate delivery
                if rng.gen_bool(dup_rate) {
                    let _ = client.publish(topic.clone(), QoS::AtMostOnce, false, payload);
                }
            }
        }

        // sleep until the next scheduled tick, so timing does not drift
        next += period;
        let n = Instant::now();
        if next > n {
            thread::sleep(next - n);
        }
    }
}

fn main() {
    let host = env::var("MQTT_HOST").unwrap_or_else(|_| "localhost".to_string());
    let port: u16 = env::var("MQTT_PORT")
        .ok()
        .and_then(|p| p.parse().ok())
        .unwrap_or(1883);
    let client_id = env::var("CLIENT_ID").unwrap_or_else(|_| "simulator-1".to_string());

    let devices = env_f64("NUM_DEVICES", 20.0) as usize;
    let hz = env_f64("PUBLISH_HZ", 5.0).max(0.1);
    let dup_rate = prob("DUPLICATE_RATE", 0.01);
    let late_rate = prob("LATE_RATE", 0.02);
    // about 1 reset per device per 10 minutes
    let reset_rate = prob("RESET_RATE", 1.0 / (600.0 * hz));
    // per device per tick; roughly one device somewhere goes quiet every few minutes
    let silent_rate = prob("SILENT_RATE", 1.0 / (180.0 * hz * devices as f64));

    let mut opts = MqttOptions::new(client_id, host, port);
    opts.set_keep_alive(Duration::from_secs(5));
    let (client, mut connection) = Client::new(opts, 4096);

    for i in 0..devices {
        let c = client.clone();
        let id = format!("VEH-{}", 1000 + i);
        thread::spawn(move || run_device(c, id, hz, dup_rate, late_rate, reset_rate, silent_rate));
    }
    eprintln!("simulating {devices} devices at {hz} Hz");

    for event in connection.iter() {
        if let Err(e) = event {
            eprintln!("connection error: {e}");
            thread::sleep(Duration::from_secs(1));
        }
    }
}