package ctxwire

import (
	"context"
	"encoding/json"
)

// NewJSONPropagator returns a new Propagator with the given name and context key
// that uses JSON encoding and decoding.
func NewJSONPropagator(name string, contextKey any) Propagator {
	return NewPropagator(name, contextKey, EncoderFunc(jsonEncoder), DecoderFunc(jsonDecoder))
}

func jsonEncoder(ctx context.Context, key any) ([]byte, error) {
	v := ctx.Value(key)
	if v == nil {
		return nil, nil
	}
	return json.Marshal(v)
}

func jsonDecoder(ctx context.Context, key any, data []byte) (context.Context, error) {
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, err
	}
	return context.WithValue(ctx, key, v), nil
}
