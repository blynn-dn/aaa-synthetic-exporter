package probe

import (
	"fmt"
	"github.com/blynn-dn/aaa-synthetic-exporter/config"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
	"time"
)

var (
	moduleUnknownCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "aaa_module_unknown_total",
		Help: "Count of unknown modules requested by probes",
	})

	targetUnknownCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "aaa_target_unknown_total",
		Help: "Count of unknown targets requested by probes",
	})

	durationGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "probe_duration_seconds",
		Help: "Duration of probe",
	})

	statusCodeGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "probe_status_code",
		Help: "Response status code",
	})

	// a map of the supported probes
	supportedProbes = map[string]ProbeFn{
		"radius": CheckRadius,
		"tacacs": CheckTacacs,
	}
)

type ProbeFn func(target string, secret string, username string, password string, operation string) Results

type Results struct {
	Success      bool
	StatusCode   int
	Body         interface{}
	ErrorMessage string
}

func init() {
	prometheus.MustRegister(moduleUnknownCounter)
}

// Handler calls the requested module to perform an AAA check
// returns Prometheus formatted results
func Handler(w http.ResponseWriter, r *http.Request, c *config.Config, logger log.Logger) {
	// todo consider setting a context to pass into probes vs the probes creating a context
	params := r.URL.Query()

	moduleName := params.Get("module")
	target := params.Get("target")

	results := Results{}

	fmt.Printf("\nmod: %s, target: %s\n", moduleName, target)

	targetInfo, ok := c.Services[target]
	if !ok {
		http.Error(w, fmt.Sprintf("Unknown target %q", target), http.StatusBadRequest)
		level.Debug(logger).Log("msg", "Unknown target", "target", target)
		targetUnknownCounter.Add(1)
		return
	}
	fmt.Printf("%v, %s", targetInfo, targetInfo.Username)

	// get a reference to the specified probe
	probe, ok := supportedProbes[moduleName]
	if !ok {
		http.Error(w, fmt.Sprintf("Unknown probe %q", moduleName), http.StatusBadRequest)
		level.Debug(logger).Log("msg", "unknown probe", "probe", moduleName)
		moduleUnknownCounter.Add(1)
		return
	}

	// set the timer then call the specified probe
	setupStart := time.Now()
	results = probe(target, targetInfo.Secret, targetInfo.Username, targetInfo.Password, "authenticate")

	// create a new registry and add metrics
	registry := prometheus.NewRegistry()
	registry.MustRegister(durationGauge)
	registry.MustRegister(statusCodeGauge)

	// update the probe metric values
	statusCodeGauge.Set(float64(results.StatusCode))
	durationGauge.Set(time.Since(setupStart).Seconds())

	// return results
	h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	h.ServeHTTP(w, r)
}
