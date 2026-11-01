package main

import (
	"encoding/json"
	"net/http"
	"sync/atomic"
	"strings"
	"time"
)

type Counters struct {
	Received   atomic.Int64
	Accepted   atomic.Int64
	BadMessage atomic.Int64
	Duplicates atomic.Int64
	Late       atomic.Int64
	Reconnects atomic.Int64
	Dropped    atomic.Int64
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func newMux(tr *DeviceTracker, c *Counters, hub *Hub) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	mux.HandleFunc("GET /api/devices", func(w http.ResponseWriter, r *http.Request) {
		views := []DeviceView{}
		for _, id := range tr.IDs() {
			if v, ok := tr.View(id); ok {
				views = append(views, v)
			}
		}
		writeJSON(w, http.StatusOK, views)
	})
	mux.HandleFunc("/api/devices/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/history") {
			id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/devices/"), "/history")

			window := r.URL.Query().Get("window")
			if window == "" {
				window = "15m"
			}

			duration, err := time.ParseDuration(window)
			if err != nil || duration <= 0 || duration > 15*time.Minute {
				http.Error(w, "invalid window (maximum 15m)", http.StatusBadRequest)
				return
			}

			history := tr.History(id, time.Now().Add(-duration))
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(history)
			return
		}

	})
	mux.HandleFunc("GET /api/devices/{id}", func(w http.ResponseWriter, r *http.Request) {
		v, ok := tr.View(r.PathValue("id"))
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown device"})
			return
		}
		writeJSON(w, http.StatusOK, v)
	})

	mux.HandleFunc("GET /stats", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]int64{
			"received":   c.Received.Load(),
			"accepted":   c.Accepted.Load(),
			"badMessage": c.BadMessage.Load(),
			"dedupd":     c.Duplicates.Load(),
			"late":       c.Late.Load(),
			"reconnects": c.Reconnects.Load(),
			"dropped":    c.Dropped.Load(),
		})
	})
	mux.HandleFunc("GET /api/stream", hub.ServeStream)

	return mux
}
