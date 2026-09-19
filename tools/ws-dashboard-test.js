#!/usr/bin/env node
/**
 * Dashboard WebSocket diagnostic — tests global mode + pagination.
 * Verifies that spatial layers return data (no viewport filtering),
 * large layers return cursors for pagination, and indicator layers work.
 */
const WebSocket = require("ws");
const ws = new WebSocket("ws://localhost:22012/ws");

const spatialLayers = ["earthquakes", "ems_activations", "flights_military", "ocean_buoys", "radiosondes", "satellites"];
const nonSpatialLayers = ["air_quality", "fires_active", "ships", "lightning", "subsea_cables", "volcanoes"];
const indicatorLayers = ["markets", "space_weather"];

const results = {};

ws.on("open", () => {
  console.log("=== Dashboard WebSocket Diagnostic (Global Mode + Pagination) ===\n");

  const allLayers = [
    ...spatialLayers, ...nonSpatialLayers, ...indicatorLayers,
  ];

  for (const l of allLayers) {
    results[l] = { entities: 0, total: null, cursor: null, msgs: 0, indicators: 0 };
    // Subscribe in global mode with small limit to test pagination
    ws.send(JSON.stringify({
      type: "subscribe",
      layer_id: l,
      data: { mode: "global", limit: 10 },
    }));
  }

  // After 5 seconds, test pagination by requesting next page for any layer with a cursor
  setTimeout(() => {
    for (const l of allLayers) {
      const r = results[l];
      if (r.cursor) {
        console.log("  Requesting page 2 for " + l + " (cursor: " + r.cursor.slice(0, 20) + "...)");
        ws.send(JSON.stringify({
          type: "page",
          layer_id: l,
          data: { cursor: r.cursor, limit: 10 },
        }));
      }
    }
  }, 5000);
});

ws.on("message", (data) => {
  try {
    const msg = JSON.parse(data.toString());
    const lid = msg.layer_id;
    if (!lid || !results[lid]) return;

    results[lid].msgs++;
    if (msg.type === "layer.snapshot") {
      const entities = msg.data?.entities?.length || 0;
      results[lid].entities += entities;
      if (msg.data?.total !== undefined) results[lid].total = msg.data.total;
      if (msg.data?.cursor) results[lid].cursor = msg.data.cursor;
    } else if (msg.type === "indicator.update") {
      results[lid].indicators++;
    }
  } catch (e) { /* ignore */ }
});

setTimeout(() => {
  console.log("PAGINATION RESULTS (global mode, limit=10):");
  for (const l of [...spatialLayers, ...nonSpatialLayers]) {
    const r = results[l];
    const totalStr = r.total !== null ? String(r.total) : "N/A";
    const cursorStr = r.cursor ? "YES" : "no";
    const status = r.entities > 0 ? "  ✓" : "  ✗ EMPTY";
    console.log(
      "  " + l.padEnd(24) +
      "loaded=" + String(r.entities).padEnd(6) +
      "total=" + totalStr.padEnd(10) +
      "cursor=" + cursorStr.padEnd(5) +
      status
    );
  }

  console.log("\nINDICATOR LAYERS:");
  for (const l of indicatorLayers) {
    const r = results[l];
    const status = r.indicators > 0 ? "  ✓ indicator" : "  ✗ NO DATA";
    console.log("  " + l.padEnd(24) + "indicators=" + r.indicators + status);
  }

  console.log("\n=== VERIFICATION ===");
  const spatialWithData = spatialLayers.filter((l) => results[l].entities > 0);
  const withCursors = [...spatialLayers, ...nonSpatialLayers].filter((l) => results[l].cursor);
  console.log("Spatial layers with data: " + spatialWithData.length + "/" + spatialLayers.length +
    (spatialWithData.length === spatialLayers.length ? " ✓ ALL FIXED" : " ✗ STILL BROKEN"));
  console.log("Layers with pagination cursors: " + withCursors.length +
    (withCursors.length > 0 ? " ✓ PAGINATION WORKING" : " ✗ NO CURSORS"));

  ws.close();
  process.exit(0);
}, 12000);
