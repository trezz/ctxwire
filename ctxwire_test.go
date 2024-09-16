package ctxwire_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/trezz/ctxwire"
)

type (
	strKey struct{}
	intKey struct{}
	logKey struct{}
)

var (
	keyStr strKey
	keyInt intKey
	keyLog logKey
)

func TestBackPropagation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(logProducerHandler))
	t.Cleanup(server.Close)

	ctxwire.Configure(
		ctxwire.NewJSONPropagator("str", keyStr),
		ctxwire.NewJSONPropagator("int", keyInt),
		ctxwire.NewValuePropagator("log", keyLog,
			ctxwire.EncoderFunc(logEncoder),
			ctxwire.DecoderFunc(logDecoder),
		),
	)

	// Client update its context.
	ctx := context.WithValue(context.Background(), keyStr, "foo")
	ctx = context.WithValue(ctx, intKey{}, 42)
	ctx = context.WithValue(ctx, keyLog, logState{
		attrs: []logAttr{
			logWithService("search"),
			logWithIndex("products"),
		},
	})

	// Client sends the request with a transport that extracts the query logs
	// from the response headers.
	// The request is created with the context, to be able to extract the query logs
	// from the response headers into it.
	req, err := http.NewRequest(http.MethodGet, server.URL+"/", nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	ctx, err = ctxwire.Extract(ctx, resp.Header)
	require.NoError(t, err)

	require.Equal(t, "bar", ctx.Value(keyStr))
	require.Equal(t, 42, ctx.Value(keyInt))

	finalLogState := ctx.Value(keyLog).(logState)
	var finalLog logEntry
	for _, attr := range finalLogState.attrs {
		attr(&finalLog)
	}
	require.Equal(t, "search", finalLog.Service)
	require.Equal(t, "new_products", finalLog.Index)
	require.Equal(t, "123", finalLog.UserToken)
	require.Equal(t, 42, finalLog.LatencyMS)
}

func logProducerHandler(w http.ResponseWriter, r *http.Request) {
	// Server adds values to its own context.
	ctx := context.WithValue(r.Context(), keyStr, "bar")
	ctx = context.WithValue(ctx, keyLog, logState{
		attrs: []logAttr{
			logWithUserToken("123"),
			logWithLatency(42),
			logWithIndex("new_products"),
		},
	})
	// Logs in the context are written to the response headers.
	_ = ctxwire.Inject(ctx, w.Header())
	_, _ = w.Write([]byte("OK"))
}

type logState struct {
	attrs []logAttr
}

type logAttr func(l *logEntry)

type logEntry struct {
	Service   string `json:"service,omitempty"`
	Index     string `json:"index,omitempty"`
	UserToken string `json:"user_token,omitempty"`
	LatencyMS int    `json:"latency_ms,omitempty"`
}

func logWithService(service string) logAttr { return func(l *logEntry) { l.Service = service } }
func logWithIndex(index string) logAttr     { return func(l *logEntry) { l.Index = index } }
func logWithUserToken(token string) logAttr { return func(l *logEntry) { l.UserToken = token } }
func logWithLatency(latency int) logAttr    { return func(l *logEntry) { l.LatencyMS = latency } }

func logWithJSONEntry(data json.RawMessage) logAttr {
	return func(l *logEntry) {
		_ = json.Unmarshal(data, l)
	}
}

func logEncoder(ctx context.Context, key any) ([]byte, error) {
	v, ok := ctx.Value(key).(logState)
	if !ok {
		return nil, nil
	}

	var e logEntry
	for _, attr := range v.attrs {
		attr(&e)
	}
	return json.Marshal(e)
}

func logDecoder(ctx context.Context, key any, data []byte) (context.Context, error) {
	v, ok := ctx.Value(key).(logState)
	if !ok {
		return ctx, nil
	}
	var eJSON json.RawMessage
	if err := json.Unmarshal(data, &eJSON); err != nil {
		return nil, err
	}
	v.attrs = append(slices.Clone(v.attrs), logWithJSONEntry(eJSON))
	return context.WithValue(ctx, key, v), nil
}

type (
	encodeKey struct{}
	decodeKey struct{}
)

var (
	keyEncode encodeKey
	keyDecode decodeKey
)

func TestError(t *testing.T) {
	ctxwire.Configure(
		ctxwire.NewValuePropagator("encode", keyEncode,
			ctxwire.EncoderFunc(errEncoder),
			ctxwire.DecoderFunc(ctxwire.DecodeJSON)),
		ctxwire.NewValuePropagator("decode", keyDecode,
			ctxwire.EncoderFunc(ctxwire.EncodeJSON),
			ctxwire.DecoderFunc(errDecoder)),
	)

	ctx := context.WithValue(context.Background(), keyEncode, "foo")
	h := http.Header{}
	err := ctxwire.Inject(ctx, h)
	require.EqualError(t, err, "encode context value: failed!")

	ctx = context.WithValue(context.Background(), keyDecode, "bar")
	require.NoError(t, ctxwire.Inject(ctx, h))
	_, err = ctxwire.Extract(context.Background(), h)
	require.EqualError(t, err, "decode context value: failed!")
}

func errEncoder(ctx context.Context, key any) ([]byte, error) {
	v := ctx.Value(key)
	if v == nil {
		return nil, nil
	}
	return nil, errors.New("failed!")
}

func errDecoder(_ context.Context, _ any, _ []byte) (context.Context, error) {
	return nil, errors.New("failed!")
}
