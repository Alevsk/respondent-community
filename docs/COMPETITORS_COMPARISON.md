# Competitive Assessment: Respondent vs. Gridline, Crucix, World Monitor

**Date:** 2026-04-10
**Sources Analyzed:**
- [Gridline.world](https://gridline.world/) — Real-time geopolitical intelligence dashboard (SaaS, $24/mo Pro)
- [Crucix](https://github.com/calesthio/Crucix) — Self-hosted OSINT terminal (AGPL-3.0, ~8.6k stars)
- [World Monitor](https://github.com/koala73/worldmonitor) — Open-source global intelligence dashboard

---

## Platform Comparison Matrix

| Capability | **Respondent** | **Gridline** | **Crucix** |
|---|---|---|---|
| **Map Engine** | Cesium 3D Globe | Leaflet 2D | Globe.GL 3D + D3 flat |
| **Data Sources** | 38+ declarative YAML | 60+ APIs, 30+ layers | 29 source modules |
| **Backend** | Go (gRPC/WS/HTTP) | Unknown (likely Node) | Node.js + Express only |
| **Frontend** | React 18 + TS + MUI | React + Vite + Tailwind | Single HTML file (vanilla JS) |
| **State** | Zustand + React Query | TanStack Query | Global `D` variable |
| **AI** | Entity enrichment, NL search, insights | AI analyst (Pro) | 9 LLM providers, trade ideas |
| **Alerts** | Notification panel (in-app) | None visible | Telegram + Discord bots |
| **Auth** | Keycloak (deferred) | Clerk | None (self-hosted) |
| **Deploy** | Docker (server + feeder) | SaaS | `npm install && npm run dev` |
| **License** | MIT | SaaS ($24/mo Pro) | AGPL-3.0 (8.6k stars) |
| **Delta Detection** | None | None | Sweep-over-sweep with severity |
| **Recording/Media** | Yes (aspect ratios, filters) | No | No |
| **CCTV** | Yes | No | No |
| **Orbital/SGP4** | Yes | No | Basic CelesTrak |
| **Historical Data** | Yes (48h lookback, time range) | No | 3-run hot storage |
| **Mobile** | Yes (responsive + drawer) | Responsive breakpoints | Flat map fallback |

---

## Competitor Deep Dives

### Gridline.world

**What it is:** A real-time geopolitical intelligence dashboard aggregating 30+ OSINT data layers into a single interactive map. Targets analysts, researchers, journalists. Built in ~2 weeks with Claude Code. Founded 2025.

**Core Modules (17 free, more in Pro):**
1. Conflict Tracking (GDELT)
2. Satellite Fire Detection (NASA FIRMS)
3. GPS Jamming/Spoofing Detection (Pro)
4. Nuclear Facilities Monitor (370+ plants)
5. Live Vessel Tracking / AIS (Pro)
6. Earthquake/Volcano/Weather Alerts (USGS, NOAA, NWS)
7. Military Bases and Chokepoints
8. Undersea Cables and Pipeline Infrastructure
9. Commodity, Forex, Crypto, Stock Markets (Yahoo Finance)
10. Country Intelligence Profiles (198 countries, World Bank, IMF)
11. AI-Powered Intelligence Analyst (Pro)
12. Prediction Markets / Sentiment (Polymarket)
13. Satellite Imagery (Pro)

**Design Language:**
- Background: `#0a0a0a` base, `#111111` canvas, `#141414` inset, `#181818` panels
- Typography: JetBrains Mono (primary), Inter (base)
- Primary accent: `#00ffaa` (mint green)
- Secondary: `#00ccff` (cyan blue)
- Danger scale: `#ff2222` / `#ff6600` / `#ffaa00` / `#00cc66`
- Panels: `rgba(x,x,x,0.85)` with `backdrop-filter: blur(8px/12px/24px)`
- Animations: fadeInUp, scanline, glowPulse, cursor-blink

**Tech Stack:** Vite + React SPA, Tailwind CSS, Clerk auth, TanStack Query, Leaflet maps, CARTO basemap tiles, PWA-capable

**Data Sources:** GDELT, NASA FIRMS, USGS, NWS/NOAA, Wikidata, AIS, Yahoo Finance, Polymarket, World Bank, IMF, GPS jamming feeds, Cloudflare Radar

**Pricing:** Free tier (17 modules, 20+ layers) / Pro $24/month (AI analyst, AIS, satellite imagery, GPS jamming)

---

### Crucix

**What it is:** A self-hosted OSINT terminal. "27 sources. One command. Zero cloud." Everything runs locally with no external cloud dependency. Watches geopolitical, economic, environmental, and space domains. Alerts via Telegram/Discord.

**Key Features:**
1. 27-source parallel sweep (`Promise.allSettled()`, 30s per-source timeout)
2. 3D WebGL globe (Globe.GL) + D3 flat map toggle
3. 9 marker types (fires, aircraft, radiation, maritime, conflicts, satellites, SDR, nuclear, weather)
4. Delta engine (sweep-over-sweep change detection with severity classification)
5. Multi-tier alerts (FLASH/PRIORITY/ROUTINE) via Telegram + Discord
6. 9 LLM providers (Claude, OpenAI, Gemini, Grok, OpenRouter, MiniMax, Mistral, Ollama, Codex)
7. Trade/action ideas with confidence ratings
8. Two-way bot interaction (/status, /sweep, /brief, /portfolio, /alerts)
9. Performance modes (VISUALS FULL / LITE)
10. i18n (English, French)

**Data Sources (29, organized by tier):**

| Tier | Category | Sources |
|------|----------|---------|
| 1 | OSINT & Geopolitical | GDELT, OpenSky, NASA FIRMS, Maritime AIS, Safecast, ACLED, ReliefWeb, WHO, OFAC, OpenSanctions, ADS-B Exchange |
| 2 | Economic | FRED, US Treasury, BLS, EIA, GSCPI, USAspending, UN Comtrade |
| 3 | Environment & Social | NOAA, EPA RadNet, USPTO patents, Bluesky, Reddit sentiment, Telegram channels, KiwiSDR |
| 4 | Space | CelesTrak satellites |
| 5 | Markets | Yahoo Finance (indexes, crypto, commodities, yields, VIX) |
| 6 | Cyber | CISA-KEV, Cloudflare Radar |

**Architecture:** Node.js 22+, Express v5 (only dependency), single HTML file dashboard, no database (JSON on disk), pure ESM, raw `fetch()` for everything (no SDKs).

**Notable Patterns:**
- Delta engine with semantic deduplication (SHA256 content hashing)
- Alert fatigue prevention (cooldown decay + per-tier hourly caps)
- Hot/cold memory architecture (atomic writes, 3-run hot storage)
- Data compaction for LLM context (~8KB budget)
- "Signals not conclusions" UX (glossary with "this means / does NOT mean")
- Factory pattern for LLM providers
- Geo-tagging via 150+ location keywords with coordinate randomization

---

## What We Can Borrow

### From Gridline

| Idea | Priority | Rationale |
|---|---|---|
| **GPS Jamming/Spoofing Layer** | HIGH | Extremely topical (Ukraine, Middle East). GNSS disruption data is a unique differentiator. New `sources.d/` YAML source. |
| **Country Intelligence Profiles** | HIGH | 198 countries with GDP, inflation, trade balance, governance, population. Render as entity detail tabs. World Bank + IMF free APIs. |
| **Prediction Markets (Polymarket)** | MEDIUM | Cross-referencing event probabilities with OSINT data. Could be indicator or layer. |
| **Commodity/Forex/Crypto Markets** | MEDIUM | Yahoo Finance as indicator source alongside existing IndicatorHUD. |
| **Nuclear Facilities Monitor** | MEDIUM | 370+ nuclear plants as static overlay. Simple YAML source from IAEA. |
| **Server-Rendered SEO Pages** | LOW | Relevant if we go SaaS. Not a priority now. |

### From Crucix

| Idea | Priority | Rationale |
|---|---|---|
| **Delta/Change Detection Engine** | HIGH | Sweep-over-sweep deltas with severity classification. Build into feeder for meaningful notifications. |
| **Multi-Tier Alert System** | HIGH | Telegram + Discord delivery with cooldowns, hourly caps, content deduplication. |
| **Alert Fatigue Prevention** | HIGH | Content hashing + cooldown decay + per-tier caps. Essential at scale. |
| **"Signals Not Conclusions" UX** | MEDIUM | Glossary with "this means / does NOT mean" for indicators. Add to IndicatorHUD. |
| **Trade/Action Ideas from AI** | MEDIUM | 5-8 actionable suggestions with citations and confidence ratings. Extends our AI system. |
| **Economic Data Sources** | MEDIUM | FRED, BLS, EIA, Treasury, GSCPI, UN Comtrade, USAspending. |
| **Cybersecurity Layer** | MEDIUM | CISA-KEV + Cloudflare Radar. We have internet_outages but not CISA-KEV. |
| **RSS/OSINT News Feed** | MEDIUM | 19 RSS feeds + 17 Telegram channels, geo-tagged on globe. |
| **Defense Contract Monitoring** | MEDIUM | USAspending.gov data. Relevant for target audience. |

---

## Sources We're Missing (Gap Analysis)

| Source | Found In | Difficulty | Value |
|---|---|---|---|
| GPS Jamming/Spoofing | Gridline | Medium | Very High |
| Country Profiles (World Bank/IMF) | Gridline | Easy | High |
| FRED Economic Data | Crucix | Easy | High |
| GDELT News Events | Gridline, Crucix | Easy | High |
| Prediction Markets (Polymarket) | Gridline | Easy | Medium |
| Yahoo Finance Markets | Gridline, Crucix | Easy | Medium |
| Defense Contracts (USAspending) | Crucix | Easy | Medium |
| CISA-KEV Vulnerabilities | Crucix | Easy | Medium |
| Nuclear Facilities (IAEA) | Gridline | Easy | Medium |
| WHO Disease Outbreaks | Crucix | Easy | Medium |
| UN Comtrade (Trade Flows) | Crucix | Easy | Medium |
| OFAC/OpenSanctions | Crucix | Easy | Medium |
| US Treasury Data | Crucix | Easy | Low |
| BLS Employment Data | Crucix | Easy | Low |
| Supply Chain Index (GSCPI) | Crucix | Easy | Low |
| Patent Filings (USPTO) | Crucix | Easy | Low |
| KiwiSDR Receivers | Crucix | Easy | Low |

---

## Features We're Missing (Gap Analysis)

| Feature | Found In | Effort | Impact |
|---|---|---|---|
| **Delta/Change Detection** | Crucix | Medium | Very High |
| **External Alert Delivery** (Telegram/Discord) | Crucix | Medium | Very High |
| **Alert Fatigue Prevention** | Crucix | Low | High |
| **Market Indicators Panel** | Gridline, Crucix | Low | High |
| **Country Profile View** | Gridline | Medium | High |
| **OSINT News Feed** (geo-tagged) | Crucix | Medium | High |
| **Indicator Glossary/Help** | Crucix | Low | Medium |
| **Flat Map Toggle** | Crucix | Low | Medium |
| **LLM Trade/Action Ideas** | Crucix | Low | Medium |
| **Dual Bot Interaction** | Crucix | Medium | Medium |

---

## What We Already Do Better

1. **3D Globe (Cesium)** — far superior to Leaflet 2D (Gridline) or Globe.GL (Crucix). Terrain, 3D buildings, orbital mechanics.
2. **Declarative Source System** — YAML-based source definitions vs. hand-coded modules. Zero code changes to add sources.
3. **Enterprise Backend** — gRPC + PostgreSQL + NATS + Valkey vs. JSON-on-disk (Crucix). Production-grade scalability.
4. **Dead-Reckoning Interpolation** — smooth entity motion between updates. No competitor has this.
5. **Recording/Media Production** — aspect ratio overlays, CRT/NVG/FLIR post-processing. Completely unique.
6. **CCTV Integration** — camera feed mesh with calibration. No competitor has this.
7. **Historical Time Travel** — 48h lookback with custom range picker. Crucix keeps 3 runs; Gridline has none.
8. **Viewport-Based Progressive Loading** — spatial H3 indexing with on-demand backfill. Far more scalable.
9. **Entity Detail System** — tabbed panels with AI enrichment, observation history, metadata.
10. **Mobile-First UI** — responsive drawers, bottom nav, safe areas. More polished than competitors.

---

## Recommended Roadmap (Priority Order)

1. **Delta Detection Engine** — build change detection into feeder pipeline. Classify entity state changes as escalated/deescalated/new.
2. **GPS Jamming/Spoofing Source** — find reliable GNSS disruption data feed, add as YAML source.
3. **External Alerts (Telegram/Discord)** — extend notification system to push externally with fatigue prevention.
4. **Country Intelligence Profiles** — World Bank + IMF APIs, rendered as entity detail tab or panel.
5. **Economic Indicator Sources** — FRED, EIA, Yahoo Finance as indicator sources for IndicatorHUD.
6. **GDELT News Events** — geo-tagged news layer providing narrative context.
7. **Nuclear Facilities Layer** — static IAEA dataset as YAML source.
8. **Market Indicators Panel** — VIX, yields, commodities, crypto as compact HUD widget.
9. **Cybersecurity Layer** — CISA-KEV + existing Cloudflare Radar.
10. **Indicator Contextual Help** — "what this means / doesn't mean" tooltips on all gauges.
