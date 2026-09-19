// Minimal WS listener that captures ai_insight notifications from the server.
// Proof tool for Phase D: confirms the analysis engine pushes ai_insight WS
// messages end-to-end. A client with no notification_filter receives all.
const WebSocket = require("ws");
const URL = process.env.WS_URL || "ws://localhost:8090/ws";
const ws = new WebSocket(URL);
const counts = {};
ws.on("open", () => console.log("connected:", URL, "— waiting for ai_insight..."));
ws.on("message", (data) => {
  let msg;
  try { msg = JSON.parse(data.toString()); } catch { return; }
  const t = msg.type || "unknown";
  counts[t] = (counts[t] || 0) + 1;
  if (t === "ai_insight") {
    console.log("=== GOT ai_insight #" + counts[t] + " ===");
    console.log(JSON.stringify(msg).slice(0, 1200));
  }
});
ws.on("error", (e) => console.log("ws error:", e.message));
ws.on("close", () => console.log("closed. type counts:", JSON.stringify(counts)));
// Periodic heartbeat so we can see it's alive and what message types arrive.
setInterval(() => console.log("alive; type counts:", JSON.stringify(counts)), 30000);
