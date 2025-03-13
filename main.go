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
	"github.com/prometheus/client_golang/prometheus/collectors"
	v2 "github.com/prometheus/client_golang/prometheus/collectors/version"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/promslog"
	promslogFlag "github.com/prometheus/common/promslog/flag"
	"github.com/prometheus/common/version"
	"github.com/prometheus/exporter-toolkit/web"
	webflag "github.com/prometheus/exporter-toolkit/web/kingpinflag"
)

const Namespace string = "cobblemon"

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

type Config struct {
	WorldPath              *string `yaml:"world-path"`
	DisableExporterMetrics *bool   `yaml:"disable-exporter-metrics"`
}

func NewConfig() *Config {
	var (
		worldPath              = kingpin.Flag("mc.world", "Path the to world folder").Envar("MC_WORLD").Default("/minecraft/world").String()
		disableExporterMetrics = kingpin.Flag("web.disable-exporter-metrics", "Disabling collection of exporter metrics (like go_*)").Envar("WEB_DISABLED_EXPORTER_METRICS").Bool()
	)

	return &Config{
		WorldPath:              worldPath,
		DisableExporterMetrics: disableExporterMetrics,
	}
}

func main() {
	config := NewConfig()
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

	statsFile, err := os.ReadFile("stats.yml")
	if err != nil {
		logger.Error("Failed to read stats file", "err", err)
	}

	exporter, err := exporter.NewExporter(logger, Namespace, *config.WorldPath, statsFile)
	if err != nil {
		logger.Error("Failed to create exporter", "err", err)
	}
	prometheus.MustRegister(exporter)

	logger.Info("Disabling collection of exporter metrics (like go_*)", "value", config.DisableExporterMetrics)
	if *config.DisableExporterMetrics {
		prometheus.Unregister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
		prometheus.Unregister(collectors.NewGoCollector())
	}

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
