package ctxwire_test

import (
	"context"
	"encoding/json"
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

func TestReceive(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(logProducerHandler))
	t.Cleanup(server.Close)

	ctxwire.Configure(
		ctxwire.NewJSONPropagator("str", keyStr),
		ctxwire.NewJSONPropagator("int", keyInt),
		ctxwire.NewPropagator("log", keyLog,
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
	c := http.Client{
		Transport: ctxwire.Transport(http.DefaultTransport),
	}
	// The request is created with the context, to be able to extract the query logs
	// from the response headers into it.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/", nil)
	require.NoError(t, err)
	_, err = c.Do(req)
	require.NoError(t, err)

	require.Equal(t, "bar", req.Context().Value(keyStr))
	require.Equal(t, 42, req.Context().Value(keyInt))

	finalLogState := req.Context().Value(keyLog).(logState)
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
