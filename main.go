package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Jeffail/gabs/v2"
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
const StatsDirectory string = "./testData/cobblemonplayerdata"
var WantedStats = []string {
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

	exporter, err := constructExporter(logger)
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

type Player struct {
	ID   string `json:"uuid"`
	Name string `json:"username"`
}

func (e *Exporter) getPlayerFromID(id string) (*Player, error) {
	var player Player
	url := fmt.Sprintf("https://api.ashcon.app/mojang/v2/user/%s", id)
	e.logger.Info(url)

	resp, err := http.Get(url)
	if err != nil {
		e.logger.Error("Failed to connect to api.ashcon.app", "err", err)
		return nil, err
	}

	if resp.StatusCode == 200 {
		err := json.NewDecoder(resp.Body).Decode(&player)
		if err != nil {
			e.logger.Error("Failed to connect decode response", "err", err)
			return nil, err
		}
		err = resp.Body.Close()
		if err != nil {
			return nil, err
		}

		return &player, nil
	} else {
		return nil, fmt.Errorf("error retrieving player info from api.ashcon.app: %s", fmt.Sprintf("Status Code: %d", resp.StatusCode))
	}
}

func (e *Exporter) getPlayerStats(ch chan<- prometheus.Metric) error {

	files, err := os.ReadDir(StatsDirectory)
	if err != nil {
		return err
	}

	for _, file := range files {
		e.logger.Info(file.Name())
		playerDirName := fmt.Sprintf("%s/%s", StatsDirectory, file.Name())

		playerDir, err := os.ReadDir(playerDirName)
		if err != nil {
			return err
		}
		for _, playerFile := range playerDir {
			playerID := strings.Split(playerFile.Name(), ".")[0]
			e.logger.Info(playerID)
			player, err := e.getPlayerFromID(playerID)
			if err != nil {
				return err
			}
			e.logger.Info(player.ID)
			e.logger.Info(player.Name)

			playerFilePath := fmt.Sprintf("%s/%s", playerDirName, playerFile.Name())
			file, err := os.ReadFile(playerFilePath)
			if err != nil {
				return err
			}

			jsonParsed, err := gabs.ParseJSON(file)
			if err != nil {
				return err
			}

			for _, statName := range WantedStats {
				statPath := fmt.Sprintf("advancementData.%s", statName)
				statData := jsonParsed.Path(statPath).Data().(float64)
				ch <- prometheus.MustNewConstMetric(
					e.counter,
					prometheus.CounterValue,
					statData,
					player.Name, Namespace, statName,
				)
			}
		}
	}
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
