1. MQTT Communication: Used EMQX as the central broker to connect the Rust simulator with the Go ingestion service.

2. Data Validation: Validate incoming telemetry before updating vehicle records.

3. Duplicate and Late Messages: Ignore duplicate and out-of-order messages to protect the latest vehicle state.

4. Odometer Resets: Treat odometer decreases as counter resets instead of calculating negative distance.

5. Offline Detection: Mark vehicles offline after 15 seconds without receiving telemetry.

6. Rolling Statistics: Maintain a 60-second window for speed, battery, moving percentage and distance calculations.

7. Live Updates: Use SSE with bounded client buffers so slow dashboard clients do not block telemetry ingestion.

8. Deployment: Use Docker Compose to run the broker, Go ingestion service, Rust simulator and web dashboard together.
