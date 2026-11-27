// Package otel provides explicit OpenTelemetry tracing without installing global providers.
package otel

import (
	"context"
	"errors"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdk "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"log/slog"
	"net/http"
	"net/url"
	"time"
)

type Config struct {
	Service, EndpointURL string
	SampleRatio          float64
	AllowInsecure        bool
}
type Runtime struct {
	Provider   *sdk.TracerProvider
	Propagator propagation.TraceContext
	Tracer     trace.Tracer
}

func New(ctx context.Context, c Config) (*Runtime, error) {
	u, err := url.Parse(c.EndpointURL)
	if err != nil || u.Hostname() == "" || u.User != nil || u.Fragment != "" || u.RawQuery != "" || (u.Scheme != "https" && !(c.AllowInsecure && u.Scheme == "http")) || c.Service == "" || !(c.SampleRatio >= 0 && c.SampleRatio <= 1) {
		return nil, errors.New("invalid tracing configuration")
	}
	exporter, err := otlptracehttp.New(ctx, otlptracehttp.WithEndpointURL(c.EndpointURL), otlptracehttp.WithTimeout(5*time.Second))
	if err != nil {
		return nil, err
	}
	p := sdk.NewTracerProvider(sdk.WithBatcher(exporter), sdk.WithSampler(sdk.ParentBased(sdk.TraceIDRatioBased(c.SampleRatio))), sdk.WithResource(resource.NewSchemaless(attribute.String("service.name", c.Service))))
	return &Runtime{Provider: p, Tracer: p.Tracer("common-golang")}, nil
}
func (r *Runtime) Shutdown(ctx context.Context) error { return r.Provider.Shutdown(ctx) }
func (r *Runtime) Inject(ctx context.Context, c propagation.TextMapCarrier) {
	r.Propagator.Inject(ctx, c)
}
func (r *Runtime) Extract(ctx context.Context, c propagation.TextMapCarrier) context.Context {
	return r.Propagator.Extract(ctx, c)
}

// HTTPHandler uses a caller-supplied static route label, never the raw URL. Use only trusted inbound context or disable extraction at the public boundary.
func (r *Runtime) HTTPHandler(name string, trustInbound bool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, q *http.Request) {
		ctx := q.Context()
		if trustInbound {
			ctx = r.Extract(ctx, propagation.HeaderCarrier(q.Header))
		}
		ctx, span := r.Tracer.Start(ctx, name, trace.WithSpanKind(trace.SpanKindServer))
		defer span.End()
		next.ServeHTTP(w, q.WithContext(ctx))
	})
}

type Transport struct {
	Runtime *Runtime
	Base    http.RoundTripper
	Name    string
}

func (t Transport) RoundTrip(q *http.Request) (*http.Response, error) {
	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}
	ctx, span := t.Runtime.Tracer.Start(q.Context(), t.Name, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()
	copy := q.Clone(ctx)
	t.Runtime.Inject(ctx, propagation.HeaderCarrier(copy.Header))
	res, err := base.RoundTrip(copy)
	if err != nil || (res != nil && res.StatusCode >= 500) {
		span.SetStatus(codes.Error, "HTTP dependency failure")
	}
	if res != nil {
		span.SetAttributes(attribute.Int("http.response.status_code", res.StatusCode))
	}
	return res, err
}
func WithLog(ctx context.Context, l *slog.Logger) *slog.Logger {
	if l == nil {
		l = slog.Default()
	}
	s := trace.SpanContextFromContext(ctx)
	if !s.IsValid() {
		return l
	}
	return l.With("trace_id", s.TraceID().String(), "span_id", s.SpanID().String())
}
