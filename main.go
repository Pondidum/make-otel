package main

import (
	"context"
	"fmt"
	"makeotel/parser"
	"os"
	"time"

	"github.com/go-logr/logr/funcr"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otlpgrpc "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.12.0"
	"go.opentelemetry.io/otel/trace"
)

func main() {
	err := run(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("this program takes one argument: path")
	}

	file := args[0]

	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()

	p := parser.NewCallgrindParser(f)
	profile, err := p.Parse()
	if err != nil {
		return err
	}

	root := profile.Roots()[0]
	fmt.Println(root.Name)

	ctx, shutdown := initTracer()
	// printCalls(profile, "", start)

	// ctx, end := createRootSpan(profile, start)
	spans(ctx, profile, time.Now(), root, nil)

	shutdown()
	return nil
}

// func printCalls(profile *parser.Profile, indent string, fn *parser.Function) {
// 	for _, call := range fn.Calls() {
// 		fmt.Printf("%s=> %s (%vs)\n", indent, call.CalleeId, call.Cost.Seconds())
// 		fn, found := profile.GetFunction(call.CalleeId)
// 		if found {
// 			printCalls(profile, indent+"  ", fn)
// 		}
// 	}
// }

func initTracer() (context.Context, func()) {
	ctx := context.Background()

	otel.SetLogger(funcr.New(func(prefix, args string) {
		fmt.Println(args)
	}, funcr.Options{Verbosity: 100}))

	exporter, _ := otlpgrpc.New(ctx, otlpgrpc.WithInsecure())

	res, _ := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String("makeotel"),
		))

	ssp := sdktrace.NewSimpleSpanProcessor(exporter)

	// ParentBased/AlwaysSample Sampler is the default and that's fine for this
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(ssp),
	)

	// inject the tracer into the otel globals (and this starts the background stuff, I think)
	otel.SetTracerProvider(tracerProvider)

	// set up the W3C trace context as the global propagator
	otel.SetTextMapPropagator(propagation.TraceContext{})

	return ctx, func() {
		tracerProvider.Shutdown(ctx)
		exporter.Shutdown(ctx)
	}
}

var tr = otel.Tracer("make-otel")

func spans(ctx context.Context, profile *parser.Profile, start time.Time, fn *parser.Function, call *parser.Call) {
	ctx, span := tr.Start(ctx, fn.Name, trace.WithTimestamp(start))

	calls := fn.Called
	if call != nil {
		calls = call.Calls
	}

	span.SetAttributes(
		attribute.String("module", fn.Module),
		attribute.Int("called", calls),
	)

	nextStart := start
	callTotal := time.Duration(0)
	for _, call := range fn.Calls() {
		if calledFn, found := profile.GetFunction(call.CalleeId); found {
			spans(ctx, profile, nextStart, calledFn, call)
			nextStart = nextStart.Add(call.Cost)
			callTotal = callTotal + call.Cost
		}
	}

	if call != nil && callTotal > 0 {
		workTime := call.Cost - callTotal

		_, s := tr.Start(ctx, fn.Name+"_body", trace.WithTimestamp(nextStart))
		s.End(trace.WithTimestamp(nextStart.Add(workTime)))
	}

	duration := profile.TotalCost
	if call != nil {
		duration = call.Cost
	}

	span.End(trace.WithTimestamp(start.Add(duration)))

}
