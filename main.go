package main

import (
	"encoding/json"
	"fmt"
	"github.com/blynn-dn/aaa-synthetic-exporter/config"
	"github.com/blynn-dn/aaa-synthetic-exporter/probe"
	"github.com/go-kit/log/level"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/promlog"
	_ "github.com/prometheus/common/promlog/flag"
	"github.com/prometheus/common/version"
	_ "github.com/prometheus/exporter-toolkit/web"
	"gopkg.in/alecthomas/kingpin.v2"
	"gopkg.in/yaml.v3"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var (
	sc = &config.SafeConfig{
		C: &config.Config{},
	}

	configFile  = kingpin.Flag("config.file", "configuration file.").Default("config.yaml").String()
	configCheck = kingpin.Flag("config.check", "If true validate the config file and then exit.").Default().Bool()
	configPort  = kingpin.Arg("port", "port to listen on").Default("9115").String()

	// uncomment the following line to use a new registry
	//promRegistry = prometheus.NewRegistry()
)

// Helper: calls respondWithJSON to send an error message to the requester
func respondWithError(w http.ResponseWriter, code int, message string) {
	respondWithJSON(w, code, map[string]string{"error": message})
}

// Helper: sends a JSON formatted response
func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, _ := json.Marshal(payload)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}

// Handle unsupported request paths
func notFound(w http.ResponseWriter, r *http.Request) {
	respondWithError(w, http.StatusBadRequest, fmt.Sprintf("page not found: %s", r.URL.Path))
}

func init() {
	prometheus.MustRegister(version.NewCollector("aaa_probe"))
	//promRegistry.MustRegister(version.NewCollector("aaa_probe"))
}

func run() int {
	kingpin.CommandLine.UsageWriter(os.Stdout)
	promlogConfig := &promlog.Config{}
	//flag.AddFlags(kingpin.CommandLine, promlogConfig)
	kingpin.Version(version.Print("aaa_probe"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	logger := promlog.New(promlogConfig)

	level.Info(logger).Log("msg", "Starting aaa_probe", "version", version.Info())
	level.Info(logger).Log("build_context", version.BuildContext())

	// create reload config channel
	reloadCh := make(chan chan error)

	// load the config file
	if err := sc.LoadConfig(*configFile, logger); err != nil {
		level.Error(logger).Log("msg", "Error loading config", "err", err)
		return 1
	}

	// command line flag `config.check`
	if *configCheck {
		level.Info(logger).Log("msg", "Config file is ok exiting...")
		return 0
	}
	//fmt.Printf("config: %v\n", sc.C)
	level.Info(logger).Log("msg", "Loaded config file")

	// currently using gorilla mux to simplify routing
	r := mux.NewRouter()

	// call if route not defined/found
	r.NotFoundHandler = http.HandlerFunc(notFound)

	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// provide a simple usage/splash page
		w.Write([]byte(`<html>
			<head><title>AAA Synthetic Exporter</title></head>
			<body>
			<h1>AAA Synthetic Exporter</h1>
			<p><a href='/metrics'>Metrics</a></p>
			<p><a href='/-/reload'>Reload</a></p>
			<p><a href='/config'>Get Current Config</a></p>

			<script>
			document.write("<h2>Probe Examples</h1><blockquote>")			
			document.write("http://" + document.location.hostname + ":9115/v1/probe?module=TACACS&target=tacacs01.example.net<br>");
			document.write("http://" + document.location.hostname + ":9115/v1/probe?module=RADIUS&target=radius01.example.net");
			document.write("</blockquote>")

			document.write("<h2>Reload Config</h2><blockquote>")
			document.write("POST http://" + document.location.hostname + ":9115/-/reload<br></blockquote>")
			</script>

			</body>
			</html>`))
	})

	// process probe module requests
	// Path Args:
	//   module: probe or radius
	//   target: target IP or FQDN
	r.HandleFunc("/v1/probe", func(w http.ResponseWriter, r *http.Request) {
		sc.Lock()
		conf := sc.C
		sc.Unlock()
		probe.Handler(w, r, conf, logger)
	})

	r.Handle("/metrics", promhttp.Handler())
	/*
			// unfortunately tacquito has an init() that add various prom metrics of which we really
			// don't need.  Since there doesn't seem to be a way to remove these metrics, one option is
		    // to use a new registry. But not that this will exclude the various "go" metrics
			r.Handle(
				"/metrics",
				promhttp.InstrumentMetricHandler(
					promRegistry, promhttp.HandlerFor(promRegistry, promhttp.HandlerOpts{})))
	*/

	// handle GET/POST `/config` which returns the current config
	r.HandleFunc("/config", func(w http.ResponseWriter, r *http.Request) {
		sc.RLock()
		c, err := yaml.Marshal(sc.C)
		sc.RUnlock()
		if err != nil {
			level.Warn(logger).Log("msg", "Error marshalling configuration", "err", err)
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		w.Write(c)
	})

	// handle POST `/reload` which reloads the config
	r.HandleFunc("/-/reload",
		func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "POST" {
				w.WriteHeader(http.StatusMethodNotAllowed)
				fmt.Fprintf(w, "This endpoint requires a POST request.\n")
				return
			}

			rc := make(chan error)
			reloadCh <- rc
			if err := <-rc; err != nil {
				http.Error(w, fmt.Sprintf("failed to reload config: %s", err), http.StatusInternalServerError)
			}
		})

	// use a channel for closing the http service on an unexpected interrupt
	srvc := make(chan struct{})

	// listen for SIGTERM signal
	term := make(chan os.Signal, 1)
	signal.Notify(term, os.Interrupt, syscall.SIGTERM)

	// listen for SIGHUP signal
	hup := make(chan os.Signal, 1)
	signal.Notify(hup, syscall.SIGHUP)

	// create the http server
	srv := &http.Server{
		Addr: fmt.Sprintf(":%s", *configPort),
		// Good practice to set timeouts to avoid Slowloris attacks.
		WriteTimeout: time.Second * 15,
		ReadTimeout:  time.Second * 15,
		IdleTimeout:  time.Second * 60,
		Handler:      r, // Pass our instance of gorilla/mux in.
	}

	// Run our server in a goroutine so that it doesn't block.
	go func() {
		if err := srv.ListenAndServe(); err != nil {
			level.Error(logger).Log("msg", "Error starting HTTP server", "err", err)

			// close channel -- this will signal the parent to exit
			close(srvc)
		}
	}()

	// handle config reload
	go func() {
		for {
			select {

			// reload if SIGHUP caught
			case <-hup:
				if err := sc.LoadConfig(*configFile, logger); err != nil {
					level.Error(logger).Log("msg", "Error reloading config", "err", err)
					continue
				}
				level.Info(logger).Log("msg", "Reloaded config file")

			// reload if /reload was requested
			case rc := <-reloadCh:
				if err := sc.LoadConfig(*configFile, logger); err != nil {
					level.Error(logger).Log("msg", "Error reloading config", "err", err)
					rc <- err
				} else {
					level.Info(logger).Log("msg", "Reloaded config file")
					rc <- nil
				}
			}
		}
	}()

	// handle SIGTERM or service interruption
	for {
		select {
		case <-term:
			level.Info(logger).Log("msg", "Received SIGTERM, exiting gracefully...")
			srv.Close()
			return 0
		case <-srvc:
			// http server unexpectedly interrupted/exited
			return 1
		}
	}
}

func main() {
	// call run(); exit with the appropriate exit error code
	os.Exit(run())
}
