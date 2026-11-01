\# Fleet Telemetry Pipeline



Simulator (Rust) -> EMQX (MQTT broker) -> Ingest service (Go) -> HTTP API



\## Run

1\. docker compose up -d       (starts EMQX and the ingest service)

2\. Start the simulator:

&#x20;  cd simulator

&#x20;  docker run --rm --init -v ${PWD}:/app -w /app -e CARGO\_TARGET\_DIR=/tmp/target -e MQTT\_HOST=host.docker.internal rust:latest cargo run

3\. Open http://localhost:8080/api/devices and http://localhost:8080/stats

&#x20;  EMQX dashboard: http://localhost:18083 (admin / public)



\## What works

\- Go ingest service subscribes to fleet/+/+/telemetry

\- Validation of every message

\- Duplicates and late messages are rejected

\- Odometer drop is treated as a tracker reset: distance is never negative

\- OFFLINE after 15 s of silence, ONLINE on the next message

\- 60-second rolling window: average speed and battery, moving %, distance

\- HTTP API: /healthz, /api/devices, /api/devices/{id}, /stats

\- Tests pass with `go test -race ./...`

\- Rust simulator: one vehicle with a state machine and a consistent odometer



&#x20;

