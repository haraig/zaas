package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"zaas/api/internal/gen"
	"zaas/api/internal/service"
	"zaas/api/internal/store"
)

// maxEmailLength is the RFC 5321 limit for the entire email address.
const maxEmailLength = 254

// Sentinel errors for email validation.
var (
	errEmailTooLong       = errors.New("email exceeds maximum length of 254 characters")
	errEmailInvalidChars  = errors.New("email contains invalid characters")
	errEmailInvalidFormat = errors.New("invalid email address")
)

// validateEmail parses and validates an email address.
// It rejects values containing CR/LF, angle brackets, or exceeding RFC 5321 limits.
func validateEmail(raw string) (string, error) {
	if len(raw) > maxEmailLength {
		return "", errEmailTooLong
	}
	if strings.ContainsAny(raw, "\r\n<>") {
		return "", errEmailInvalidChars
	}
	addr, err := mail.ParseAddress(raw)
	if err != nil {
		return "", errEmailInvalidFormat
	}
	return addr.Address, nil
}

func (s *Server) AuthRegister(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		WriteError(w, r, http.StatusServiceUnavailable, "INTERNAL_ERROR", "Auth endpoints require database configuration", 0)
		return
	}

	var req gen.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_PARAM", "Invalid request body", 0)
		return
	}

	emailAddr, err := validateEmail(string(req.Email))
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_PARAM", "Invalid email address", 0)
		return
	}

	if len(req.DisplayName) == 0 || len(req.DisplayName) > 100 {
		WriteError(w, r, http.StatusBadRequest, "INVALID_PARAM", "display_name must be 1-100 characters", 0)
		return
	}

	// Check if email already registered and verified - return 202 to prevent enumeration.
	// Distinguish a genuine DB error from "not found" so we can surface real failures.
	existing, err := s.store.GetClientByEmail(r.Context(), emailAddr)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		slog.ErrorContext(r.Context(), "auth register: GetClientByEmail failed", "error", err)
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Registration failed", 0)
		return
	}
	if existing != nil && existing.VerifiedAt != nil && existing.RevokedAt == nil {
		writeAccepted(r.Context(), w)
		return
	}

	// Create unverified client if not exists
	if existing == nil {
		if err := s.store.CreateClient(r.Context(), emailAddr, req.DisplayName); err != nil {
			WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Registration failed", 0)
			return
		}
	}

	// Create verification token
	token := service.GenerateToken()
	tokenHash := service.HashKey(token)
	expiresAt := time.Now().Add(24 * time.Hour)
	if err := s.tokens.CreateToken(r.Context(), emailAddr, tokenHash, store.TokenTypeRegistration, expiresAt); err != nil {
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Registration failed", 0)
		return
	}

	if s.email != nil {
		verifyURL := fmt.Sprintf("%s/verify?token=%s&type=registration", s.config.BaseURL, token)
		if err := s.email.SendVerification(r.Context(), emailAddr, verifyURL); err != nil {
			slog.ErrorContext(r.Context(), "auth register: SendVerification failed", "error", err)
		}
	}

	writeAccepted(r.Context(), w)
}

func (s *Server) AuthVerify(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		WriteError(w, r, http.StatusServiceUnavailable, "INTERNAL_ERROR", "Auth endpoints require database configuration", 0)
		return
	}

	var req gen.VerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_PARAM", "Invalid request body", 0)
		return
	}

	tokenType := store.TokenType(req.Type)

	if tokenType != store.TokenTypeRegistration && tokenType != store.TokenTypeReissue {
		WriteError(w, r, http.StatusBadRequest, "INVALID_PARAM", "Invalid token type", 0)
		return
	}

	tokenHash := service.HashKey(req.Token)
	token, err := s.tokens.ConsumeToken(r.Context(), tokenHash, tokenType)
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_PARAM", "Token is invalid, expired, or already used", 0)
		return
	}

	if tokenType == store.TokenTypeReissue {
		if err := s.store.RevokeClient(r.Context(), token.Email); err != nil {
			slog.ErrorContext(r.Context(), "auth verify: RevokeClient failed", "error", err)
			WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Verification failed", 0)
			return
		}
	}

	apiKey := service.GenerateAPIKey()
	apiKeyHash := service.HashKey(apiKey)
	apiKeyPrefix := service.KeyPrefix(apiKey)

	if err := s.store.MarkClientVerified(r.Context(), token.Email, apiKeyHash, apiKeyPrefix); err != nil {
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Verification failed", 0)
		return
	}

	msg := "Store this key safely - it will not be shown again."
	if tokenType == store.TokenTypeReissue {
		msg = "Your old key has been revoked. " + msg
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]string{
		"api_key": apiKey,
		"message": msg,
	}); err != nil {
		slog.ErrorContext(r.Context(), "auth verify encode error", "error", err)
	}
}

func (s *Server) AuthReissue(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		WriteError(w, r, http.StatusServiceUnavailable, "INTERNAL_ERROR", "Auth endpoints require database configuration", 0)
		return
	}

	var req gen.ReissueRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_PARAM", "Invalid request body", 0)
		return
	}

	emailAddr, err := validateEmail(string(req.Email))
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, "INVALID_PARAM", "Invalid email address", 0)
		return
	}

	existing, err := s.store.GetClientByEmail(r.Context(), emailAddr)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		slog.ErrorContext(r.Context(), "auth reissue: GetClientByEmail failed", "error", err)
		writeAccepted(r.Context(), w)
		return
	}
	if existing != nil && existing.VerifiedAt != nil {
		token := service.GenerateToken()
		tokenHash := service.HashKey(token)
		expiresAt := time.Now().Add(24 * time.Hour)
		if err := s.tokens.CreateToken(r.Context(), emailAddr, tokenHash, store.TokenTypeReissue, expiresAt); err != nil {
			slog.ErrorContext(r.Context(), "auth reissue: CreateToken failed", "error", err)
		} else if s.email != nil {
			verifyURL := fmt.Sprintf("%s/verify?token=%s&type=reissue", s.config.BaseURL, token)
			if err := s.email.SendReissue(r.Context(), emailAddr, verifyURL); err != nil {
				slog.ErrorContext(r.Context(), "auth reissue: SendReissue failed", "error", err)
			}
		}
	}

	writeAccepted(r.Context(), w)
}

func writeAccepted(ctx context.Context, w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	if err := json.NewEncoder(w).Encode(map[string]string{
		"message": "If this email is eligible, a verification link has been sent.",
	}); err != nil {
		slog.ErrorContext(ctx, "writeAccepted encode error", "error", err)
	}
}
