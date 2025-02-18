package main

import (
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/prometheus/client_golang/prometheus"
	v2 "github.com/prometheus/client_golang/prometheus/collectors/version"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/promslog"
	promslogFlag "github.com/prometheus/common/promslog/flag"
	"github.com/prometheus/common/version"
	"github.com/prometheus/exporter-toolkit/web"
	webflag "github.com/prometheus/exporter-toolkit/web/kingpinflag"
)

type Exporter struct {
	logger  *slog.Logger
	counter *prometheus.Desc
}

const Namespace string = "cobblemon"

func main() {
	promslogConfig := &promslog.Config{
		Level: &promslog.AllowedLevel{},
	}

	promslogFlag.AddFlags(kingpin.CommandLine, promslogConfig)
	kingpin.Version(version.Print("minecraft_exporter"))
	kingpin.HelpFlag.Short('h')
	flag := webflag.AddFlags(kingpin.CommandLine, ":9155")

	kingpin.Parse()
	logger := promslog.New(promslogConfig)
	logger.Info("Starting cobblemon-exporter", "version", version.Info())

	prometheus.MustRegister(v2.NewCollector("cobblemon-exporter"))

	exporter, err := constructExporter(logger)
	if err != nil {
		logger.Error("Failed to create exporter", "err", err)
	}
	prometheus.MustRegister(exporter)

	http.Handle("/metrics", promhttp.Handler())

	go func() {
		logger.Info("Listening on address", "address", ":9155")
		server := &http.Server{
			Addr:             (*flag.WebListenAddresses)[0],
			ReadHeaderTimeout: 60 * time.Second,
		}
		if err := web.ListenAndServe(server, flag, logger); err != nil {
			logger.Error("Error running HTTP server", "err", err)
			os.Exit(1)
		}
	}()

	done := make(chan struct{})
	go func() {
		logger.Info("Listening signals...")
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		<-c
		close(done)
	}()
	<-done

	logger.Info("Shutting down...")
}

func constructExporter(logger *slog.Logger) (*Exporter, error) {

	return &Exporter{
		logger: logger,
		counter: prometheus.NewDesc(
			prometheus.BuildFQName(Namespace, "", "counter"),
			"",
			[]string{"player", "namespace", "type"},
			nil,
		),
	}, nil

}

func (e *Exporter) getPlayerStats(ch chan<- prometheus.Metric) error {
	ch <- prometheus.MustNewConstMetric(
		e.counter,
		prometheus.CounterValue,
		12,
		"Alexis", Namespace, "test",
	)
	return nil
}

func (e *Exporter) Collect(metrics chan<- prometheus.Metric) {
	err := e.getPlayerStats(metrics)
	if err != nil {
		e.logger.Error("Fail to get player stats", "err", err)
	}
}

func (e *Exporter) Describe(descs chan<- *prometheus.Desc) {
	descs <- e.counter
}
