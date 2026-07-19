package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"zaas/api/internal/gen"
)

// Using a generic local struct keeps response construction independent of
// the oapi-codegen oneOf union types, which are awkward for direct use.
type singleResponse struct {
	Result any      `json:"result"`
	Meta   gen.Meta `json:"meta"`
}

type multiResponse struct {
	Results []any    `json:"results"`
	Meta    gen.Meta `json:"meta"`
}

func WriteSingle(w http.ResponseWriter, r *http.Request, result any, params map[string]any) {
	writeJSON(w, http.StatusOK, singleResponse{
		Result: result,
		Meta:   buildMeta(r, params),
	})
}

func WriteMultiple(w http.ResponseWriter, r *http.Request, results []any, params map[string]any) {
	writeJSON(w, http.StatusOK, multiResponse{
		Results: results,
		Meta:    buildMeta(r, params),
	})
}

// problemTypeURL maps a ZaaS error code to its stable type URI.
func problemTypeURL(code string) string {
	slugs := map[string]string{
		"RATE_LIMITED":        "rate-limited",
		"INVALID_PARAM":       "invalid-param",
		"INTERNAL_ERROR":      "internal-error",
		"INVALID_API_KEY":     "invalid-api-key",
		"SERVICE_UNAVAILABLE": "service-unavailable",
	}
	slug, ok := slugs[code]
	if !ok {
		slug = "unknown"
	}
	return fmt.Sprintf("https://zaas.at/errors/%s", slug)
}

// problemTitle maps a ZaaS error code to its RFC 9457 title.
func problemTitle(code string) string {
	titles := map[string]string{
		"RATE_LIMITED":        "Too Many Requests",
		"INVALID_PARAM":       "Bad Request",
		"INTERNAL_ERROR":      "Internal Server Error",
		"INVALID_API_KEY":     "Unauthorized",
		"SERVICE_UNAVAILABLE": "Service Unavailable",
	}
	title, ok := titles[code]
	if !ok {
		return "Error"
	}
	return title
}

// WriteError writes an RFC 9457 application/problem+json response.
// retryAfter is seconds to wait before retrying; pass 0 to omit.
func WriteError(w http.ResponseWriter, r *http.Request, status int, code, detail string, retryAfter int) {
	pCode := gen.ProblemCode(code)
	instance := r.URL.Path
	reqID := chimiddleware.GetReqID(r.Context())

	body := gen.Problem{
		Type:      problemTypeURL(code),
		Title:     problemTitle(code),
		Status:    status,
		Detail:    detail,
		Instance:  &instance,
		Code:      &pCode,
		RequestId: &reqID,
	}
	if retryAfter > 0 {
		body.RetryAfter = &retryAfter
	}

	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.ErrorContext(r.Context(), "json encode error", "error", err)
	}
}

// buildMeta copies params into a new map so callers cannot mutate the
// embedded response after the fact, then assembles the Meta envelope.
func buildMeta(r *http.Request, params map[string]any) gen.Meta {
	p := make(map[string]interface{}, len(params))
	for k, v := range params {
		p[k] = v
	}
	reqID := chimiddleware.GetReqID(r.Context())
	return gen.Meta{
		Endpoint:  r.URL.Path,
		Timestamp: time.Now().UTC(),
		Params:    p,
		RequestId: &reqID,
	}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Error("json encode error", "error", err)
	}
}

// defaultPtr returns *p if p is non-nil, otherwise def.
func defaultPtr[T any](p *T, def T) T {
	if p == nil {
		return def
	}
	return *p
}

// writeResults writes a single-result or multi-result response depending on
// the length of results. It avoids repeating the []any conversion boilerplate
// in every handler.
func writeResults[T any](w http.ResponseWriter, r *http.Request, results []T, params map[string]any) {
	if len(results) == 1 {
		WriteSingle(w, r, results[0], params)
		return
	}
	anys := make([]any, len(results))
	for i, v := range results {
		anys[i] = v
	}
	WriteMultiple(w, r, anys, params)
}
