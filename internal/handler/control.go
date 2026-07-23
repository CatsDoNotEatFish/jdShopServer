package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"jdShopServer/internal/middleware"
	"jdShopServer/internal/model"
	"jdShopServer/internal/service"
)

type ControlHandler struct {
	hub *service.ControlHub
}

func NewControlHandler(hub *service.ControlHub) *ControlHandler {
	return &ControlHandler{hub: hub}
}

func (h *ControlHandler) Stream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	// The normal server write timeout must not terminate a long-lived stream.
	_ = http.NewResponseController(w).SetWriteDeadline(time.Time{})
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	events, unsubscribe := h.hub.Subscribe(middleware.GetUserID(r.Context()))
	defer unsubscribe()

	if err := writeControlEvent(w, model.ControlEvent{
		Type:     "connected",
		IssuedAt: time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		return
	}
	flusher.Flush()

	keepalive := time.NewTicker(15 * time.Second)
	defer keepalive.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case event, ok := <-events:
			if !ok {
				return
			}
			if err := writeControlEvent(w, event); err != nil {
				return
			}
			flusher.Flush()
		case <-keepalive.C:
			if _, err := fmt.Fprint(w, ": keepalive\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func writeControlEvent(w http.ResponseWriter, event model.ControlEvent) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "event: control\ndata: %s\n\n", payload)
	return err
}
