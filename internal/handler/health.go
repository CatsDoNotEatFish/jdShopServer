package handler

import (
	"net/http"
	"time"
)

type HealthHandler struct {
	Version string
}

func NewHealthHandler(version string) *HealthHandler {
	return &HealthHandler{Version: version}
}

func (h *HealthHandler) Check(w http.ResponseWriter, r *http.Request) {
	respondOK(w, map[string]any{
		"status":  "healthy",
		"time":    time.Now().UTC().Format(time.RFC3339),
		"version": h.Version,
	})
}
