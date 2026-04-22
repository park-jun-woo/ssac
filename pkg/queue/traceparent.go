//ff:func feature=pkg-queue type=util control=sequence topic=tracing
//ff:what W3C TraceContext traceparent 추출/주입 — 큐 메시지에 span context 전파

package queue

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// extractTraceparent pulls the active span's W3C `traceparent` value out of
// ctx for storage alongside an enqueued message. When no span is active
// (tracing disabled, or the caller hasn't started one), returns "". The
// global TextMapPropagator is consulted so apps that register Baggage or
// other propagators still work — only the `traceparent` key is persisted
// here to keep the column single-valued; other carrier fields (tracestate,
// baggage) are recoverable from a full OTel-aware store if needed later.
func extractTraceparent(ctx context.Context) string {
	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	return carrier["traceparent"]
}

// injectTraceparent returns a derived ctx with span context reconstructed
// from a previously persisted `traceparent` string. Empty input returns
// ctx unchanged so callers can pass it blindly after reading from
// fullend_queue. The extraction uses the same global propagator as
// extractTraceparent so the pair is symmetric.
func injectTraceparent(ctx context.Context, traceparent string) context.Context {
	if traceparent == "" {
		return ctx
	}
	carrier := propagation.MapCarrier{"traceparent": traceparent}
	return otel.GetTextMapPropagator().Extract(ctx, carrier)
}
