package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/peergoim/signaling-server/internal/middleware/response"
	"github.com/zeromicro/go-zero/core/trace"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	oteltrace "go.opentelemetry.io/otel/trace"
)

func Tracer() gin.HandlerFunc {
	serviceName := "signaling-server"

	return func(context *gin.Context) {
		spanName := context.Request.URL.Path
		tracer := otel.Tracer(trace.TraceName)
		propagator := otel.GetTextMapPropagator()

		ctx := propagator.Extract(context.Request.Context(), propagation.HeaderCarrier(context.Request.Header))
		spanCtx, span := tracer.Start(
			ctx,
			spanName,
			oteltrace.WithSpanKind(oteltrace.SpanKindServer),
			oteltrace.WithAttributes(semconv.HTTPServerAttributesFromHTTPRequest(
				serviceName, spanName, context.Request)...),
		)
		defer span.End()

		// convenient for tracking error messages
		propagator.Inject(spanCtx, propagation.HeaderCarrier(context.Writer.Header()))

		trw := response.NewWithCodeResponseWriter(context.Writer)
		context.Request = context.Request.WithContext(spanCtx)

		context.Next()

		span.SetAttributes(semconv.HTTPAttributesFromHTTPStatusCode(trw.Code)...)
		span.SetStatus(semconv.SpanStatusFromHTTPStatusCodeAndSpanKind(
			trw.Code, oteltrace.SpanKindServer))
	}
}
