package main

import (
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	exporter "github.com/AlexisHutin/cobblemon-prometheus-exporter/exporter"
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

const Namespace string = "cobblemon"
const StatsDirectory string = "./testData/cobblemonplayerdata"

var StatsList = []string{
	"totalCaptureCount",
	"totalEggsHatched",
	"totalEvolvedCount",
	"totalBattleVictoryCount",
	"totalPvPBattleVictoryCount",
	"totalPvWBattleVictoryCount",
	"totalPvNBattleVictoryCount",
	"totalShinyCaptureCount",
	"totalTradedCount",
}

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

	exporter, err := exporter.NewExporter(logger, Namespace, StatsDirectory, StatsList)
	if err != nil {
		logger.Error("Failed to create exporter", "err", err)
	}
	prometheus.MustRegister(exporter)

	http.Handle("/metrics", promhttp.Handler())

	go func() {
		logger.Info("Listening on address", "address", ":9155")
		server := &http.Server{
			Addr:              (*flag.WebListenAddresses)[0],
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
