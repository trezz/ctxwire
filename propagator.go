package ctxwire

import (
	"context"
	"encoding/base64"
	"net/http"
	"sync"
)

// propagator propagates context values between requests and responses.
type Propagator interface {
	// Inject injects the context values into the given headers.
	Inject(ctx context.Context, h http.Header) error
	// Extract extracts the context values from the given headers into a copy of
	// the given context.
	Extract(ctx context.Context, h http.Header) (context.Context, error)
}

// NewPropagator returns a new Propagator with the given name, context key,
// encoder, and decoder.
func NewPropagator(name string, contextKey any, encoder Encoder, decoder Decoder) Propagator {
	return &propagator{
		name:       name,
		contextKey: contextKey,
		encoder:    encoder,
		decoder:    decoder,
	}
}

type propagator struct {
	name       string
	contextKey any
	encoder    Encoder
	decoder    Decoder
}

// Inject implements the Propagator interface.
func (p *propagator) Inject(ctx context.Context, h http.Header) error {
	data, err := p.encoder.Encode(ctx, p.contextKey)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}
	h.Set(headerKey(p.name), base64.StdEncoding.EncodeToString(data))
	return nil
}

// Extract implements the Propagator interface.
func (p *propagator) Extract(ctx context.Context, h http.Header) (context.Context, error) {
	vStr := h.Get(headerKey(p.name))
	if vStr == "" {
		return ctx, nil
	}
	v, err := base64.StdEncoding.DecodeString(vStr)
	if err != nil {
		return nil, err
	}
	return p.decoder.Decode(ctx, p.contextKey, v)
}

func headerKey(name string) string { return "x-ctxwire-" + name }

// Encoder is an interface for encoding context values into bytes.
type Encoder interface {
	// Encode encodes the context value associated with the given key into bytes.
	Encode(ctx context.Context, key any) (data []byte, err error)
}

// Decoder is an interface for decoding bytes into context values.
type Decoder interface {
	// Decode decodes the given data into a context value associated with the
	// given key and returns a new context with the value set.
	Decode(ctx context.Context, key any, data []byte) (context.Context, error)
}

// EncoderFunc is an adapter type to allow the use of ordinary functions as encoders.
type EncoderFunc func(ctx context.Context, key any) ([]byte, error)

// Encode implements the Encoder interface.
func (f EncoderFunc) Encode(ctx context.Context, key any) ([]byte, error) {
	return f(ctx, key)
}

// DecoderFunc is an adapter type to allow the use of ordinary functions as decoders.
type DecoderFunc func(ctx context.Context, key any, data []byte) (context.Context, error)

// Decode implements the Decoder interface.
func (f DecoderFunc) Decode(ctx context.Context, key any, data []byte) (context.Context, error) {
	return f(ctx, key, data)
}

var register propagatorRegister

type propagatorRegister struct {
	mu          sync.Mutex
	propagators []Propagator
}

func (r *propagatorRegister) add(propagators ...Propagator) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.propagators = append(r.propagators, propagators...)
}

// Inject implements the Propagator interface.
func (r *propagatorRegister) Inject(ctx context.Context, h http.Header) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, p := range r.propagators {
		if err := p.Inject(ctx, h); err != nil {
			return err
		}
	}
	return nil
}

// Configure configures the propagators to be used to propagate context values
// between requests and responses.
func Configure(propagators ...Propagator) {
	register.add(propagators...)
}

// Extract implements the Propagator interface.
func (r *propagatorRegister) Extract(ctx context.Context, h http.Header) (context.Context, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, p := range r.propagators {
		var err error
		ctx, err = p.Extract(ctx, h)
		if err != nil {
			return nil, err
		}
	}
	return ctx, nil
}

// Inject injects the context values into the given headers.
func Inject(ctx context.Context, h http.Header) error {
	if err := register.Inject(ctx, h); err != nil {
		return newError("inject context into header", err)
	}
	return nil
}

// Extract extracts the context values from the given headers into a copy of
// the given context.
func Extract(ctx context.Context, h http.Header) (context.Context, error) {
	newCtx, err := register.Extract(ctx, h)
	if err != nil {
		return nil, newError("extract context from header", err)
	}
	return newCtx, nil
}
