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
	// instrument the gin-gonic web server
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	// instrument HTTP requests
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.12.0"
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

func main() {
	gogc, err := strconv.ParseInt(os.Getenv("CUSTOM_GOGC"), 10, 64)
	if err != nil || gogc == 0 {
		gogc = 100
	}
	debug.SetGCPercent(int(gogc))
	cleanup := initTracer()
	defer cleanup()
	// gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(otelgin.Middleware("Go-service-middleware"))
	health := engine.Group("/")
	{
		health.GET("", func(ctx *gin.Context) {
			response, err := otelhttp.Get(ctx.Request.Context(), "http://pokeapi.co/api/v2/pokedex/kanto/")

			if err != nil {
				fmt.Print(err.Error())
				os.Exit(1)
			}

			responseData, err := ioutil.ReadAll(response.Body)
			if err != nil {
				fmt.Println(err)
			}
			// fmt.Print(string(responseData))
			ctx.JSON(http.StatusOK, map[string]string{"status": string(responseData)})
		})
	}
	engine.Run()
}
