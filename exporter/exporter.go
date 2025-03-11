package exporter

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/Jeffail/gabs/v2"
	"github.com/prometheus/client_golang/prometheus"
)

type Exporter struct {
	logger  *slog.Logger
	counter *prometheus.Desc
	namespace *string
	statsDirectory *string
	statsList []string
}

type Player struct {
	ID   string `json:"uuid"`
	Name string `json:"username"`
}

func NewExporter(logger *slog.Logger, namespace string, statsDirectory string, statsList []string) (*Exporter, error) {

	return &Exporter{
		logger: logger,
		namespace: &namespace,
		statsDirectory: &statsDirectory,
		statsList: statsList,
		counter: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "counter"),
			"",
			[]string{"player", "namespace", "type"},
			nil,
		),
	}, nil

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

	files, err := os.ReadDir(*e.statsDirectory)
	if err != nil {
		return err
	}

	for _, file := range files {
		e.logger.Info(file.Name())
		playerDirName := fmt.Sprintf("%s/%s", *e.statsDirectory, file.Name())

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

			for _, statName := range e.statsList {
				statPath := fmt.Sprintf("advancementData.%s", statName)
				statData := jsonParsed.Path(statPath).Data().(float64)
				ch <- prometheus.MustNewConstMetric(
					e.counter,
					prometheus.CounterValue,
					statData,
					player.Name, *e.namespace, statName,
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
