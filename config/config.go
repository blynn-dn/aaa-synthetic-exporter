package config

import (
	"fmt"
	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	_ "github.com/prometheus/common/config"
	yaml "gopkg.in/yaml.v3"
	"os"
	"sync"
)

var (
	configReloadSuccess = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "aaa_probe",
		Name:      "config_last_reload_successful",
		Help:      "aaa probe exporter config loaded successfully.",
	})

	configReloadSeconds = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "aaa_probe",
		Name:      "config_last_reload_success_timestamp_seconds",
		Help:      "Timestamp of the last successful configuration reload.",
	})
)

type Service struct {
	Username string `yaml:"username,flow"`
	Password string `yaml:"password,omitempty"`
	Secret   string `yaml:"secret"`
}

type Default struct {
	Port string `yaml:"port,omitempty"`
}

type Config struct {
	Services map[string]Service `yaml:"services"`
	Defaults Default            `yaml:"defaults"`
}

type SafeConfig struct {
	sync.RWMutex
	C *Config
}

func init() {
	prometheus.MustRegister(configReloadSuccess)
	prometheus.MustRegister(configReloadSeconds)
}

func (sc *SafeConfig) LoadConfig(confFile string, logger log.Logger) (err error) {
	var c = &Config{}

	defer func() {
		if err != nil {
			configReloadSuccess.Set(0)
		} else {
			configReloadSuccess.Set(1)
			configReloadSeconds.SetToCurrentTime()
		}
	}()

	yamlReader, err := os.Open(confFile)

	fmt.Printf("file: %s\n", confFile)

	if err != nil {
		return fmt.Errorf("error reading config file: %s", err)
	}
	defer yamlReader.Close()
	decoder := yaml.NewDecoder(yamlReader)
	decoder.KnownFields(true)

	if err = decoder.Decode(c); err != nil {
		return fmt.Errorf("error parsing config file: %s", err)
	}

	fmt.Printf("->%v\n", c)

	sc.Lock()
	sc.C = c
	sc.Unlock()

	return nil
}
