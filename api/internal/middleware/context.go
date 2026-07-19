package middleware

import (
	"context"

	"zaas/api/internal/store"
)

type contextKey int

const clientKey contextKey = iota

// ClientInfo holds the authenticated client info attached to requests.
type ClientInfo struct {
	ID           string
	DisplayName  string
	RateLimitRPM int
}

// WithClient attaches client info to the context.
func WithClient(ctx context.Context, info *ClientInfo) context.Context {
	return context.WithValue(ctx, clientKey, info)
}

// GetClient retrieves client info from context. Returns nil for anonymous requests.
func GetClient(ctx context.Context) *ClientInfo {
	info, _ := ctx.Value(clientKey).(*ClientInfo)
	return info
}

// ClientInfoFromStore converts a store.Client to middleware.ClientInfo.
func ClientInfoFromStore(c *store.Client) *ClientInfo {
	return &ClientInfo{
		ID:           c.ID,
		DisplayName:  c.DisplayName,
		RateLimitRPM: c.RateLimitRPM,
	}
}
