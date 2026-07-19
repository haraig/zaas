package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"zaas/api/internal/gen"
	"zaas/api/internal/handler"
)

func newRouter() http.Handler {
	r := chi.NewRouter()
	r.Route("/api/v1", func(r chi.Router) {
		gen.HandlerFromMux(&handler.Server{}, r)
	})
	return r
}

func getBody(t *testing.T, router http.Handler, path string) map[string]any {
	t.Helper()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, path, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET %s: got status %d, want 200. Body: %s", path, w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	return body
}

func getErrorBody(t *testing.T, router http.Handler, path string, wantStatus int) map[string]any {
	t.Helper()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, path, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != wantStatus {
		t.Fatalf("GET %s: got status %d, want %d. Body: %s", path, w.Code, wantStatus, w.Body.String())
	}
	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return body
}

func TestDiceHandler_Default(t *testing.T) {
	t.Parallel()
	body := getBody(t, newRouter(), "/api/v1/dice")
	if body["result"] == nil {
		t.Error("expected result field for count=1")
	}
}

func TestDiceHandler_Multi(t *testing.T) {
	t.Parallel()
	body := getBody(t, newRouter(), "/api/v1/dice?count=3&sides=20")
	if body["results"] == nil {
		t.Error("expected results field for count=3")
	}
	results := body["results"].([]any)
	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}
}

func TestDiceHandler_InvalidSides(t *testing.T) {
	t.Parallel()
	body := getErrorBody(t, newRouter(), "/api/v1/dice?sides=7", http.StatusBadRequest)
	if body["code"] != "INVALID_PARAM" {
		t.Errorf("code: got %v, want INVALID_PARAM", body["code"])
	}
}

func TestCoinHandler(t *testing.T) {
	t.Parallel()
	body := getBody(t, newRouter(), "/api/v1/coin")
	result, ok := body["result"].(string)
	if !ok {
		t.Fatal("result should be a string")
	}
	if result != "heads" && result != "tails" {
		t.Errorf("unexpected coin result: %q", result)
	}
}

func TestNumberHandler_Default(t *testing.T) {
	t.Parallel()
	body := getBody(t, newRouter(), "/api/v1/number")
	if body["result"] == nil {
		t.Error("expected result field")
	}
}

func TestNumberHandler_Float(t *testing.T) {
	t.Parallel()
	body := getBody(t, newRouter(), "/api/v1/number?float=true&count=2")
	if body["results"] == nil {
		t.Error("expected results field for count=2")
	}
}

func TestNumberHandler_InvalidCount(t *testing.T) {
	t.Parallel()
	getErrorBody(t, newRouter(), "/api/v1/number?count=200", http.StatusBadRequest)
}

func TestUUIDHandler_Default(t *testing.T) {
	t.Parallel()
	body := getBody(t, newRouter(), "/api/v1/uuid")
	result, ok := body["result"].(string)
	if !ok {
		t.Fatal("result should be a string")
	}
	if len(result) != 36 {
		t.Errorf("expected standard UUID (36 chars), got %q", result)
	}
}

func TestUUIDHandler_Multi(t *testing.T) {
	t.Parallel()
	body := getBody(t, newRouter(), "/api/v1/uuid?count=5&format=no-dashes")
	results := body["results"].([]any)
	if len(results) != 5 {
		t.Errorf("expected 5 results, got %d", len(results))
	}
	if len(results[0].(string)) != 32 {
		t.Errorf("no-dashes UUID should be 32 chars, got %q", results[0])
	}
}

func TestColorHandler_Default(t *testing.T) {
	t.Parallel()
	body := getBody(t, newRouter(), "/api/v1/color")
	result, ok := body["result"].(string)
	if !ok {
		t.Fatal("result should be a string")
	}
	if len(result) != 7 || result[0] != '#' {
		t.Errorf("expected hex color, got %q", result)
	}
}

func TestColorHandler_InvalidFormat(t *testing.T) {
	t.Parallel()
	getErrorBody(t, newRouter(), "/api/v1/color?format=cmyk", http.StatusBadRequest)
}

func TestCoordinatesHandler_Default(t *testing.T) {
	t.Parallel()
	body := getBody(t, newRouter(), "/api/v1/coordinates")
	if body["result"] == nil {
		t.Error("expected result field")
	}
}

func TestCoordinatesHandler_Multi(t *testing.T) {
	t.Parallel()
	body := getBody(t, newRouter(), "/api/v1/coordinates?count=3")
	if body["results"] == nil {
		t.Error("expected results field for count=3")
	}
}

// TestPasswordHandler_SymbolsDefaultTrue verifies that when no symbols parameter
// is supplied, the handler defaults to symbols=true (matching the OpenAPI spec default).
func TestPasswordHandler_SymbolsDefaultTrue(t *testing.T) {
	t.Parallel()
	body := getBody(t, newRouter(), "/api/v1/password")
	meta, ok := body["meta"].(map[string]any)
	if !ok {
		t.Fatal("meta field missing")
	}
	params, ok := meta["params"].(map[string]any)
	if !ok {
		t.Fatal("meta.params missing")
	}
	symbols, ok := params["symbols"]
	if !ok {
		t.Fatal("meta.params.symbols missing")
	}
	if symbols != true {
		t.Errorf("meta.params.symbols: got %v, want true (spec default)", symbols)
	}
}

// TestWordsHandler_WordsDefaultFour verifies that when no words parameter is
// supplied, the handler defaults to 4 words (matching the OpenAPI spec default).
func TestWordsHandler_WordsDefaultFour(t *testing.T) {
	t.Parallel()
	body := getBody(t, newRouter(), "/api/v1/words")
	meta, ok := body["meta"].(map[string]any)
	if !ok {
		t.Fatal("meta field missing")
	}
	params, ok := meta["params"].(map[string]any)
	if !ok {
		t.Fatal("meta.params missing")
	}
	wordsVal, ok := params["words"]
	if !ok {
		t.Fatal("meta.params.words missing")
	}
	// JSON numbers decode as float64.
	if wordsVal != float64(4) {
		t.Errorf("meta.params.words: got %v, want 4 (spec default)", wordsVal)
	}
}

func TestHealthz(t *testing.T) {
	t.Parallel()
	r := chi.NewRouter()
	r.Get("/healthz", handler.Healthz)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("healthz: got %d, want 200", w.Code)
	}
}

func TestReadyz(t *testing.T) {
	t.Parallel()
	r := chi.NewRouter()
	r.Get("/readyz", handler.Readyz)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("readyz: got %d, want 200", w.Code)
	}
}

func TestOpenAPISpecHandler(t *testing.T) {
	t.Parallel()
	r := chi.NewRouter()
	r.Get("/api/v1/openapi.yaml", handler.OpenAPISpec)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/openapi.yaml", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("openapi.yaml: got %d, want 200", w.Code)
	}
	if w.Header().Get("Content-Type") != "application/yaml" {
		t.Errorf("Content-Type: got %q, want application/yaml", w.Header().Get("Content-Type"))
	}
	body := w.Body.String()
	if len(body) < 100 {
		t.Errorf("openapi.yaml body suspiciously short: %d bytes", len(body))
	}
}
