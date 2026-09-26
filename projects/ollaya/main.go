package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func initTracer(ctx context.Context) (*sdktrace.TracerProvider, error) {
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		endpoint = "jaeger.optimus.svc:4317"
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String("ollaya"),
			semconv.DeploymentEnvironmentKey.String("dev"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithInsecure(),
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
	)
	if err != nil {
		log.Printf("Warning: failed to initialize OTLP trace exporter to %s: %v", endpoint, err)
		tp := sdktrace.NewTracerProvider(sdktrace.WithResource(res))
		otel.SetTracerProvider(tp)
		return tp, nil
	}

	bsp := sdktrace.NewBatchSpanProcessor(exporter)
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(bsp),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	log.Printf("OpenTelemetry Tracer initialized for Ollaya -> %s", endpoint)
	return tp, nil
}

func main() {
	tp, err := initTracer(context.Background())
	if err == nil && tp != nil {
		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = tp.Shutdown(shutdownCtx)
		}()
	}

	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok","server":"ollaya"}`))
	})

	// Model pull endpoint
	mux.HandleFunc("/api/pull", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","digest":"sha256:laya-warm-digest"}`))
	})

	// SystemOne wire endpoints
	handleDecide := func(w http.ResponseWriter, r *http.Request) {
		ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))
		tr := otel.Tracer("ollaya")
		_, span := tr.Start(ctx, "SystemOneInference")
		span.SetAttributes(
			attribute.String("model", "laya"),
			attribute.String("http.path", r.URL.Path),
		)
		defer span.End()

		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"model": "laya",
			"answers": map[string]any{
				"severity": map[string]any{
					"value": "High",
					"score": 2.1,
				},
				"safety_risk": map[string]any{
					"value":       "HIGH",
					"probability": 0.94,
				},
				"field_visit_required": map[string]any{
					"value":       true,
					"probability": 0.91,
				},
			},
			"confidence": 0.94,
			"timing_ms":  18.5,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}

	mux.HandleFunc("/v1/systemone", handleDecide)
	mux.HandleFunc("/v1/decisions", handleDecide)
	mux.HandleFunc("/api/decide", handleDecide)
	mux.Handle("/metrics", promhttp.Handler())

	log.Println("Starting Ollaya server on :11435")
	if err := http.ListenAndServe(":11435", mux); err != nil {
		log.Fatalf("Ollaya server failed: %v", err)
	}
}
