package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"runtime/debug"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	// auto-instrument the gin-gonic web server
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	// instrument HTTP requests
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.7.0"
	_ "go.uber.org/automaxprocs"
)

func initTracer() func() {

	// Top-level Context for incoming requests
	ctx := context.Background()

	// Create trace exporter to be able to retrieve
	// the collected spans.
	exporter, err := otlptracegrpc.New(ctx)
	if err != nil {
		log.Ctx(ctx).Err(err).Msg("Err in creating otlp exporter")
	}

	// Register the exporter with the TracerProvider using
	// a BatchSpanProcessor
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String("go-example"),
		)),
	)
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	return func() {
		ctx := context.Background()
		// Shutdown will flush any remaining spans and shut down the exporter
		err := provider.Shutdown(ctx)
		if err != nil {
			log.Ctx(ctx).Err(err).Msg("Err in otlp shutdown function")
		}
	}
}

// Define the handler used by gin middleware
func postStartFunc() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		// Instrument the http client
		client := http.Client{
			Transport: otelhttp.NewTransport(http.DefaultTransport),
		}
		re, err := http.NewRequestWithContext(
			// Pass request context rather than gin context
			ctx.Request.Context(),
			"GET",
			"https://pokeapi.co/api/v2/pokedex/kanto/",
			nil,
		)
		if err != nil {
			fmt.Print(err.Error())
			os.Exit(1)
		}
		response, err := client.Do(re)
		if err != nil {
			fmt.Print(err.Error())
			os.Exit(1)
		}
		responseData, err := ioutil.ReadAll(response.Body)
		if err != nil {
			fmt.Print(err.Error())
			os.Exit(1)
		}
		// Write response to page
		ctx.Writer.Write(responseData)
		// fmt.Println(string(responseData))

		// Execute the pending handlers in the chain
		ctx.Next()
	}
}

func main() {
	gogc, err := strconv.ParseInt(os.Getenv("CUSTOM_GOGC"), 10, 64)
	if err != nil || gogc == 0 {
		gogc = 100
	}
	debug.SetGCPercent(int(gogc))
	cleanup := initTracer()
	defer cleanup()

	engine := gin.New()
	engine.Use(otelgin.Middleware("Go-service-middleware"))
	health := engine.Group("/health")
	health.GET("/v2", postStartFunc())
	engine.Run()
}
