#!/usr/bin/env node
/**
 * Measure actual entity counts and payload sizes for all layers
 * when subscribed WITHOUT viewport (full dataset).
 */
const WebSocket = require("ws");
const ws = new WebSocket("ws://localhost:22012/ws");

const allLayers = [
  "flights_commercial", "flights_military", "satellites", "earthquakes",
  "ems_activations", "fires_active", "ships", "lightning", "air_quality",
  "ocean_buoys", "radiosondes", "subsea_cables", "volcanoes",
  "markets", "space_weather",
];

const results = {};

ws.on("open", () => {
  for (const l of allLayers) {
    results[l] = { total: 0, chunks: 0, bytes: 0 };
    ws.send(JSON.stringify({ type: "subscribe", layer_id: l, data: {} }));
  }
});

ws.on("message", (rawData) => {
  try {
    const str = rawData.toString();
    const msg = JSON.parse(str);
    const lid = msg.layer_id;
    if (lid && results[lid]) {
      if (msg.type === "layer.snapshot") {
        const n = (msg.data && msg.data.entities) ? msg.data.entities.length : 0;
        results[lid].total += n;
        results[lid].chunks++;
        results[lid].bytes += str.length;
      } else if (msg.type === "indicator.update") {
        results[lid].total = "indicator";
        results[lid].chunks++;
        results[lid].bytes += str.length;
      }
    }
  } catch (e) { /* ignore */ }
});

setTimeout(() => {
  console.log("Layer".padEnd(24) + "Entities".padEnd(10) + "Chunks".padEnd(8) + "Size (KB)");
  console.log("-".repeat(55));
  let totalEntities = 0;
  let totalBytes = 0;
  for (const l of allLayers) {
    const r = results[l];
    const count = typeof r.total === "number" ? r.total : 0;
    totalEntities += count;
    totalBytes += r.bytes;
    console.log(
      l.padEnd(24) +
      String(r.total).padEnd(10) +
      String(r.chunks).padEnd(8) +
      (r.bytes / 1024).toFixed(0)
    );
  }
  console.log("-".repeat(55));
  console.log("TOTAL".padEnd(24) + String(totalEntities).padEnd(10) + "".padEnd(8) + (totalBytes / 1024 / 1024).toFixed(1) + " MB");
  console.log("\nNote: Layers currently capped at 2000 entities by backend MaxResults.");
  console.log("Actual dataset sizes may be much larger (e.g., flights_commercial has 43K+ in Postgres).");
  ws.close();
  process.exit(0);
}, 15000);
