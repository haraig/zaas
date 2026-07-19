package ratelimiter

import (
	"errors"
	"testing"
)

func TestParseScriptResult_Valid(t *testing.T) {
	t.Parallel()
	allowed, count, oldestMs, err := parseScriptResult([]interface{}{int64(1), int64(3), int64(1234567890)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Error("expected allowed=true for allowedRaw=1")
	}
	if count != 3 {
		t.Errorf("expected count=3, got %d", count)
	}
	if oldestMs != 1234567890 {
		t.Errorf("expected oldestMs=1234567890, got %d", oldestMs)
	}
}

func TestParseScriptResult_NotAllowed(t *testing.T) {
	t.Parallel()
	allowed, _, _, err := parseScriptResult([]interface{}{int64(0), int64(5), int64(0)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allowed {
		t.Error("expected allowed=false for allowedRaw=0")
	}
}

func TestParseScriptResult_WrongLength(t *testing.T) {
	t.Parallel()
	_, _, _, err := parseScriptResult([]interface{}{int64(1), int64(1)})
	if !errors.Is(err, ErrUnexpectedResult) {
		t.Errorf("expected ErrUnexpectedResult, got %v", err)
	}
}

func TestParseScriptResult_WrongType_Allowed(t *testing.T) {
	t.Parallel()
	_, _, _, err := parseScriptResult([]interface{}{"not-an-int64", int64(1), int64(0)})
	if !errors.Is(err, ErrInvalidResultType) {
		t.Errorf("expected ErrInvalidResultType for bad allowed field, got %v", err)
	}
}

func TestParseScriptResult_WrongType_Count(t *testing.T) {
	t.Parallel()
	_, _, _, err := parseScriptResult([]interface{}{int64(1), "not-an-int64", int64(0)})
	if !errors.Is(err, ErrInvalidResultType) {
		t.Errorf("expected ErrInvalidResultType for bad count field, got %v", err)
	}
}

func TestParseScriptResult_WrongType_OldestMs(t *testing.T) {
	t.Parallel()
	_, _, _, err := parseScriptResult([]interface{}{int64(1), int64(1), 3.14})
	if !errors.Is(err, ErrInvalidResultType) {
		t.Errorf("expected ErrInvalidResultType for bad oldestMs field, got %v", err)
	}
}
