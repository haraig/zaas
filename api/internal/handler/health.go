package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

type healthBody struct {
	Status string `json:"status"`
}

func Healthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(healthBody{Status: "ok"}); err != nil {
		slog.ErrorContext(r.Context(), "healthz encode error", "error", err)
	}
}

func Readyz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(healthBody{Status: "ready"}); err != nil {
		slog.ErrorContext(r.Context(), "readyz encode error", "error", err)
	}
}
