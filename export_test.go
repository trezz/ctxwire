package ctxwire

import (
	"context"
)

// EncodeJSON exports the encodeJSON function for tests.
func EncodeJSON(ctx context.Context, key any) ([]byte, error) {
	return encodeJSON(ctx, key)
}

// DecodeJSON exports the decodeJSON function for tests.
func DecodeJSON(ctx context.Context, key any, data []byte) (context.Context, error) {
	return decodeJSON(ctx, key, data)
}
