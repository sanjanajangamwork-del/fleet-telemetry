const API = "";

const grid = document.getElementById("vehicleGrid");

let devices = new Map();
let previousReceived = 0;

function get(obj, lower, upper) {
    return obj?.[lower] ?? obj?.[upper];
}

function formatNumber(value, decimals = 1) {
    return Number(value || 0).toFixed(decimals);
}

// Load all devices when dashboard opens.
async function loadDevices() {
    try {
        const response = await fetch(`${API}/api/devices`);
        if (!response.ok) throw new Error("Device API unavailable");

        const data = await response.json();

        devices.clear();

        data.forEach(device => {
            devices.set(get(device, "deviceId", "DeviceID"), device);
        });

        renderDashboard();
    } catch (error) {
        console.error("Device loading failed:", error);
        grid.innerHTML = `<div class="loading">Unable to connect to backend.</div>`;
    }
}

// Render vehicle cards.
function renderDashboard() {
    const list = Array.from(devices.values());

    const online = list.filter(d => get(d, "online", "Online")).length;
    const offline = list.length - online;

    document.getElementById("totalDevices").textContent = list.length;
    document.getElementById("onlineDevices").textContent = online;
    document.getElementById("offlineDevices").textContent = offline;

    document.getElementById("lastUpdated").textContent =
        "Updated " + new Date().toLocaleTimeString();

    if (list.length === 0) {
        grid.innerHTML = `<div class="loading">No vehicles connected.</div>`;
        return;
    }

    grid.innerHTML = list.map(device => {
        const id = get(device, "deviceId", "DeviceID");
        const online = get(device, "online", "Online");
        const latest = get(device, "latest", "Latest") || {};
        const stats = get(device, "window60s", "Window60s") || {};

        const state = latest.state || latest.State || "OFF";
        const speed = latest.speedKph ?? latest.SpeedKph ?? 0;
        const battery = latest.batteryPct ?? latest.BatteryPct ?? 0;

        const moving = stats.movingPct ?? stats.MovingPct ?? 0;
        const distance = stats.distanceM ?? stats.DistanceM ?? 0;
        const totalDistance = get(device, "totalDistanceM", "TotalDistanceM") || 0;

        const lastSeen = get(device, "lastSeen", "LastSeen");
        const seconds = lastSeen
            ? Math.max(0, Math.floor((Date.now() - new Date(lastSeen).getTime()) / 1000))
            : "-";

        return `
            <article class="vehicle-card ${online ? "" : "offline"}">
                <div class="vehicle-header">
                    <h3>${id}</h3>
                    <span class="badge ${online ? state : "OFF"}">
                        ${online ? state : "OFFLINE"}
                    </span>
                </div>

                <div class="vehicle-main">
                    <div>
                        <div class="metric-label">SPEED</div>
                        <div class="metric-value">${formatNumber(speed)} <small>km/h</small></div>
                    </div>

                    <div>
                        <div class="metric-label">BATTERY</div>
                        <div class="metric-value">${formatNumber(battery, 0)}%</div>
                        <div class="battery-track">
                            <div class="battery-fill" style="width:${Math.min(100, Math.max(0, battery))}%"></div>
                        </div>
                    </div>
                </div>

                <div class="vehicle-details">
                    <div>
                        <div class="metric-label">MOVING TIME</div>
                        <div class="detail-value">${formatNumber(moving)}%</div>
                    </div>

                    <div>
                        <div class="metric-label">DISTANCE / 60 SEC</div>
                        <div class="detail-value">${formatNumber(distance, 0)} m</div>
                    </div>

                    <div>
                        <div class="metric-label">TOTAL DISTANCE</div>
                        <div class="detail-value">${formatNumber(totalDistance, 0)} m</div>
                    </div>

                    <div>
                        <div class="metric-label">LAST MESSAGE</div>
                        <div class="detail-value">${seconds} sec ago</div>
                    </div>
                </div>
            </article>
        `;
    }).join("");
}

// Connect to live Server-Sent Events.
function connectStream() {
    const stream = new EventSource(`${API}/api/stream`);

    stream.onmessage = event => {
        try {
            const device = JSON.parse(event.data);
            const id = get(device, "deviceId", "DeviceID");

            devices.set(id, device);
            renderDashboard();
        } catch (error) {
            console.error("Stream message error:", error);
        }
    };

    stream.onerror = () => {
        console.warn("Stream disconnected. Browser will reconnect automatically.");
    };
}

// Calculate actual incoming MQTT messages per second.
async function updateMessageRate() {
    try {
        const response = await fetch(`${API}/stats`);
        if (!response.ok) return;

        const stats = await response.json();
        const received = stats.received ?? stats.Received ?? 0;

        const rate = received - previousReceived;

        document.getElementById("messageRate").textContent =
            Math.max(0, rate);

        previousReceived = received;
    } catch (error) {
        console.error("Stats unavailable:", error);
    }
}

// Clock.
function updateClock() {
    document.getElementById("clock").textContent =
        new Date().toLocaleTimeString();
}

loadDevices();
connectStream();
updateMessageRate();

setInterval(updateMessageRate, 1000);
setInterval(updateClock, 1000);
setInterval(loadDevices, 10000);

updateClock();