# aaa-synthetic-exporter
This app support for AAA synthetic test and exports a Prometheus formatted result.

The app works very similar (almost identical) to blackbox_exporter. In fact,
the app leverages much of the blackbox_exporter code and logic.  In other words,
I borrowed heavily and in some cases copied at least the logic/concepts.

## Supported Endpoints:
* `/metrics` - (GET) renders the Apps Prometheus metrics
* `/config` - (GET) renders the current config file
* `/-/reload` - (POST) reloads the config file
* `/v1/probe?module=<module>&target=<target>` - (GET) performs the requested module and renders the module's Prometheus metrics
    * `<module>` - RADIUS or TACACS
    * `<target>` - RADIUS or TACACS server's FQDN

#### Probe Examples
```bash
curl 'http://localhost:9115/v1/probe?module=tacacs&target=tacacs01.example.net'
# HELP probe_duration_seconds Duration of probe
# TYPE probe_duration_seconds gauge
probe_duration_seconds 0.144547083
# HELP probe_status_code Response status code
# TYPE probe_status_code gauge
probe_status_code 1
```
