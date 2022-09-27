# go-otel-example

## Quickstart

```
$ export HONEYCOMB_API_KEY=<YOUR-API-KEY>
$ export OTEL_METRICS_EXPORTER="none"
$ export OTEL_EXPORTER_OTLP_ENDPOINT="https://api.honeycomb.io:443"
$ export OTEL_EXPORTER_OTLP_HEADERS="x-honeycomb-team=${HONEYCOMB_API_KEY}"
$ go run main.go
```