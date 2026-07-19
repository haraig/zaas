package service

import (
	"testing"
)

func TestGenerateAPIKey(t *testing.T) {
	t.Parallel()
	key := GenerateAPIKey()
	if len(key) != 5+32 { // "zaas_" + 32 hex chars
		t.Fatalf("expected length 37, got %d: %s", len(key), key)
	}
	if key[:5] != "zaas_" {
		t.Fatalf("expected zaas_ prefix, got %s", key[:5])
	}
}

func TestGenerateAPIKey_Unique(t *testing.T) {
	t.Parallel()
	k1 := GenerateAPIKey()
	k2 := GenerateAPIKey()
	if k1 == k2 {
		t.Fatal("keys should be unique")
	}
}

func TestHashKey(t *testing.T) {
	t.Parallel()
	hash := HashKey("zaas_deadbeefdeadbeefdeadbeefdeadbeef")
	if len(hash) != 64 { // SHA-256 hex
		t.Fatalf("expected 64 char hash, got %d", len(hash))
	}
}

func TestKeyPrefix(t *testing.T) {
	t.Parallel()
	prefix := KeyPrefix("zaas_deadbeefdeadbeefdeadbeefdeadbeef")
	if prefix != "zaas_dea" {
		t.Fatalf("expected zaas_dea, got %s", prefix)
	}
}

func TestGenerateToken(t *testing.T) {
	t.Parallel()
	token := GenerateToken()
	if len(token) != 64 { // 32 bytes hex-encoded
		t.Fatalf("expected length 64, got %d", len(token))
	}
}
