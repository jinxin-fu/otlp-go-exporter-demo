/**
 * Created with IntelliJ goland.
 * @Auther: jinxin
 * @Date: 2021/12/08/16:45
 * @Description:
 */
package main

import (
	"context"
	"fmt"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/global"
	controller "go.opentelemetry.io/otel/sdk/metric/controller/basic"
	processor "go.opentelemetry.io/otel/sdk/metric/processor/basic"
	"go.opentelemetry.io/otel/sdk/metric/selector/simple"
	"log"
	"math/rand"
	"strings"
	"time"
)

func initProvider() func() {
	ctx := context.Background()
	//conn, err := grpc.DialContext(ctx,, grpc.WithInsecure(), grpc.WithBlock())
	//handleErr(err, "failed to create gRPC connection to collector")

	opts := []otlpmetricgrpc.Option{
		otlpmetricgrpc.WithInsecure(),
		otlpmetricgrpc.WithEndpoint("192.168.1.51:4317"),
		otlpmetricgrpc.WithReconnectionPeriod(50 * time.Millisecond),
	}
	client := otlpmetricgrpc.NewClient(opts...)
	metricExporter, err := otlpmetric.New(ctx, client)
	handleErr(err, "failed to create metric exporter")

	// Invoke Start numerous times, should return errAlreadyStarted
	for i := 0; i < 10; i++ {
		if err := metricExporter.Start(ctx); err == nil || !strings.Contains(err.Error(), "already started") {
			handleErr(err, "unexpected Start error")
		}
	}

	if err := metricExporter.Shutdown(ctx); err != nil {
		handleErr(err, "failed to Shutdown the exporter")
	}
	// Invoke Shutdown numerous times
	for i := 0; i < 10; i++ {
		if err := metricExporter.Shutdown(ctx); err != nil {
			handleErr(err, "got error (%v) expected none")
		}
	}
	return func() {}

}
func makeLabels(n int) []attribute.KeyValue {
	used := map[string]bool{}
	l := make([]attribute.KeyValue, n)
	for i := 0; i < n; i++ {
		var k string
		for {
			k = fmt.Sprint("k", rand.Intn(1000000000))
			if !used[k] {
				used[k] = true
				break
			}
		}
		l[i] = attribute.String(k, fmt.Sprint("v", rand.Intn(1000000000)))
	}
	return l
}
func main() {
	client := otlpmetricgrpc.NewClient(
		otlpmetricgrpc.WithInsecure(),
		otlpmetricgrpc.WithEndpoint("192.168.1.51:4317"), // opentelemetry-collector address
	)
	ctx := context.Background()
	exp, err := otlpmetric.New(ctx, client)
	if err != nil {
		log.Fatalf("failed to create the collector exporter: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		if err := exp.Shutdown(ctx); err != nil {
			otel.Handle(err)
		}
	}()

	pusher := controller.New(
		processor.NewFactory(
			simple.NewWithExactDistribution(),
			exp,
		),
		controller.WithExporter(exp),
		controller.WithCollectPeriod(2*time.Second),
	)
	global.SetMeterProvider(pusher)
	if err := pusher.Start(ctx); err != nil {
		log.Fatalf("could not start metric controller: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		if err := pusher.Stop(ctx); err != nil {
			otel.Handle(err)
		}
	}()
	meter := global.Meter("label-test-meter")
	conter := metric.Must(meter).
		NewFloat64Counter(
			"an_important_metric_withlabel",
			metric.WithDescription("Measures the cumulative epicness of the app"),
		).Bind(attribute.String("counter_label", "11111111"))
	for i := 0; i < 5; i++ {
		log.Printf("Doing really hard work (%d /10)\n", i+1)
		conter.Add(ctx, 1.0)
	}

	histogram := metric.Must(meter).NewFloat64Histogram(
		"an_histogram_metric_label",
		metric.WithDescription("test demo for Histogram"),
	).Bind(makeLabels(5)...)
	histogram.Record(ctx, 0.14)

	log.Printf("Done!")
}

func handleErr(err error, message string) {
	if err != nil {
		log.Fatalf("%s: %v", message, err)
	}
}
