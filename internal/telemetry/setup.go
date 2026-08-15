package telemetry

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/KShekhurin/blog-go/config"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/log/global"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func newGrpcConnection(cfg *config.Config) (*grpc.ClientConn, error) {
	return grpc.NewClient(
		cfg.OTLPGrpcAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithPerRPCCredentials(
			uptraceCredentials{
				dsn: cfg.UptraceDSN,
			}),
	)
}

func newTraceExporter(ctx context.Context, conn *grpc.ClientConn) (*otlptrace.Exporter, error) {
	return otlptracegrpc.New(
		ctx,
		otlptracegrpc.WithGRPCConn(conn),
	)
}

func newMetricsExporter(ctx context.Context, conn *grpc.ClientConn) (*otlpmetricgrpc.Exporter, error) {
	return otlpmetricgrpc.New(
		ctx,
		otlpmetricgrpc.WithGRPCConn(conn),
	)
}

func newLogExporter(ctx context.Context, conn *grpc.ClientConn) (*otlploggrpc.Exporter, error) {
	return otlploggrpc.New(
		ctx,
		otlploggrpc.WithGRPCConn(conn),
	)
}

func newTraceProvider(ctx context.Context, res *resource.Resource, conn *grpc.ClientConn) (*sdktrace.TracerProvider, error) {
	exp, err := newTraceExporter(ctx, conn)
	if err != nil {
		return nil, err
	}

	return sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(
			exp,
			sdktrace.WithBatchTimeout(5*time.Second),
		),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(res),
	), nil
}

func newMetricsProvider(ctx context.Context, res *resource.Resource, conn *grpc.ClientConn) (*sdkmetric.MeterProvider, error) {
	exp, err := newMetricsExporter(ctx, conn)
	if err != nil {
		return nil, err
	}

	return sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(
			sdkmetric.NewPeriodicReader(
				exp,
				sdkmetric.WithInterval(5*time.Second),
			),
		),
		sdkmetric.WithResource(res),
	), nil
}

func newLogProvider(ctx context.Context, res *resource.Resource, conn *grpc.ClientConn) (*sdklog.LoggerProvider, error) {
	exp, err := newLogExporter(ctx, conn)
	if err != nil {
		return nil, err
	}

	return sdklog.NewLoggerProvider(
		sdklog.WithProcessor(
			sdklog.NewBatchProcessor(
				exp,
				sdklog.WithExportInterval(5*time.Second),
			),
		),
		sdklog.WithResource(res),
	), nil
}

func SetupOTelSDK(config *config.Config) (func(context.Context) error, error) {
	ctx := context.Background()

	var shutdownFuncs []func(ctx context.Context) error
	var err error

	shutdown := func(ctx context.Context) error {
		slices.Reverse(shutdownFuncs)
		var err error
		for _, fn := range shutdownFuncs {
			err = errors.Join(err, fn(ctx))
		}
		shutdownFuncs = nil
		return err
	}
	handleErr := func(inErr error) {
		err = errors.Join(inErr, shutdown(ctx))
	}
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName("blog-go"),
			semconv.ServiceVersion("1.0.0"),
		),
		resource.WithHost(),
		resource.WithProcess(),
	)
	if err != nil && !errors.Is(err, resource.ErrPartialResource) {
		handleErr(err)
		return shutdown, err
	}

	conn, err := newGrpcConnection(config)

	if err != nil {
		handleErr(err)
		return shutdown, err
	}
	shutdownFuncs = append(shutdownFuncs, func(ctx context.Context) error {
		return conn.Close()
	})

	traceProvider, err := newTraceProvider(ctx, res, conn)
	if err != nil {
		handleErr(err)
		return shutdown, err
	}
	shutdownFuncs = append(shutdownFuncs, traceProvider.Shutdown)
	otel.SetTracerProvider(traceProvider)

	metricProvider, err := newMetricsProvider(ctx, res, conn)
	if err != nil {
		handleErr(err)
		return shutdown, err
	}
	shutdownFuncs = append(shutdownFuncs, metricProvider.Shutdown)
	otel.SetMeterProvider(metricProvider)

	logProvider, err := newLogProvider(ctx, res, conn)
	if err != nil {
		handleErr(err)
		return shutdown, err
	}
	shutdownFuncs = append(shutdownFuncs, logProvider.Shutdown)
	global.SetLoggerProvider(logProvider)

	return shutdown, nil
}
