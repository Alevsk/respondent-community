package main

import (
	"encoding/json"
	"net/http"
)

// frontendConfigPayload builds the client-safe runtime config the SPA fetches from
// GET /config.json. Empty values are omitted so the frontend's fallback chain
// (runtime config -> build-time env -> default) stays intact.
func frontendConfigPayload(cfg FrontendConfig) map[string]string {
	payload := make(map[string]string)
	if cfg.CesiumIonToken != "" {
		payload["cesiumIonToken"] = cfg.CesiumIonToken
	}
	return payload
}

// frontendConfigHandler serves the runtime frontend config as JSON. The payload is
// computed once from config (env is static for the process lifetime).
func frontendConfigHandler(cfg FrontendConfig) http.HandlerFunc {
	body, _ := json.Marshal(frontendConfigPayload(cfg))
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(body)
	}
}
