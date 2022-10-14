package tracing

import (
	"context"
	"fmt"
	"makeotel/version"
	"net"
	"net/url"
	"regexp"
	"strings"

	"github.com/go-logr/logr/funcr"
	"go.opentelemetry.io/otel"
	otlpgrpc "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	otlphttp "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.12.0"
)

func InitTracer(conf *Config) (func(), error) {
	ctx := context.Background()

	if conf.Debug {
		otel.SetLogger(funcr.New(func(prefix, args string) {
			fmt.Println(args)
		}, funcr.Options{Verbosity: 100}))
	}

	exporter, err := createExporter(ctx, conf)
	if err != nil {
		return nil, err
	}

	ssp := sdktrace.NewSimpleSpanProcessor(exporter)

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithResource(
			resource.NewWithAttributes(
				semconv.SchemaURL,
				semconv.ServiceNameKey.String("makefile"),
				semconv.ServiceVersionKey.String(version.VersionNumber()),
			)),
		sdktrace.WithSpanProcessor(ssp),
	)

	otel.SetTracerProvider(tracerProvider)

	// set up the W3C trace context as the global propagator
	otel.SetTextMapPropagator(propagation.TraceContext{})

	return func() {
		tracerProvider.Shutdown(ctx)
		exporter.Shutdown(ctx)
	}, nil
}

type Config struct {
	Endpoint   string
	HeadersRaw []string
	Debug      bool

	Headers map[string]string
}

func (c *Config) ParseHeaders() error {

	headers := map[string]string{}

	for _, pair := range c.Headers {
		s := strings.Split(pair, "=")
		if len(pair) != 2 {
			return fmt.Errorf("expected a key value pair in the form key=value, but got %s", pair)
		}

		header := s[0]
		value := s[1]

		headers[header] = value
	}

	c.Headers = headers
	return nil
}

func createExporter(ctx context.Context, conf *Config) (sdktrace.SpanExporter, error) {

	endpoint := strings.ToLower(conf.Endpoint)
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}

	if strings.HasPrefix(endpoint, "https://") || strings.HasPrefix(endpoint, "http://") {

		opts := []otlphttp.Option{}

		hostAndPort := u.Host
		if u.Port() == "" {
			if u.Scheme == "https" {
				hostAndPort += ":443"
			} else {
				hostAndPort += ":80"
			}
		}
		opts = append(opts, otlphttp.WithEndpoint(hostAndPort))

		if u.Path == "" {
			u.Path = "/v1/traces"
		}
		opts = append(opts, otlphttp.WithURLPath(u.Path))

		if u.Scheme == "http" {
			opts = append(opts, otlphttp.WithInsecure())
		}

		opts = append(opts, otlphttp.WithHeaders(conf.Headers))

		return otlphttp.New(ctx, opts...)
	} else {
		opts := []otlpgrpc.Option{}

		opts = append(opts, otlpgrpc.WithEndpoint(endpoint))

		isLocal, err := isLoopbackAddress(endpoint)
		if err != nil {
			return nil, err
		}

		if isLocal {
			opts = append(opts, otlpgrpc.WithInsecure())
		}

		opts = append(opts, otlpgrpc.WithHeaders(conf.Headers))

		return otlpgrpc.New(ctx, opts...)
	}

}

func isLoopbackAddress(endpoint string) (bool, error) {
	hpRe := regexp.MustCompile(`^[\w.-]+:\d+$`)
	uriRe := regexp.MustCompile(`^(http|https)`)

	endpoint = strings.TrimSpace(endpoint)

	var hostname string
	if hpRe.MatchString(endpoint) {
		parts := strings.SplitN(endpoint, ":", 2)
		hostname = parts[0]
	} else if uriRe.MatchString(endpoint) {
		u, err := url.Parse(endpoint)
		if err != nil {
			return false, err
		}
		hostname = u.Hostname()
	}

	ips, err := net.LookupIP(hostname)
	if err != nil {
		return false, err
	}

	allAreLoopback := true
	for _, ip := range ips {
		if !ip.IsLoopback() {
			allAreLoopback = false
		}
	}

	return allAreLoopback, nil
}
