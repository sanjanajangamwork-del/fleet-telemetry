Live Fleet Telemetry Pipeline
-A real-time vehicle telemetry monitoring system built using Rust, MQTT, Go, and a web dashboard.

# 1. Architecture
The system consists of four services:

Rust Simulator → EMQX MQTT Broker → Go Ingestion Service → Web Dashboard

Simulator: Generates telemetry for 20 vehicles at 5 messages per second per vehicle.
EMQX: Receives and distributes telemetry using MQTT.
Go Ingestion Service: Validates messages, processes vehicle data, calculates statistics, and exposes HTTP/SSE endpoints.
Web Dashboard: Displays live vehicle information and fleet statistics.

# 2. Technologies Used

 - Rust
 - Go
 - MQTT
 - EMQX
 - HTML, CSS, JavaScript
 - Docker and Docker Compose
 - Server-Sent Events (SSE)

# 3. Requirements

-Docker Desktop or Docker Engine
-Docker Compose
-Git (for cloning the repository)

No local Rust or Go installation is required when running through Docker Compose.
 
 # 4. Run the Project

Clone the repository:

- git clone https://github.com/sanjanajangamwork-del/fleet-telemetry.git
cd fleet-telemetry

- Build and start all services:
   docker compose up -d --build

- Check service status:
   docker compose ps

- Open the dashboard:
   http://localhost:3000

# Other endpoints:

- API: http://localhost:8080/api/devices
- Health: http://localhost:8080/healthz
- Statistics: http://localhost:8080/stats
- EMQX Dashboard: http://localhost:18083

# 5. Features

# Telemetry Processing

- MQTT subscription to vehicle telemetry topics.
- Validation of incoming telemetry messages.
- Duplicate message rejection.
- Out-of-order message rejection.
- Odometer reset handling without negative distance calculations.
- Vehicle ONLINE/OFFLINE detection.
- A vehicle is marked OFFLINE after more than 15 seconds without telemetry.

# Rolling Statistics

The system maintains a rolling 60-second window for:

- Average speed.
- Average battery level.
- Moving percentage.
- Distance travelled.

# API Endpoints

| Endpoint                               | Description                |
| -------------------------------------- | -------------------------- |
| `/healthz`                             | Service health             |
| `/api/devices`                         | List vehicles              |
| `/api/devices/{id}`                    | Individual vehicle details |
| `/api/devices/{id}/history?window=15m` | Vehicle telemetry history  |
| `/api/stream`                          | Live updates through SSE   |
| `/stats`                               | Fleet statistics           |

# Dashboard

The dashboard displays:

- Total vehicles.
- Online and offline vehicles.
- Messages per second.
- Individual vehicle speed.
- Battery percentage.
- Moving percentage.
- Distance travelled.
- Last message information.

Live vehicle updates are delivered through Server-Sent Events.

# 6. Testing

Run Go tests locally from the ingest directory:

- go test -race ./...
(The tests cover important telemetry-processing scenarios, including duplicate messages, out-of-order messages, odometer resets, and offline detection.)

# 7. Docker Services

The application uses four Docker Compose services:

| Service   | Purpose                         |
| --------- | ------------------------------- |
| emqx      | MQTT broker                     |
| ingest    | Go telemetry processing and API |
| simulator | Rust telemetry simulator        |
| web       | Dashboard served through Nginx  |

Stop the application:
docker compose down

View logs:
docker compose logs -f

View logs for an individual service:
docker compose logs -f ingest

## Docker Image Sizes

The following table shows the content size of each Docker image used in the project.

| Docker Image                 | Content Size |
| ---------------------------- | -----------: |
| `fleet-ingest:latest`        |      10.1 MB |
| `fleet-simulator:latest`     |      29.4 MB |
| `fleet-telemetry-web:latest` |      26.4 MB |
| `emqx/emqx:latest`           |       153 MB |

The Go ingestion, Rust simulator and web dashboard use separately built application images. EMQX is used as the MQTT broker.
