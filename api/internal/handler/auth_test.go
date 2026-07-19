package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"zaas/api/internal/config"
	"zaas/api/internal/service"
	"zaas/api/internal/store"
)

// mockEmailSender captures sent emails for assertion.
type mockEmailSender struct {
	verificationSent bool
	reissueSent      bool
	lastTo           string
}

func (m *mockEmailSender) SendVerification(_ context.Context, to, _ string) error {
	m.verificationSent = true
	m.lastTo = to
	return nil
}

func (m *mockEmailSender) SendReissue(_ context.Context, to, _ string) error {
	m.reissueSent = true
	m.lastTo = to
	return nil
}

// inMemoryStore implements both ClientStore and TokenStore for testing.
type inMemoryStore struct {
	clients map[string]*store.Client // keyed by email
	tokens  map[string]*store.Token  // keyed by token_hash
}

func newInMemoryStore() *inMemoryStore {
	return &inMemoryStore{
		clients: make(map[string]*store.Client),
		tokens:  make(map[string]*store.Token),
	}
}

func (s *inMemoryStore) CreateClient(_ context.Context, email, displayName string) error {
	if _, exists := s.clients[email]; exists {
		return store.ErrAlreadyExists
	}
	s.clients[email] = &store.Client{
		ID:           "test-uuid-" + email,
		Email:        email,
		DisplayName:  displayName,
		RateLimitRPM: 600,
	}
	return nil
}

func (s *inMemoryStore) GetClientByEmail(_ context.Context, email string) (*store.Client, error) {
	c, ok := s.clients[email]
	if !ok {
		return nil, store.ErrNotFound
	}
	return c, nil
}

func (s *inMemoryStore) GetClientByAPIKeyHash(_ context.Context, keyHash string) (*store.Client, error) {
	for _, c := range s.clients {
		if c.APIKeyHash == keyHash {
			return c, nil
		}
	}
	return nil, store.ErrNotFound
}

func (s *inMemoryStore) MarkClientVerified(_ context.Context, email, apiKeyHash, apiKeyPrefix string) error {
	c, ok := s.clients[email]
	if !ok {
		return store.ErrNotFound
	}
	now := time.Now()
	c.VerifiedAt = &now
	c.APIKeyHash = apiKeyHash
	c.APIKeyPrefix = apiKeyPrefix
	c.RevokedAt = nil
	return nil
}

func (s *inMemoryStore) RevokeClient(_ context.Context, email string) error {
	c, ok := s.clients[email]
	if !ok {
		return store.ErrNotFound
	}
	now := time.Now()
	c.RevokedAt = &now
	return nil
}

func (s *inMemoryStore) CreateToken(_ context.Context, email, tokenHash string, tokenType store.TokenType, expiresAt time.Time) error {
	s.tokens[tokenHash] = &store.Token{
		ID:        "tok-" + tokenHash[:8],
		Email:     email,
		TokenHash: tokenHash,
		Type:      tokenType,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now(),
	}
	return nil
}

func (s *inMemoryStore) ConsumeToken(_ context.Context, tokenHash string, tokenType store.TokenType) (*store.Token, error) {
	tok, ok := s.tokens[tokenHash]
	if !ok {
		return nil, store.ErrNotFound
	}
	if tok.Type != tokenType {
		return nil, store.ErrNotFound
	}
	if tok.UsedAt != nil {
		return nil, store.ErrNotFound
	}
	if time.Now().After(tok.ExpiresAt) {
		return nil, store.ErrNotFound
	}
	now := time.Now()
	tok.UsedAt = &now
	return tok, nil
}

func newAuthServer(memStore *inMemoryStore, emailSender *mockEmailSender) *Server {
	return New(config.Config{BaseURL: "http://localhost:8080"}, Deps{
		Store:  memStore,
		Tokens: memStore,
		Email:  emailSender,
	})
}

func TestAuthRegister_NewEmail_Returns202(t *testing.T) {
	t.Parallel()
	memStore := newInMemoryStore()
	emailSender := &mockEmailSender{}
	srv := newAuthServer(memStore, emailSender)

	body := `{"email":"test@example.com","display_name":"Test App"}`
	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/v1/auth/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.AuthRegister(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}
	if !emailSender.verificationSent {
		t.Fatal("expected verification email to be sent")
	}
	if emailSender.lastTo != "test@example.com" {
		t.Fatalf("expected email to test@example.com, got %s", emailSender.lastTo)
	}
}

func TestAuthRegister_InvalidDisplayName_Returns400(t *testing.T) {
	t.Parallel()
	srv := newAuthServer(newInMemoryStore(), &mockEmailSender{})

	body := `{"email":"test@example.com","display_name":""}`
	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/v1/auth/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.AuthRegister(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestAuthRegister_AlreadyVerified_Returns202NoEmail(t *testing.T) {
	t.Parallel()
	memStore := newInMemoryStore()
	now := time.Now()
	memStore.clients["verified@example.com"] = &store.Client{
		ID: "existing", Email: "verified@example.com", DisplayName: "Existing",
		VerifiedAt: &now, RateLimitRPM: 600,
	}
	emailSender := &mockEmailSender{}
	srv := newAuthServer(memStore, emailSender)

	body := `{"email":"verified@example.com","display_name":"Existing"}`
	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/v1/auth/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.AuthRegister(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rec.Code)
	}
	if emailSender.verificationSent {
		t.Fatal("expected no email sent for already-verified client")
	}
}

func TestAuthRegister_NilStore_Returns503(t *testing.T) {
	t.Parallel()
	srv := New(config.Config{BaseURL: "http://localhost:8080"}, Deps{})

	body := `{"email":"test@example.com","display_name":"Test"}`
	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/v1/auth/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.AuthRegister(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestAuthVerify_Registration_Returns200WithKey(t *testing.T) {
	t.Parallel()
	memStore := newInMemoryStore()
	_ = memStore.CreateClient(context.Background(), "test@example.com", "Test")

	// Pre-create token using the same hash the handler will compute
	rawToken := "test-token-abc123"
	tokenHash := service.HashKey(rawToken)
	memStore.tokens[tokenHash] = &store.Token{
		ID:        "tok-1",
		Email:     "test@example.com",
		TokenHash: tokenHash,
		Type:      store.TokenTypeRegistration,
		ExpiresAt: time.Now().Add(time.Hour),
		CreatedAt: time.Now(),
	}

	srv := newAuthServer(memStore, &mockEmailSender{})

	body := `{"token":"test-token-abc123","type":"registration"}`
	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/v1/auth/verify", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.AuthVerify(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp["api_key"] == "" {
		t.Fatal("expected api_key in response")
	}
	if len(resp["api_key"]) != 37 { // zaas_ + 32 hex chars
		t.Fatalf("expected api_key length 37, got %d: %s", len(resp["api_key"]), resp["api_key"])
	}
}

func TestAuthVerify_InvalidToken_Returns400(t *testing.T) {
	t.Parallel()
	srv := newAuthServer(newInMemoryStore(), &mockEmailSender{})

	body := `{"token":"nonexistent-token","type":"registration"}`
	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/v1/auth/verify", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.AuthVerify(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestAuthVerify_ReissueType_RevokesOldKey(t *testing.T) {
	t.Parallel()
	memStore := newInMemoryStore()
	now := time.Now()
	memStore.clients["test@example.com"] = &store.Client{
		ID: "client-1", Email: "test@example.com", DisplayName: "Test",
		VerifiedAt: &now, APIKeyHash: "oldhash", APIKeyPrefix: "zaas_old",
		RateLimitRPM: 600,
	}

	rawToken := "reissue-token-xyz"
	tokenHash := service.HashKey(rawToken)
	memStore.tokens[tokenHash] = &store.Token{
		ID:        "tok-2",
		Email:     "test@example.com",
		TokenHash: tokenHash,
		Type:      store.TokenTypeReissue,
		ExpiresAt: time.Now().Add(time.Hour),
		CreatedAt: time.Now(),
	}

	srv := newAuthServer(memStore, &mockEmailSender{})

	body := `{"token":"reissue-token-xyz","type":"reissue"}`
	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/v1/auth/verify", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.AuthVerify(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Old key should be replaced with a new one
	client := memStore.clients["test@example.com"]
	if client.APIKeyHash == "oldhash" {
		t.Fatal("expected API key to be updated")
	}

	var resp map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["api_key"] == "" {
		t.Fatal("expected api_key in response")
	}
}

func TestAuthReissue_VerifiedEmail_Returns202(t *testing.T) {
	t.Parallel()
	memStore := newInMemoryStore()
	now := time.Now()
	memStore.clients["test@example.com"] = &store.Client{
		ID: "c1", Email: "test@example.com", VerifiedAt: &now, RateLimitRPM: 600,
	}
	emailSender := &mockEmailSender{}
	srv := newAuthServer(memStore, emailSender)

	body := `{"email":"test@example.com"}`
	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/v1/auth/reissue", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.AuthReissue(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rec.Code)
	}
	if !emailSender.reissueSent {
		t.Fatal("expected reissue email to be sent")
	}
}

func TestAuthReissue_UnknownEmail_Returns202NoEmail(t *testing.T) {
	t.Parallel()
	emailSender := &mockEmailSender{}
	srv := newAuthServer(newInMemoryStore(), emailSender)

	body := `{"email":"unknown@example.com"}`
	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/v1/auth/reissue", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.AuthReissue(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rec.Code)
	}
	if emailSender.reissueSent {
		t.Fatal("expected no email sent for unknown email")
	}
}

// --- Email validation / SMTP header injection prevention ---

func TestAuthRegister_CRLFEmail_Returns400_NoEmailSent(t *testing.T) {
	t.Parallel()
	emailSender := &mockEmailSender{}
	srv := newAuthServer(newInMemoryStore(), emailSender)

	body := `{"email":"victim@x.com\r\nBcc: attacker@evil.com","display_name":"Injector"}`
	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/v1/auth/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.AuthRegister(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for CRLF email, got %d", rec.Code)
	}
	if emailSender.verificationSent {
		t.Fatal("expected no email to be sent for invalid email address")
	}
}

func TestAuthRegister_InvalidEmailFormat_Returns400(t *testing.T) {
	t.Parallel()
	emailSender := &mockEmailSender{}
	srv := newAuthServer(newInMemoryStore(), emailSender)

	body := `{"email":"not-an-email","display_name":"Test"}`
	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/v1/auth/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.AuthRegister(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid email, got %d", rec.Code)
	}
	if emailSender.verificationSent {
		t.Fatal("expected no email to be sent for invalid email address")
	}
}

func TestAuthReissue_CRLFEmail_Returns400_NoEmailSent(t *testing.T) {
	t.Parallel()
	emailSender := &mockEmailSender{}
	srv := newAuthServer(newInMemoryStore(), emailSender)

	body := "{\"email\":\"victim@x.com\\r\\nBcc: attacker@evil.com\"}"
	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/v1/auth/reissue", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.AuthReissue(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for CRLF email in reissue, got %d", rec.Code)
	}
	if emailSender.reissueSent {
		t.Fatal("expected no email to be sent for invalid email address")
	}
}
