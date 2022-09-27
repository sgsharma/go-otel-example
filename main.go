package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"runtime/debug"
	"strconv"

	"github.com/gin-gonic/gin"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	// instrument the gin-gonic web server
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	// instrument HTTP requests
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.12.0"
	"go.opentelemetry.io/otel/trace"
	_ "go.uber.org/automaxprocs"
)

func initTracer() func() {

	ctx := context.Background()

	exporter, err := otlptracegrpc.New(ctx)
	if err != nil {
		log.Fatal(err)
	}
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resource.NewWithAttributes(semconv.SchemaURL, semconv.ServiceNameKey.String("ExampleService"))),
	)
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	return func() {
		ctx := context.Background()
		err := provider.Shutdown(ctx)
		if err != nil {
			log.Fatal(err)
		}
	}
}

type temp struct{}

func (t temp) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// HTTP transport to instrument http.Client:
	client := http.Client{
		Transport: otelhttp.NewTransport(http.DefaultTransport),
	}
	ctx := r.Context()
	span := trace.SpanFromContext(r.Context())
	span.SetAttributes(
		attribute.String("foo", "bar"),
	)
	//
	re, err := http.NewRequestWithContext(
		ctx,
		"GET",
		"http://pokeapi.co/api/v2/pokedex/kanto/",
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
	fmt.Fprint(w, string(responseData))
}

// Wrap the HTTP handler func with OTel HTTP instrumentation
func wrapHandler(f http.Handler, fname string) http.Handler {
	wrappedHandler := otelhttp.NewHandler(f, fname)
	http.Handle("/", wrappedHandler)
	return wrappedHandler
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
	engine.GET("/", gin.WrapH(wrapHandler(temp{}, "getPokedex")))
	engine.Run()
}
