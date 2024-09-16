// Package ctxwire propagate context values between HTTP requests and responses
// over the wire using HTTP headers.
package ctxwire

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
)

// Error is the error type used by the package.
// It wraps the original error and adds a message.
type Error struct {
	message string
	err     error
}

var _ error = (*Error)(nil)

// Error implements the error interface.
func (e *Error) Error() string {
	return fmt.Sprintf("%s: %s", e.message, e.err.Error())
}

// Unwrap implements the errors.Wrapper interface.
func (e *Error) Unwrap() error {
	return e.err
}

func newError(message string, err error) error {
	var ctxwireErr *Error
	if errors.As(err, &ctxwireErr) {
		return err
	}
	return &Error{message: message, err: err}
}

// propagator propagates context values between requests and responses.
type Propagator interface {
	// Inject injects the context values into the given headers.
	Inject(ctx context.Context, h http.Header) error
	// Extract extracts the context values from the given headers into a copy of
	// the given context.
	Extract(ctx context.Context, h http.Header) (context.Context, error)
}

// NewValuePropagator returns a new ValuePropagator with the given name.
// The context key is used to store the context value in the context.
// The encoder and decoder are used to encode and decode the context value.
func NewValuePropagator(name string, contextKey any, encoder Encoder, decoder Decoder) *ValuePropagator {
	return &ValuePropagator{
		name:       name,
		contextKey: contextKey,
		encoder:    encoder,
		decoder:    decoder,
	}
}

// NewJSONPropagator returns a new ValuePropagator with the given name configured
// to encode and decode the context value as JSON.
// The context key is used to store the context value in the context.
func NewJSONPropagator(name string, contextKey any) *ValuePropagator {
	return NewValuePropagator(name, contextKey, EncoderFunc(encodeJSON), DecoderFunc(decodeJSON))
}

func encodeJSON(ctx context.Context, key any) ([]byte, error) {
	v := ctx.Value(key)
	if v == nil {
		return nil, nil
	}
	return json.Marshal(v)
}

func decodeJSON(ctx context.Context, key any, data []byte) (context.Context, error) {
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, err
	}
	return context.WithValue(ctx, key, v), nil
}

// ValuePropagator propagates a single context value between requests and responses.
// It implements the Propagator interface.
type ValuePropagator struct {
	name       string
	contextKey any
	encoder    Encoder
	decoder    Decoder
}

var _ Propagator = (*ValuePropagator)(nil)

// Inject implements the Propagator interface.
func (p *ValuePropagator) Inject(ctx context.Context, h http.Header) error {
	data, err := p.encoder.Encode(ctx, p.contextKey)
	if err != nil {
		return newError("encode context value", err)
	}
	if len(data) == 0 {
		return nil
	}
	h.Set(headerKey(p.name), base64.StdEncoding.EncodeToString(data))
	return nil
}

// Extract implements the Propagator interface.
func (p *ValuePropagator) Extract(ctx context.Context, h http.Header) (context.Context, error) {
	vStr := h.Get(headerKey(p.name))
	if vStr == "" {
		return ctx, nil
	}
	v, err := base64.StdEncoding.DecodeString(vStr)
	if err != nil {
		return nil, newError("base64 decode context value", err)
	}
	newCtx, err := p.decoder.Decode(ctx, p.contextKey, v)
	if err != nil {
		return nil, newError("decode context value", err)
	}
	return newCtx, nil
}

func headerKey(name string) string { return "x-ctxwire-" + name }

// Encoder is an interface for encoding context values into bytes.
// Errors returned by the encoder should be wrapped with ctxwire.NewError.
type Encoder interface {
	// Encode encodes the context value associated with the given key into bytes.
	Encode(ctx context.Context, key any) (data []byte, err error)
}

// Decoder is an interface for decoding bytes into context values.
// Errors returned by the encoder should be wrapped with ctxwire.NewError.
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

// Configure configures the propagators to be used to propagate context values
// between requests and responses.
func Configure(propagators ...Propagator) {
	register.add(propagators...)
}

// Inject injects the context values into the given headers.
func Inject(ctx context.Context, h http.Header) error {
	if err := register.Inject(ctx, h); err != nil {
		return newError("inject context values", err)
	}
	return nil
}

// Extract extracts the context values from the given headers into a copy of
// the given context.
func Extract(ctx context.Context, h http.Header) (context.Context, error) {
	newCtx, err := register.Extract(ctx, h)
	if err != nil {
		return nil, newError("extract context values", err)
	}
	return newCtx, nil
}

var register propagatorRegister

type propagatorRegister struct {
	mu          sync.Mutex
	propagators []Propagator
}

var _ Propagator = (*propagatorRegister)(nil)

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
