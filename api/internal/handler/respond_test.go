package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"zaas/api/internal/handler"
)

func TestWriteSingle(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	handler.WriteSingle(w, r("/api/v1/coin"), "heads", map[string]any{})
	res := w.Result()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want 200", res.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["result"] != "heads" {
		t.Errorf("result: got %v, want heads", body["result"])
	}
	meta, ok := body["meta"].(map[string]any)
	if !ok {
		t.Fatal("meta field missing or wrong type")
	}
	if meta["endpoint"] != "/api/v1/coin" {
		t.Errorf("meta.endpoint: got %v, want /api/v1/coin", meta["endpoint"])
	}
	if _, ok := meta["timestamp"]; !ok {
		t.Error("meta.timestamp missing")
	}
}

func TestWriteMultiple(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	results := []any{"a", "b", "c"}
	handler.WriteMultiple(w, r("/api/v1/uuid"), results, map[string]any{"count": 3})
	var body map[string]any
	if err := json.NewDecoder(w.Result().Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := body["results"]; !ok {
		t.Error("results field missing")
	}
	if _, ok := body["result"]; ok {
		t.Error("result (singular) should not be present for multiple")
	}
}

func TestWriteError(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	handler.WriteError(w, r("/api/v1/dice"), http.StatusBadRequest, "INVALID_PARAM", "sides must be one of 4,6,8,10,12,20,100", 0)
	res := w.Result()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); ct != "application/problem+json" {
		t.Errorf("Content-Type: got %q, want application/problem+json", ct)
	}
	var body map[string]any
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["code"] != "INVALID_PARAM" {
		t.Errorf("code: got %v, want INVALID_PARAM", body["code"])
	}
	if body["type"] == nil || body["type"] == "" {
		t.Error("type field missing or empty")
	}
	if body["title"] == nil || body["title"] == "" {
		t.Error("title field missing or empty")
	}
	if body["detail"] == nil || body["detail"] == "" {
		t.Error("detail field missing or empty")
	}
	if body["status"] == nil {
		t.Error("status field missing")
	}
	if body["instance"] == nil || body["instance"] == "" {
		t.Error("instance field missing or empty")
	}
}

func TestWriteErrorRetryAfter(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	handler.WriteError(w, r("/api/v1/dice"), http.StatusTooManyRequests, "RATE_LIMITED", "Rate limit exceeded", 42)
	var body map[string]any
	if err := json.NewDecoder(w.Result().Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["retry_after"] == nil {
		t.Error("retry_after field missing")
	}
	if body["retry_after"] != float64(42) {
		t.Errorf("retry_after: got %v, want 42", body["retry_after"])
	}
}

func r(path string) *http.Request {
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, path, nil)
	return req
}

var _ = time.Now
