//ff:func feature=pkg-queue type=test control=sequence topic=tracing
//ff:what traceparent extract→inject round-trip이 TraceContext 를 보존하는지 검증한다

package queue

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// TestTraceparentRoundTrip exercises the helpers in isolation: a synthetic
// SpanContext is placed on ctx, extracted via the global propagator, then
// re-injected into a fresh ctx. The reconstructed SpanContext must carry
// the same TraceID/SpanID so downstream dispatchMessage observes the
// caller's span as parent.
func TestTraceparentRoundTrip(t *testing.T) {
	// Ensure the global propagator is the W3C TraceContext one the helpers
	// target. Other tests may leave it unset (which defaults to a no-op).
	otel.SetTextMapPropagator(propagation.TraceContext{})

	traceID, err := trace.TraceIDFromHex("0af7651916cd43dd8448eb211c80319c")
	if err != nil {
		t.Fatal(err)
	}
	spanID, err := trace.SpanIDFromHex("b7ad6b7169203331")
	if err != nil {
		t.Fatal(err)
	}
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
		Remote:     true,
	})
	srcCtx := trace.ContextWithSpanContext(context.Background(), sc)

	tp := extractTraceparent(srcCtx)
	if tp == "" {
		t.Fatal("extractTraceparent returned empty string for ctx with active SpanContext")
	}

	dstCtx := injectTraceparent(context.Background(), tp)
	got := trace.SpanContextFromContext(dstCtx)
	if !got.IsValid() {
		t.Fatalf("injectTraceparent produced invalid SpanContext from %q", tp)
	}
	if got.TraceID() != traceID {
		t.Errorf("TraceID = %s, want %s", got.TraceID(), traceID)
	}
	if got.SpanID() != spanID {
		t.Errorf("SpanID = %s, want %s", got.SpanID(), spanID)
	}
}

// TestTraceparentEmptyNoSpan confirms the disabled-tracing path: with no
// global propagator configured for a fresh SpanContext, extract returns ""
// and inject returns ctx unchanged (cheap, no allocation).
func TestTraceparentEmptyNoSpan(t *testing.T) {
	// Reset propagator to the default (no-op) so the test is hermetic.
	otel.SetTextMapPropagator(propagation.TraceContext{})

	ctx := context.Background()
	if got := extractTraceparent(ctx); got != "" {
		t.Errorf("extractTraceparent with no active span = %q, want empty", got)
	}
	if injectTraceparent(ctx, "") != ctx {
		t.Errorf("injectTraceparent(ctx, \"\") should return the same ctx")
	}
}
