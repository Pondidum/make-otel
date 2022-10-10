package main

import (
	"context"
	"fmt"
	"makeotel/parser"
	"makeotel/tracing"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
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

	shutdown, err := tracing.InitTracer()
	if err != nil {
		return err
	}

	ctx := tracing.WithTraceParent(context.Background())
	spans(ctx, profile, time.Now(), root, nil)

	shutdown()
	return nil
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

	if call == nil {
		// should be the root span
		span.SetAttributes(
			attribute.String("creator", profile.Creator),
			attribute.String("command", profile.Command),
		)
	}

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

		if workTime > 0 {
			_, s := tr.Start(ctx, fn.Name+"_body", trace.WithTimestamp(nextStart))
			s.End(trace.WithTimestamp(nextStart.Add(workTime)))
		}
	}

	duration := profile.TotalCost
	if call != nil {
		duration = call.Cost
	}

	span.End(trace.WithTimestamp(start.Add(duration)))

}
