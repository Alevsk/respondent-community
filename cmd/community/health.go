package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"
)

// healthHandler always returns 200 OK — the process is alive.
func healthHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprint(w, "OK")
}

// readinessHandler checks that SQLite is responsive.
type readinessHandler struct {
	db *sql.DB
}

func (h *readinessHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := h.db.PingContext(ctx); err != nil {
		http.Error(w, "not ready", http.StatusServiceUnavailable)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprint(w, "OK")
}
