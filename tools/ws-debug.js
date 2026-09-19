#!/usr/bin/env node
/**
 * WebSocket Debugging & Performance Testing Tool for Respondent
 *
 * Usage:
 *   node tools/ws-debug.js                    # Run full diagnostic
 *   node tools/ws-debug.js --layer satellites  # Test a single layer
 *   node tools/ws-debug.js --perf             # Run throughput benchmark
 *   node tools/ws-debug.js --all-layers       # Subscribe to all layers sequentially
 */

const WebSocket = require("ws");

const WS_URL = process.env.WS_URL || "ws://localhost:22012/ws";
const CONNECT_TIMEOUT = 5000;
const RESPONSE_TIMEOUT = 10000;

// Spatial layers use viewport-based filtering (GEOSEARCH in Valkey).
// Derived from sources.d/*.yaml files with `filtering: viewport`.
const SPATIAL_LAYERS = new Set([
  "flights_commercial",
  "flights_military",
  "satellites",
  "radiosondes",
  "earthquakes",
  "ocean_buoys",
  "ems_activations",
  "wildfires",
  "lightning",
  "iss",
]);

// Non-spatial layers use full-index scan (SMEMBERS/SSCAN in Valkey).
// Subscribing to these WITH a viewport causes 0 entities (backend bug/design).
const NON_SPATIAL_LAYERS = [
  "fires_active",
  "ships",
  "internet_infrastructure",
  "air_quality",
  "radiation",
  "ukraine_civilian_harm",
  "subsea_cables",
  "weather_alerts",
  "nuclear_facilities",
  "news_articles",
  "disaster_alerts",
  "meshtastic",
  "conflict_events",
  "aviation_weather",
  "border_crossings",
  "volcanoes",
  "rocket_launches",
  "internet_outages",
  "radiation_us",
  "markets",
  "space_weather",
  "radio_aprs",
];

// All known layer types from the database
const ALL_LAYERS = [...SPATIAL_LAYERS, ...NON_SPATIAL_LAYERS];

// Wide viewport covering most of the globe (avoids haversine edge case at ±180°)
const GLOBAL_VIEWPORT = { west: -179, south: -85, east: 179, north: 85 };

function isSpatial(layerType) {
  return SPATIAL_LAYERS.has(layerType);
}

// Parse CLI args
const args = process.argv.slice(2);
const singleLayer = args.includes("--layer")
  ? args[args.indexOf("--layer") + 1]
  : null;
const perfMode = args.includes("--perf");
const allLayersMode = args.includes("--all-layers");
const timeRangeMode = args.includes("--time-range");
const verbose = args.includes("--verbose") || args.includes("-v");

function createConnection() {
  return new Promise((resolve, reject) => {
    const startTime = Date.now();
    const ws = new WebSocket(WS_URL);
    const timeout = setTimeout(() => {
      ws.close();
      reject(new Error(`Connection timeout after ${CONNECT_TIMEOUT}ms`));
    }, CONNECT_TIMEOUT);

    ws.on("open", () => {
      clearTimeout(timeout);
      const elapsed = Date.now() - startTime;
      resolve({ ws, connectTime: elapsed });
    });

    ws.on("error", (err) => {
      clearTimeout(timeout);
      reject(err);
    });
  });
}

function sendAndWaitForResponse(ws, message, timeoutMs = RESPONSE_TIMEOUT) {
  return new Promise((resolve, reject) => {
    const startTime = Date.now();
    let responseCount = 0;
    let totalBytes = 0;
    const messages = [];

    const timeout = setTimeout(() => {
      ws.removeListener("message", handler);
      resolve({
        messages,
        responseCount,
        totalBytes,
        elapsed: Date.now() - startTime,
        timedOut: true,
      });
    }, timeoutMs);

    function handler(data) {
      responseCount++;
      const dataStr = data.toString();
      totalBytes += dataStr.length;
      try {
        const parsed = JSON.parse(dataStr);
        messages.push(parsed);
      } catch {
        messages.push({ raw: dataStr.slice(0, 200) });
      }
    }

    ws.on("message", handler);
    ws.send(JSON.stringify(message));
  });
}

function collectMessages(ws, durationMs) {
  return new Promise((resolve) => {
    const startTime = Date.now();
    let messageCount = 0;
    let totalBytes = 0;
    const typeCounts = {};
    const layerCounts = {};
    let entityCount = 0;

    function handler(data) {
      messageCount++;
      const dataStr = data.toString();
      totalBytes += dataStr.length;
      try {
        const parsed = JSON.parse(dataStr);
        const type = parsed.type || "unknown";
        typeCounts[type] = (typeCounts[type] || 0) + 1;
        if (parsed.layer_id) {
          layerCounts[parsed.layer_id] = (layerCounts[parsed.layer_id] || 0) + 1;
        }
        // Count entities in different message formats
        if (parsed.data?.entities) {
          entityCount += parsed.data.entities.length;
        }
        if (parsed.data?.entity) {
          entityCount += 1;
        }
        if (parsed.data?.updates) {
          entityCount += parsed.data.updates.length;
        }
      } catch {
        // ignore parse errors
      }
    }

    ws.on("message", handler);

    setTimeout(() => {
      ws.removeListener("message", handler);
      const elapsed = Date.now() - startTime;
      resolve({
        messageCount,
        totalBytes,
        typeCounts,
        layerCounts,
        entityCount,
        elapsed,
        messagesPerSecond: (messageCount / (elapsed / 1000)).toFixed(2),
        bytesPerSecond: (totalBytes / (elapsed / 1000)).toFixed(0),
        kbPerSecond: (totalBytes / 1024 / (elapsed / 1000)).toFixed(2),
      });
    }, durationMs);
  });
}

// ---- Tests ----

async function testConnection() {
  console.log("\n=== TEST: WebSocket Connection ===");
  try {
    const { ws, connectTime } = await createConnection();
    console.log(`  ✓ Connected in ${connectTime}ms`);
    console.log(`  URL: ${WS_URL}`);
    console.log(`  readyState: ${ws.readyState} (OPEN=1)`);
    ws.close();
    return true;
  } catch (err) {
    console.log(`  ✗ Connection failed: ${err.message}`);
    return false;
  }
}

async function testLayerSubscription(layerType) {
  const spatial = isSpatial(layerType);
  console.log(`\n--- Layer: ${layerType} (${spatial ? "spatial" : "non-spatial"}) ---`);
  const { ws } = await createConnection();

  // Spatial layers need a viewport; non-spatial layers must NOT send one
  const subscribeMsg = {
    type: "subscribe",
    layer_id: layerType,
    data: spatial ? { viewport: GLOBAL_VIEWPORT } : {},
  };

  const result = await sendAndWaitForResponse(ws, subscribeMsg, 5000);

  let snapshotEntities = 0;
  let liveUpdates = 0;
  let batchUpdates = 0;
  let snapshotFound = false;

  for (const msg of result.messages) {
    if (msg.type === "snapshot" || msg.type === "layer.snapshot") {
      snapshotFound = true;
      const entities = msg.data?.entities || [];
      snapshotEntities += entities.length;
      if (verbose && entities.length > 0) {
        const sample = entities[0];
        console.log(`    Sample entity: ${JSON.stringify(sample).slice(0, 200)}`);
      }
    } else if (msg.type === "layer.update") {
      liveUpdates++;
    } else if (msg.type === "layer.batch_update") {
      batchUpdates++;
      if (msg.data?.updates) {
        liveUpdates += msg.data.updates.length;
      }
    }
  }

  const status = snapshotEntities > 0 ? "✓" : snapshotFound ? "⚠ (0 entities)" : "✗ (no snapshot)";
  console.log(`  ${status} Snapshot: ${snapshotEntities} entities`);
  if (liveUpdates > 0) console.log(`    + ${liveUpdates} live updates`);
  if (batchUpdates > 0) console.log(`    + ${batchUpdates} batch messages`);
  console.log(`    Messages: ${result.responseCount}, Bytes: ${result.totalBytes}, Time: ${result.elapsed}ms`);

  ws.close();
  return { layerType, snapshotEntities, liveUpdates, totalMessages: result.responseCount, totalBytes: result.totalBytes };
}

async function testAllLayers() {
  console.log("\n=== TEST: All Layer Subscriptions ===");
  const results = [];

  for (const layer of ALL_LAYERS) {
    try {
      const result = await testLayerSubscription(layer);
      results.push(result);
    } catch (err) {
      console.log(`  ✗ ${layer}: ${err.message}`);
      results.push({ layerType: layer, snapshotEntities: -1, error: err.message });
    }
  }

  console.log("\n=== SUMMARY: Layer Data ===");
  console.log(
    "Layer".padEnd(30) +
    "Entities".padStart(10) +
    "Live Upd".padStart(10) +
    "Messages".padStart(10) +
    "Bytes".padStart(12)
  );
  console.log("-".repeat(72));

  let totalEntities = 0;
  let emptyLayers = 0;
  for (const r of results.sort((a, b) => (b.snapshotEntities || 0) - (a.snapshotEntities || 0))) {
    const entities = r.snapshotEntities >= 0 ? String(r.snapshotEntities) : "ERROR";
    console.log(
      r.layerType.padEnd(30) +
      entities.padStart(10) +
      String(r.liveUpdates || 0).padStart(10) +
      String(r.totalMessages || 0).padStart(10) +
      String(r.totalBytes || 0).padStart(12)
    );
    if (r.snapshotEntities > 0) totalEntities += r.snapshotEntities;
    if (r.snapshotEntities === 0) emptyLayers++;
  }
  console.log("-".repeat(72));
  console.log(`Total entities received: ${totalEntities}`);
  console.log(`Layers with data: ${results.filter(r => r.snapshotEntities > 0).length}/${ALL_LAYERS.length}`);
  console.log(`Empty layers: ${emptyLayers}`);
  console.log(`Failed layers: ${results.filter(r => r.snapshotEntities < 0).length}`);
}

async function testTimeRanges() {
  console.log("\n=== TEST: Time Range Modes ===");
  const testLayer = singleLayer || "flights_commercial";
  const now = new Date();

  const ranges = [
    { name: "Live (no range)", from: null, to: null },
    { name: "1h", from: new Date(now - 3600000), to: now },
    { name: "8h", from: new Date(now - 8 * 3600000), to: now },
    { name: "24h", from: new Date(now - 24 * 3600000), to: now },
  ];

  for (const range of ranges) {
    const { ws } = await createConnection();

    if (!range.from) {
      // Live mode — subscribe
      const msg = {
        type: "subscribe",
        layer_id: testLayer,
        data: { viewport: GLOBAL_VIEWPORT },
      };
      const result = await sendAndWaitForResponse(ws, msg, 5000);
      let entities = 0;
      for (const m of result.messages) {
        if (m.data?.entities) entities += m.data.entities.length;
      }
      console.log(`  ${range.name}: ${entities} entities (${result.responseCount} msgs, ${result.totalBytes} bytes)`);
    } else {
      // First subscribe to the layer
      ws.send(JSON.stringify({
        type: "subscribe",
        layer_id: testLayer,
        data: { viewport: GLOBAL_VIEWPORT },
      }));
      // Wait a moment for subscribe to process
      await new Promise((resolve) => setTimeout(resolve, 1000));

      // Now send time_range
      const msg = {
        type: "time_range",
        layer_id: testLayer,
        data: {
          from: range.from.toISOString(),
          to: range.to.toISOString(),
          viewport: GLOBAL_VIEWPORT,
        },
      };
      const result = await sendAndWaitForResponse(ws, msg, 8000);
      let entities = 0;
      for (const m of result.messages) {
        if (m.data?.entities) entities += m.data.entities.length;
      }
      console.log(`  ${range.name}: ${entities} entities (${result.responseCount} msgs, ${result.totalBytes} bytes)`);
    }
    ws.close();
  }
}

async function testThroughputBenchmark() {
  console.log("\n=== TEST: Throughput Benchmark (30s) ===");
  const { ws } = await createConnection();

  // Subscribe to high-volume layers
  const highVolumeLayers = [
    "flights_commercial",
    "flights_military",
    "ships",
    "lightning",
    "satellites",
    "fires_active",
  ];

  for (const layer of highVolumeLayers) {
    const spatial = isSpatial(layer);
    ws.send(
      JSON.stringify({
        type: "subscribe",
        layer_id: layer,
        data: spatial ? { viewport: GLOBAL_VIEWPORT } : {},
      })
    );
    // Small delay between subscriptions to avoid overwhelming
    await new Promise((resolve) => setTimeout(resolve, 200));
  }

  console.log(`  Subscribed to ${highVolumeLayers.length} high-volume layers`);
  console.log("  Collecting messages for 30 seconds...\n");

  const stats = await collectMessages(ws, 30000);

  console.log("  === Results ===");
  console.log(`  Total messages:     ${stats.messageCount}`);
  console.log(`  Total entities:     ${stats.entityCount}`);
  console.log(`  Total data:         ${(stats.totalBytes / 1024).toFixed(1)} KB`);
  console.log(`  Messages/sec:       ${stats.messagesPerSecond}`);
  console.log(`  Throughput:         ${stats.kbPerSecond} KB/s`);
  console.log(`  Duration:           ${stats.elapsed}ms`);

  console.log("\n  Message types:");
  for (const [type, count] of Object.entries(stats.typeCounts).sort((a, b) => b[1] - a[1])) {
    console.log(`    ${type}: ${count}`);
  }

  console.log("\n  Per-layer message counts:");
  for (const [layer, count] of Object.entries(stats.layerCounts).sort((a, b) => b[1] - a[1])) {
    console.log(`    ${layer}: ${count}`);
  }

  ws.close();
  return stats;
}

async function testMultiClientThroughput() {
  console.log("\n=== TEST: Multi-Client Throughput (10 clients, 15s) ===");
  const clientCount = 10;
  const clients = [];

  for (let i = 0; i < clientCount; i++) {
    const { ws, connectTime } = await createConnection();
    clients.push({ ws, id: i, connectTime });
  }

  console.log(`  ${clientCount} clients connected`);

  // Subscribe all clients to flights_commercial (spatial layer — needs viewport)
  for (const client of clients) {
    client.ws.send(
      JSON.stringify({
        type: "subscribe",
        layer_id: "flights_commercial",
        data: { viewport: GLOBAL_VIEWPORT },
      })
    );
  }

  console.log("  All subscribed to flights_commercial");
  console.log("  Collecting for 15 seconds...\n");

  // Collect messages from all clients in parallel
  const promises = clients.map((client) => collectMessages(client.ws, 15000));
  const allStats = await Promise.all(promises);

  let totalMessages = 0;
  let totalBytes = 0;
  let totalEntities = 0;

  for (let i = 0; i < allStats.length; i++) {
    const s = allStats[i];
    totalMessages += s.messageCount;
    totalBytes += s.totalBytes;
    totalEntities += s.entityCount;
    if (verbose) {
      console.log(
        `  Client ${i}: ${s.messageCount} msgs, ${s.entityCount} entities, ${s.kbPerSecond} KB/s`
      );
    }
  }

  console.log(`  Aggregate:`);
  console.log(`    Total messages across all clients: ${totalMessages}`);
  console.log(`    Total entities across all clients: ${totalEntities}`);
  console.log(`    Total data: ${(totalBytes / 1024).toFixed(1)} KB`);
  console.log(
    `    Avg messages/client/sec: ${(totalMessages / clientCount / 15).toFixed(2)}`
  );
  console.log(
    `    Avg KB/client/sec: ${(totalBytes / 1024 / clientCount / 15).toFixed(2)}`
  );

  for (const client of clients) {
    client.ws.close();
  }
}

async function testBackpressure() {
  console.log("\n=== TEST: Backpressure Detection ===");
  const { ws } = await createConnection();

  // Subscribe to many layers rapidly (respect spatial/non-spatial)
  for (const layer of ALL_LAYERS) {
    const spatial = isSpatial(layer);
    ws.send(
      JSON.stringify({
        type: "subscribe",
        layer_id: layer,
        data: spatial ? { viewport: GLOBAL_VIEWPORT } : {},
      })
    );
  }

  console.log(`  Subscribed to all ${ALL_LAYERS.length} layers simultaneously`);
  console.log("  Monitoring for backpressure warnings (10s)...\n");

  let backpressureWarnings = 0;
  let errors = 0;
  const stats = await new Promise((resolve) => {
    let messageCount = 0;
    let totalBytes = 0;

    function handler(data) {
      messageCount++;
      const dataStr = data.toString();
      totalBytes += dataStr.length;
      try {
        const parsed = JSON.parse(dataStr);
        if (parsed.type === "backpressure" || parsed.type === "error") {
          backpressureWarnings++;
          console.log(`  ⚠ Backpressure: ${JSON.stringify(parsed).slice(0, 200)}`);
        }
        if (parsed.type === "error") {
          errors++;
        }
      } catch {}
    }

    ws.on("message", handler);

    setTimeout(() => {
      ws.removeListener("message", handler);
      resolve({ messageCount, totalBytes });
    }, 10000);
  });

  console.log(`  Messages received: ${stats.messageCount}`);
  console.log(`  Total data: ${(stats.totalBytes / 1024).toFixed(1)} KB`);
  console.log(`  Backpressure warnings: ${backpressureWarnings}`);
  console.log(`  Errors: ${errors}`);

  ws.close();
}

// ---- Main ----

async function main() {
  console.log("╔════════════════════════════════════════════════════════════╗");
  console.log("║  Respondent WebSocket Diagnostics & Performance Suite     ║");
  console.log("╚════════════════════════════════════════════════════════════╝");
  console.log(`Target: ${WS_URL}`);
  console.log(`Time: ${new Date().toISOString()}`);

  // Test 1: Basic connectivity
  const connected = await testConnection();
  if (!connected) {
    console.log("\nFATAL: Cannot connect to WebSocket server. Aborting.");
    process.exit(1);
  }

  if (singleLayer) {
    // Test single layer
    await testLayerSubscription(singleLayer);
    process.exit(0);
  }

  if (perfMode) {
    // Performance benchmarks only
    await testThroughputBenchmark();
    await testMultiClientThroughput();
    process.exit(0);
  }

  if (timeRangeMode) {
    await testTimeRanges();
    process.exit(0);
  }

  // Full diagnostic
  if (allLayersMode) {
    await testAllLayers();
  }

  await testTimeRanges();
  await testThroughputBenchmark();
  await testMultiClientThroughput();
  await testBackpressure();

  console.log("\n=== DIAGNOSTIC COMPLETE ===");
}

main().catch((err) => {
  console.error("Fatal error:", err);
  process.exit(1);
});
