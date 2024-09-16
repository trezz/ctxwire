# ctxwire

`ctxwire` is a Go package for propagating context values between HTTP requests and responses using HTTP headers. It simplifies passing metadata like tracing IDs, user info, or logs across services in a distributed system.

It is a simpler alternative to OpenTelemetry.

## Features

- Inject context values into HTTP headers for outgoing requests.
- Extract context values from incoming HTTP responses.
- Automatically propagate context values between services.
- Support for custom encoders and decoders
- Context propagation can be configured to be forward-only, back-only or both. Back propagation is particularly useful for scenarios where logs are generated across multiple services during a request and need to be aggregated at the end of the request's execution.

## Usage

### Create a JSON propagator

```go
type keyCtx struct{}
propagator := ctxwire.NewJSONPropagator("UserInfo", keyCtx{})
ctxwire.Configure(propagator)
```

This needs to be done client and server side.

### Wrap HTTP transport

```go
client := &http.Client{
    Transport: ctxwire.Transport(http.DefaultTransport),
}
```

- Use `ctxwire.InjectTransport` for forward propagation
- Use `ctxwire.ExtractTransport` for back propagation

### Extract context values from HTTP request headers

```go
ctx, err := ctxwire.Extract(context.Background(), req.Header)
```

### Inject context into HTTP response headers

```go
ctx := context.WithValue(context.Background(), ctxKey, yourValue)
err := ctxwire.Inject(ctx, response.Header())
```

## Custom encoding

```go
func myEncoder(ctx context.Context, key any) ([]byte, error) {
    // Custom encoding logic
})

func myDecoder(ctx context.Context, key any, data []byte) (context.Context, error) {
    // Custom decoding logic
})

func main() {
    propagator := ctxwire.NewPropagator("CustomKey", ctxKey,
        ctxwire.EncoderFunc(myEncoder),
        ctxwire.DecoderFunc(myDecoder))
    ctxwire.Configure(propagator)
}
```

## License

This project is licensed under the MIT License.
