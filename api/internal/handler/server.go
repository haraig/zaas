// Package handler implements the ZaaS HTTP API handlers and routing.
package handler

import (
	"net/http"

	"zaas/api/internal/config"
	"zaas/api/internal/email"
	"zaas/api/internal/gen"
	"zaas/api/internal/service"
	"zaas/api/internal/store"
)

var _ gen.ServerInterface = (*Server)(nil)

// Deps holds the optional dependencies for Server. Fields may be nil if the
// corresponding feature (auth, email) is disabled.
type Deps struct {
	Store  store.ClientStore
	Tokens store.TokenStore
	Email  email.Sender
}

// Server implements gen.ServerInterface by delegating to the service layer.
// All fields are unexported; use New to construct.
type Server struct {
	store  store.ClientStore
	tokens store.TokenStore
	email  email.Sender
	config config.Config
}

// New creates a Server with the given configuration and dependencies.
func New(cfg config.Config, deps Deps) *Server {
	return &Server{
		config: cfg,
		store:  deps.Store,
		tokens: deps.Tokens,
		email:  deps.Email,
	}
}

func (s *Server) RollDice(w http.ResponseWriter, r *http.Request, params gen.RollDiceParams) {
	sides := defaultPtr(params.Sides, gen.RollDiceParamsSides(6))
	count := defaultPtr(params.Count, 1)

	results, err := service.RollDice(int(sides), count)
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_PARAM", err.Error(), 0)
		return
	}

	p := map[string]any{"sides": int(sides), "count": count}
	writeResults(w, r, results, p)
}

func (s *Server) FlipCoin(w http.ResponseWriter, r *http.Request) {
	result := service.FlipCoin()
	WriteSingle(w, r, result, map[string]any{})
}

func (s *Server) GenerateNumber(w http.ResponseWriter, r *http.Request, params gen.GenerateNumberParams) {
	lo := defaultPtr(params.Min, 0)
	hi := defaultPtr(params.Max, 100)
	count := defaultPtr(params.Count, 1)
	useFloat := defaultPtr(params.Float, false)

	p := map[string]any{"min": lo, "max": hi, "count": count, "float": useFloat}

	if useFloat {
		results, err := service.RandomFloats(float64(lo), float64(hi), count)
		if err != nil {
			WriteError(w, r, http.StatusBadRequest, "INVALID_PARAM", err.Error(), 0)
			return
		}
		writeResults(w, r, results, p)
		return
	}

	results, err := service.RandomInts(lo, hi, count)
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_PARAM", err.Error(), 0)
		return
	}
	writeResults(w, r, results, p)
}

func (s *Server) GenerateUUID(w http.ResponseWriter, r *http.Request, params gen.GenerateUUIDParams) {
	count := defaultPtr(params.Count, 1)
	format := string(defaultPtr(params.Format, "standard"))

	results, err := service.GenerateUUIDs(count, format)
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_PARAM", err.Error(), 0)
		return
	}

	writeResults(w, r, results, map[string]any{"count": count, "format": format})
}

func (s *Server) GenerateColor(w http.ResponseWriter, r *http.Request, params gen.GenerateColorParams) {
	count := defaultPtr(params.Count, 1)
	format := string(defaultPtr(params.Format, "hex"))

	results, err := service.RandomColors(count, format)
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_PARAM", err.Error(), 0)
		return
	}

	writeResults(w, r, results, map[string]any{"count": count, "format": format})
}

func (s *Server) GeneratePassword(w http.ResponseWriter, r *http.Request, params gen.GeneratePasswordParams) {
	length := defaultPtr(params.Length, 16)
	uppercase := defaultPtr(params.Uppercase, true)
	lowercase := defaultPtr(params.Lowercase, true)
	digits := defaultPtr(params.Digits, true)
	symbols := defaultPtr(params.Symbols, true)
	count := defaultPtr(params.Count, 1)

	results, err := service.GeneratePasswords(count, length, uppercase, lowercase, digits, symbols)
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_PARAM", err.Error(), 0)
		return
	}

	p := map[string]any{
		"length": length, "uppercase": uppercase, "lowercase": lowercase,
		"digits": digits, "symbols": symbols, "count": count,
	}
	writeResults(w, r, results, p)
}

func (s *Server) GenerateWords(w http.ResponseWriter, r *http.Request, params gen.GenerateWordsParams) {
	style := string(defaultPtr(params.Style, "random"))
	words := defaultPtr(params.Words, 4)
	// Per-style separator defaults
	separator := "-"
	switch style {
	case "docker":
		separator = "_"
	case "ubuntu":
		separator = " "
	}
	if params.Separator != nil {
		separator = *params.Separator
	}
	// Per-style capitalize defaults
	capitalize := style == "ubuntu"
	if params.Capitalize != nil {
		capitalize = *params.Capitalize
	}
	count := defaultPtr(params.Count, 1)

	results, err := service.RandomWords(service.WordsParams{
		Style:      style,
		Words:      words,
		Separator:  separator,
		Capitalize: capitalize,
		Count:      count,
	})
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_PARAM", err.Error(), 0)
		return
	}

	p := map[string]any{
		"style": style, "words": words, "separator": separator,
		"capitalize": capitalize, "count": count,
	}
	writeResults(w, r, results, p)
}

func (s *Server) GenerateLorem(w http.ResponseWriter, r *http.Request, params gen.GenerateLoremParams) {
	count := defaultPtr(params.Count, 1)
	paragraphs := defaultPtr(params.Paragraphs, 1)
	sentences := defaultPtr(params.Sentences, 0)

	results, err := service.RandomLorem(count, paragraphs, sentences)
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_PARAM", err.Error(), 0)
		return
	}

	writeResults(w, r, results, map[string]any{"count": count, "paragraphs": paragraphs, "sentences": sentences})
}

func (s *Server) GenerateSSHKey(w http.ResponseWriter, r *http.Request, params gen.GenerateSSHKeyParams) {
	keyType := string(defaultPtr(params.Type, "ed25519"))
	bits := int(defaultPtr(params.Bits, 2048))
	comment := defaultPtr(params.Comment, "")
	count := defaultPtr(params.Count, 1)

	pairs, err := service.GenerateSSHKeys(count, keyType, bits, comment)
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_PARAM", err.Error(), 0)
		return
	}

	results := make([]gen.SSHKey, len(pairs))
	for i, kp := range pairs {
		results[i] = gen.SSHKey{
			PrivateKey:  kp.PrivateKey,
			PublicKey:   kp.PublicKey,
			Fingerprint: kp.Fingerprint,
		}
	}

	p := map[string]any{"type": keyType, "bits": bits, "comment": comment, "count": count}
	writeResults(w, r, results, p)
}

func (s *Server) GenerateCoordinates(w http.ResponseWriter, r *http.Request, params gen.GenerateCoordinatesParams) {
	count := defaultPtr(params.Count, 1)
	landOnly := defaultPtr(params.LandOnly, false)

	coords, err := service.RandomCoordinates(count, landOnly)
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_PARAM", err.Error(), 0)
		return
	}

	results := make([]gen.Coordinate, len(coords))
	for i, c := range coords {
		results[i] = gen.Coordinate{Lat: c.Lat, Lon: c.Lon}
	}

	p := map[string]any{"count": count, "land_only": landOnly}
	writeResults(w, r, results, p)
}
