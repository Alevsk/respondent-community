#!/usr/bin/env node
/**
 * WebSocket Stress Test for Respondent
 *
 * Tests high-throughput scenarios:
 *   node tools/ws-stress.js                    # Default: 50 clients, 30s
 *   node tools/ws-stress.js --clients 100      # 100 concurrent clients
 *   node tools/ws-stress.js --duration 60      # 60 second test
 *   node tools/ws-stress.js --ramp              # Ramp up clients gradually
 */

const WebSocket = require("ws");

const WS_URL = process.env.WS_URL || "ws://localhost:22012/ws";
const args = process.argv.slice(2);

const MAX_CLIENTS = args.includes("--clients")
  ? parseInt(args[args.indexOf("--clients") + 1], 10)
  : 50;
const DURATION_S = args.includes("--duration")
  ? parseInt(args[args.indexOf("--duration") + 1], 10)
  : 30;
const RAMP = args.includes("--ramp");

// Spatial layers need viewport
const SPATIAL_LAYERS = new Set([
  "flights_commercial", "flights_military", "satellites", "radiosondes",
  "earthquakes", "ocean_buoys", "lightning", "iss",
]);

const TEST_LAYERS = [
  "flights_commercial", "flights_military", "satellites",
  "fires_active", "ships", "lightning",
];

const VIEWPORT = { west: -120, south: -60, east: 50, north: 70 };

class ClientStats {
  constructor(id) {
    this.id = id;
    this.messageCount = 0;
    this.entityCount = 0;
    this.totalBytes = 0;
    this.errors = 0;
    this.backpressure = 0;
    this.connectTime = 0;
    this.firstMessageTime = 0;
    this.startTime = 0;
    this.latencies = [];
  }
}

async function connectClient(id) {
  return new Promise((resolve, reject) => {
    const stats = new ClientStats(id);
    const startTime = Date.now();
    const ws = new WebSocket(WS_URL);
    const timeout = setTimeout(() => {
      ws.close();
      reject(new Error(`Client ${id}: connection timeout`));
    }, 10000);

    ws.on("open", () => {
      clearTimeout(timeout);
      stats.connectTime = Date.now() - startTime;
      stats.startTime = Date.now();
      resolve({ ws, stats });
    });

    ws.on("error", (err) => {
      clearTimeout(timeout);
      reject(err);
    });
  });
}

function subscribeClient(ws, stats) {
  const layers = TEST_LAYERS;
  for (const layer of layers) {
    const spatial = SPATIAL_LAYERS.has(layer);
    ws.send(JSON.stringify({
      type: "subscribe",
      layer_id: layer,
      data: spatial ? { viewport: VIEWPORT } : {},
    }));
  }

  ws.on("message", (data) => {
    const dataStr = data.toString();
    stats.messageCount++;
    stats.totalBytes += dataStr.length;

    if (stats.firstMessageTime === 0) {
      stats.firstMessageTime = Date.now() - stats.startTime;
    }

    try {
      const msg = JSON.parse(dataStr);
      if (msg.data?.entities) stats.entityCount += msg.data.entities.length;
      if (msg.data?.entity) stats.entityCount++;
      if (msg.data?.updates) stats.entityCount += msg.data.updates.length;
      if (msg.type === "backpressure" || msg.type === "error") stats.backpressure++;
    } catch {
      stats.errors++;
    }
  });

  ws.on("error", () => { stats.errors++; });
}

async function runStressTest() {
  console.log("╔════════════════════════════════════════════════════════════╗");
  console.log("║  Respondent WebSocket Stress Test                         ║");
  console.log("╚════════════════════════════════════════════════════════════╝");
  console.log(`Target: ${WS_URL}`);
  console.log(`Clients: ${MAX_CLIENTS}, Duration: ${DURATION_S}s, Ramp: ${RAMP}`);
  console.log(`Layers: ${TEST_LAYERS.join(", ")}`);
  console.log("");

  const clients = [];
  const allStats = [];

  // Connect clients
  if (RAMP) {
    const batchSize = Math.ceil(MAX_CLIENTS / 5);
    for (let i = 0; i < MAX_CLIENTS; i += batchSize) {
      const batch = Math.min(batchSize, MAX_CLIENTS - i);
      console.log(`  Connecting clients ${i + 1}-${i + batch}...`);
      const promises = [];
      for (let j = 0; j < batch; j++) {
        promises.push(connectClient(i + j).catch((err) => {
          console.log(`  ✗ Client ${i + j}: ${err.message}`);
          return null;
        }));
      }
      const results = await Promise.all(promises);
      for (const r of results) {
        if (r) {
          clients.push(r);
          allStats.push(r.stats);
        }
      }
      if (i + batch < MAX_CLIENTS) {
        await new Promise((resolve) => setTimeout(resolve, 1000));
      }
    }
  } else {
    console.log(`  Connecting ${MAX_CLIENTS} clients simultaneously...`);
    const promises = [];
    for (let i = 0; i < MAX_CLIENTS; i++) {
      promises.push(connectClient(i).catch((err) => {
        console.log(`  ✗ Client ${i}: ${err.message}`);
        return null;
      }));
    }
    const results = await Promise.all(promises);
    for (const r of results) {
      if (r) {
        clients.push(r);
        allStats.push(r.stats);
      }
    }
  }

  const connectedCount = clients.length;
  const avgConnectTime = allStats.reduce((sum, s) => sum + s.connectTime, 0) / connectedCount;
  const maxConnectTime = Math.max(...allStats.map((s) => s.connectTime));
  console.log(`\n  ✓ ${connectedCount}/${MAX_CLIENTS} clients connected`);
  console.log(`  Avg connect time: ${avgConnectTime.toFixed(0)}ms, Max: ${maxConnectTime}ms`);

  // Subscribe all clients
  console.log(`\n  Subscribing to ${TEST_LAYERS.length} layers each...`);
  for (const { ws, stats } of clients) {
    subscribeClient(ws, stats);
  }

  // Collect data
  console.log(`  Collecting data for ${DURATION_S} seconds...\n`);

  // Print progress every 5 seconds
  const progressInterval = setInterval(() => {
    const totalMsgs = allStats.reduce((sum, s) => sum + s.messageCount, 0);
    const totalEntities = allStats.reduce((sum, s) => sum + s.entityCount, 0);
    const totalBytes = allStats.reduce((sum, s) => sum + s.totalBytes, 0);
    const elapsed = (Date.now() - allStats[0].startTime) / 1000;
    console.log(`    [${elapsed.toFixed(0)}s] msgs=${totalMsgs}, entities=${totalEntities}, data=${(totalBytes / 1024).toFixed(0)}KB`);
  }, 5000);

  await new Promise((resolve) => setTimeout(resolve, DURATION_S * 1000));
  clearInterval(progressInterval);

  // Aggregate results
  let totalMessages = 0;
  let totalEntities = 0;
  let totalBytes = 0;
  let totalErrors = 0;
  let totalBackpressure = 0;
  let clientsWithData = 0;

  for (const s of allStats) {
    totalMessages += s.messageCount;
    totalEntities += s.entityCount;
    totalBytes += s.totalBytes;
    totalErrors += s.errors;
    totalBackpressure += s.backpressure;
    if (s.messageCount > 0) clientsWithData++;
  }

  const avgMsgPerClient = totalMessages / connectedCount;
  const avgEntitiesPerClient = totalEntities / connectedCount;
  const avgBytesPerClient = totalBytes / connectedCount;

  console.log("\n╔════════════════════════════════════════════════════╗");
  console.log("║  STRESS TEST RESULTS                               ║");
  console.log("╚════════════════════════════════════════════════════╝");
  console.log(`  Connected clients:        ${connectedCount}/${MAX_CLIENTS}`);
  console.log(`  Clients receiving data:   ${clientsWithData}/${connectedCount}`);
  console.log(`  Duration:                 ${DURATION_S}s`);
  console.log("");
  console.log("  === Aggregate Throughput ===");
  console.log(`  Total messages:           ${totalMessages}`);
  console.log(`  Total entities:           ${totalEntities}`);
  console.log(`  Total data:               ${(totalBytes / 1024 / 1024).toFixed(2)} MB`);
  console.log(`  Messages/sec (aggregate): ${(totalMessages / DURATION_S).toFixed(2)}`);
  console.log(`  Entities/sec (aggregate): ${(totalEntities / DURATION_S).toFixed(2)}`);
  console.log(`  Throughput (aggregate):   ${(totalBytes / 1024 / DURATION_S).toFixed(2)} KB/s`);
  console.log("");
  console.log("  === Per-Client Averages ===");
  console.log(`  Avg messages/client:      ${avgMsgPerClient.toFixed(1)}`);
  console.log(`  Avg entities/client:      ${avgEntitiesPerClient.toFixed(0)}`);
  console.log(`  Avg data/client:          ${(avgBytesPerClient / 1024).toFixed(1)} KB`);
  console.log(`  Avg msgs/client/sec:      ${(avgMsgPerClient / DURATION_S).toFixed(2)}`);
  console.log("");
  console.log("  === Health ===");
  console.log(`  Errors:                   ${totalErrors}`);
  console.log(`  Backpressure warnings:    ${totalBackpressure}`);

  // Per-client distribution
  const msgCounts = allStats.map((s) => s.messageCount).sort((a, b) => a - b);
  const p50 = msgCounts[Math.floor(msgCounts.length * 0.5)];
  const p95 = msgCounts[Math.floor(msgCounts.length * 0.95)];
  const p99 = msgCounts[Math.floor(msgCounts.length * 0.99)];
  const min = msgCounts[0];
  const max = msgCounts[msgCounts.length - 1];

  console.log("");
  console.log("  === Message Distribution (per client) ===");
  console.log(`  min=${min}  p50=${p50}  p95=${p95}  p99=${p99}  max=${max}`);

  // First message latency
  const fmts = allStats.filter((s) => s.firstMessageTime > 0).map((s) => s.firstMessageTime).sort((a, b) => a - b);
  if (fmts.length > 0) {
    console.log("");
    console.log("  === First Message Latency ===");
    console.log(`  min=${fmts[0]}ms  p50=${fmts[Math.floor(fmts.length * 0.5)]}ms  p95=${fmts[Math.floor(fmts.length * 0.95)]}ms  max=${fmts[fmts.length - 1]}ms`);
  }

  // Cleanup
  for (const { ws } of clients) {
    ws.close();
  }

  console.log("\nDone.");
}

runStressTest().catch((err) => {
  console.error("Fatal:", err);
  process.exit(1);
});
