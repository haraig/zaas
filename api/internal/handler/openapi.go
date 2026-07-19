package handler

import (
	_ "embed"
	"net/http"
)

//go:embed openapi.yaml
var openapiSpec []byte

func OpenAPISpec(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/yaml")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(openapiSpec)
}

// GetOpenAPISpec implements gen.ServerInterface so the generated wrapper can
// serve the spec route through the standard handler pipeline.
func (s *Server) GetOpenAPISpec(w http.ResponseWriter, r *http.Request) {
	OpenAPISpec(w, r)
}
