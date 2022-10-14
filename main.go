package main

import (
	"context"
	"fmt"
	"makeotel/parser"
	"makeotel/tracing"
	"makeotel/version"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const OtlpEndpointEnvVar = "OTEL_EXPORTER_OTLP_ENDPOINT"
const OtlpTracesEndpointEnvVar = "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT"
const MakeOtelDebugEnvVar = "MAKE_OTEL_DEBUG"

func main() {
	err := run(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

type config struct {
	traceParent string
	timestamp   int64

	version bool
	help    bool
}

func helpText() error {
	maxWidth := 110

	sb := strings.Builder{}
	sb.WriteString(`Turns a callgrind format profile from Remake into an OpenTelemetry Trace, and
send it to an OpenTelemetry collector.

Usage:

      makeotel [flags] <path_to_remake_profile>

Trace Flags:

` + traceFlags(&config{}).FlagUsagesWrapped(maxWidth) + `

OpenTelemetry Flags:

` + otelFlags(&tracing.Config{}).FlagUsagesWrapped(maxWidth) + `

General Flags:

` + commandFlags(&config{}).FlagUsagesWrapped(maxWidth))

	fmt.Println(sb.String())
	return nil
}

func traceFlags(conf *config) *pflag.FlagSet {
	flags := pflag.NewFlagSet("trace", pflag.ContinueOnError)
	flags.StringVar(&conf.traceParent, "trace-parent", os.Getenv("TRACEPARENT"), "the trace id to parent the spans to.  Can also be set by TRACEPARENT env var.")
	flags.Int64Var(&conf.timestamp, "timestamp", time.Now().UTC().Unix(), "timestamp of when make was invoked, in unix epoch format")

	return flags
}

func commandFlags(conf *config) *pflag.FlagSet {
	flags := pflag.NewFlagSet("commands", pflag.ContinueOnError)

	flags.BoolVar(&conf.version, "version", false, "print the version of this tool, and exit")
	flags.BoolVar(&conf.help, "help", false, "print the help text, and exit")

	return flags
}

func otelFlags(conf *tracing.Config) *pflag.FlagSet {

	defaultEndpoint := "localhost:4317"
	if val := os.Getenv(OtlpTracesEndpointEnvVar); val != "" {
		defaultEndpoint = val
	} else if val := os.Getenv(OtlpEndpointEnvVar); val != "" {
		defaultEndpoint = val
	}

	defaultDebug := false
	if val, err := strconv.ParseBool(os.Getenv(MakeOtelDebugEnvVar)); err == nil {
		defaultDebug = val
	}

	flags := pflag.NewFlagSet("otel", pflag.ContinueOnError)

	flags.StringVar(&conf.Endpoint, "otlp-endpoint", defaultEndpoint, "A gRPC or HTTP endpoint to send traces to. Can also be set by "+OtlpEndpointEnvVar+" or "+OtlpTracesEndpointEnvVar+" env vars")
	flags.StringSliceVar(&conf.HeadersRaw, "otlp-headers", []string{}, "key value pairs in the form k=v to set as headers")
	flags.BoolVar(&conf.Debug, "otlp-debug", defaultDebug, "Set to true to see debug output from the OTEL Exporter.  Can also be set by "+MakeOtelDebugEnvVar+" env var")

	return flags
}

func run(args []string) error {
	conf := &config{}
	otelConf := &tracing.Config{}

	allFlags := []*pflag.FlagSet{
		traceFlags(conf),
		otelFlags(otelConf),
		commandFlags(conf),
	}

	flags := pflag.NewFlagSet("makeotel", pflag.ContinueOnError)
	for _, set := range allFlags {
		flags.AddFlagSet(set)
	}

	if err := flags.Parse(args); err != nil {
		return err
	}

	if conf.version {
		fmt.Println(version.VersionNumber())
		return nil
	}

	if conf.help {
		return helpText()
	}

	if flags.NArg() != 1 {
		return fmt.Errorf("this program takes one argument: path")
	}

	if err := otelConf.ParseHeaders(); err != nil {
		return err
	}

	file := flags.Arg(0)
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

	shutdown, err := tracing.InitTracer(otelConf)
	if err != nil {
		return err
	}

	ts := time.Unix(conf.timestamp, 0)
	ctx := tracing.WithTraceParent(context.Background(), conf.traceParent)
	spans(ctx, profile, ts, root, nil)

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
