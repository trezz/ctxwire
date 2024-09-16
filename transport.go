package ctxwire

import "net/http"

// ExtractTransport returns a transport that decorates the passed transport to
// extracts context values from the response headers.
func ExtractTransport(t http.RoundTripper) http.RoundTripper {
	return &extractTransport{t}
}

type extractTransport struct {
	http.RoundTripper
}

// RoundTrip implements the http.RoundTripper interface.
func (m *extractTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := m.RoundTripper.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	ctx, err := Extract(req.Context(), resp.Header)
	if err != nil {
		return resp, err
	}
	*req = *req.WithContext(ctx)

	return resp, nil
}

// InjectTransport is a transport that decorates the passed transport to inject
// request's context values to the request headers.
func InjectTransport(t http.RoundTripper) http.RoundTripper {
	return &injectTransport{t}
}

type injectTransport struct {
	http.RoundTripper
}

// RoundTrip implements the http.RoundTripper interface.
func (m *injectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	err := Inject(req.Context(), req.Header)
	if err != nil {
		return nil, err
	}
	return m.RoundTripper.RoundTrip(req)
}

// Transport returns a transport that decorates the passed transport to propagate
// context values between requests and responses.
func Transport(t http.RoundTripper) http.RoundTripper {
	return InjectTransport(ExtractTransport(t))
}
