# prometheus-inseego-exporter

A Prometheus exporter for Inseego MiFi 5G devices (MiFi X PRO 5G / M3000).
It collects two kinds of data:

- **GPS** – The MiFi's **GPS over Wi-Fi** feature streams raw NMEA 0183
  sentences on a TCP port (default `11010`). Every `gps.interval` (default
  30s) the exporter connects, reads until it has both an `RMC` and a `GGA`
  sentence (usually about a second), disconnects, and exposes the fix.
- **Device status** – The admin web UI polls an undocumented JSON endpoint,
  `http://my.mifi/srv/status`, which answers without logging in. It reports
  the cellular network, signal, traffic counters, battery, connected clients
  and more. The exporter fetches it every `status.interval` (default 30s).

Both fetches run in the background. A Prometheus scrape returns the most
recent results and never waits on the device, so scraping more often than
the intervals just returns the same values.

## Device setup

In the MiFi Admin web UI (`http://my.mifi`), go to **Settings → GPS** and:

1. Turn on **Enable GPS** (accept the GPS agreement).
2. Turn on **GPS over Wi-Fi**.
3. Note the **Port Number** (default `11010`).

The `/srv/status` endpoint needs no setup.

See the "GPS tab" section of the
[MiFi X PRO 5G user guide](https://static.inseego.com/us/download/mifixpro-5g-userguide-global.pdf)
for details. The host running the exporter must be connected to the MiFi
(Wi-Fi, USB or Ethernet).

## Building

Requires Go (see `go.mod` for the version).

| Target           | Description                                                                                 |
|------------------|---------------------------------------------------------------------------------------------|
| `make build`     | Build the binary for the host platform into `build/prometheus-inseego-exporter`.            |
| `make build-rpm` | Cross-compile for Linux and package it as an RPM in `dist/`.                                |
| `make clean`     | Remove `build/` and `dist/`.                                                                |

`build-rpm` uses [nFPM](https://nfpm.goreleaser.com) through `go run`, so it
does not need `rpmbuild` and works on macOS too. It accepts these variables:

| Variable     | Default | Description                                  |
|--------------|---------|----------------------------------------------|
| `RELEASE`    | `1`     | Package release.                             |
| `RPM_GOARCH` | `amd64` | Target architecture (`amd64`, `arm64`, ...). |

```sh
make build-rpm RELEASE=2 RPM_GOARCH=arm64
```

### Versioning

The version is set in [`version.txt`](version.txt). It is embedded in the
binary at build time, so plain `go build` picks it up too, and `make build-rpm`
uses it as the RPM version. To release a new version, edit `version.txt` and
rebuild.

`make build` and `make build-rpm` also embed the git commit hash (with
`-dirty` if there are uncommitted changes) and the UTC build date:

```sh
$ ./build/prometheus-inseego-exporter -version
prometheus-inseego-exporter 0.1.0 (commit: 4ef2fc81884e, build date: 2026-10-24T15:30:00Z)
```

A plain `go build` still reports the commit, taken from Go's built-in VCS
stamping, but shows `build date: unknown`. Go doesn't record a build date
itself.

The RPM installs:

- `/usr/bin/prometheus-inseego-exporter`
- `/etc/prometheus-inseego-exporter/config.yaml` (a copy of
  [`config.example.yaml`](config.example.yaml); local edits are kept on upgrade)

## Running

```sh
./build/prometheus-inseego-exporter -mifi.address=my.mifi:11010
# or
./build/prometheus-inseego-exporter -config.file=/etc/prometheus-inseego-exporter/config.yaml
```

Metrics are served at `http://<host>:9877/metrics`.

## Configuration

You can set every option with a command-line flag, in a YAML file passed with
`-config.file`, or both. Settings are applied in this order, lowest to highest
priority:

1. Built-in defaults
2. Values from the YAML file
3. Flags given explicitly on the command line

So a flag always overrides the file. The file can leave out any key, and
unknown keys are rejected so a typo doesn't go unnoticed. Durations use Go
syntax (`500ms`, `5s`, `2m`).

| Flag                    | YAML key               | Default         | Description                                                                 |
|-------------------------|------------------------|-----------------|-----------------------------------------------------------------------------|
| `-config.file`          | —                      | (none)          | Path to the YAML configuration file.                                        |
| `-version`              | —                      |                 | Print the version, commit hash and build date, then exit.                   |
| `-web.listen-address`   | `web.listen_address`   | `:9877`         | Address to expose metrics on.                                               |
| `-web.telemetry-path`   | `web.telemetry_path`   | `/metrics`      | Path under which to expose metrics.                                         |
| `-mifi.address`         | `mifi.address`         | `my.mifi:11010` | `host:port` of the MiFi GPS over Wi-Fi NMEA stream.                         |
| `-mifi.dial-timeout`    | `mifi.dial_timeout`    | `5s`            | Timeout when connecting to the NMEA stream.                                 |
| `-mifi.read-timeout`    | `mifi.read_timeout`    | `10s`           | Maximum time per GPS fetch to wait for `RMC` and `GGA` sentences.           |
| `-gps.interval`         | `gps.interval`         | `30s`           | Interval between GPS fetches from the NMEA stream.                          |
| `-gps.stale-after`      | `gps.stale_after`      | `90s`           | Stop exporting fix metrics if no successful update for this long. Must be longer than `gps.interval`; `0` disables. |
| `-status.enabled`       | `status.enabled`       | `true`          | Export device metrics from the `/srv/status` endpoint.                      |
| `-status.url`           | `status.url`           | `http://my.mifi/srv/status` | URL of the `/srv/status` endpoint.                              |
| `-status.timeout`       | `status.timeout`       | `5s`            | Timeout when fetching the `/srv/status` endpoint.                           |
| `-status.interval`      | `status.interval`      | `30s`           | Interval between fetches of the `/srv/status` endpoint.                     |
| `-log.level`            | `log.level`            | `info`          | Log level: `debug`, `info`, `warn`, `error`.                                |

Example file ([`config.example.yaml`](config.example.yaml)):

```yaml
web:
  listen_address: ":9877"
  telemetry_path: /metrics
mifi:
  address: my.mifi:11010
  dial_timeout: 5s
  read_timeout: 10s
gps:
  interval: 30s
  stale_after: 90s
status:
  enabled: true
  url: http://my.mifi/srv/status
  timeout: 5s
  interval: 30s
log:
  level: info
```

To turn off a boolean option from the command line, use `=`:
`-status.enabled=false`.

The exporter refuses to start if an interval is `0`, or if `gps.stale_after`
is set but not longer than `gps.interval`. Otherwise fix metrics would vanish
between fetches.

## Metrics

### GPS metrics

These come from the NMEA stream. All are prefixed with `inseego_gps_`.

| Metric                            | Type    | Description                                                                 |
|-----------------------------------|---------|-----------------------------------------------------------------------------|
| `up`                              | gauge   | `1` if the last fetch received valid NMEA sentences.                        |
| `fetches_total`                   | counter | Fetches from the NMEA stream.                                               |
| `fetch_failures_total`            | counter | Fetches that failed to connect or received no valid sentences.              |
| `fetch_duration_seconds`          | gauge   | Duration of the last fetch.                                                 |
| `sentences_total{sentence}`       | counter | Valid NMEA sentences received, by talker and type (e.g. `GPRMC`, `GNGGA`).  |
| `parse_errors_total{reason}`      | counter | Lines rejected (`checksum`, `malformed`, `rmc`, `gga`).                     |
| `last_sentence_timestamp_seconds` | gauge   | Unix time the last valid sentence was received.                             |
| `fix_valid`                       | gauge   | `1` if the last RMC sentence reported a valid fix.                          |
| `mode_info{mode}`                 | gauge   | FAA mode indicator of the last RMC sentence (`A`, `D`, `E`, `N`, ...).      |
| `fix_timestamp_seconds`           | gauge   | UTC time of the last fix, as reported by the GPS.                           |
| `latitude_degrees`                | gauge   | Latitude in decimal degrees (negative is south).                            |
| `longitude_degrees`               | gauge   | Longitude in decimal degrees (negative is west).                            |
| `speed_knots`                     | gauge   | Speed over ground.                                                          |
| `track_true_degrees`              | gauge   | Course over ground, degrees true.                                           |
| `magnetic_variation_degrees`      | gauge   | Magnetic variation (negative is west).                                      |
| `fix_quality`                     | gauge   | GGA fix quality (`0` invalid, `1` GPS, `2` DGPS, ...).                      |
| `satellites_used`                 | gauge   | Satellites used in the fix.                                                 |
| `hdop`                            | gauge   | Horizontal dilution of precision.                                           |
| `altitude_meters`                 | gauge   | Altitude above mean sea level.                                              |
| `geoid_separation_meters`         | gauge   | Geoid separation.                                                           |

Position metrics are only exported while there is a valid fix. If no `RMC` or
`GGA` sentence arrives within `gps.stale_after`, the fix metrics are dropped
and `fix_valid` reports `0`.

When GPS is turned off on the device, the TCP port still accepts connections
but sends nothing. Each fetch then waits `mifi.read_timeout`, sets `up` to
`0`, and increments `fetch_failures_total`. `inseego_gps_receiver_info`
(below) shows the device-side reason, e.g. `status="GpsOff"`.

### Device status metrics

These come from `/srv/status`, fetched every `status.interval`. All are prefixed with
`inseego_`. The *Source field* column is the key in the endpoint's
`statusData` object.

If a field is missing or isn't a number, its metric is left out rather than
reported as `0`, so the exporter copes with firmware versions that report
different fields. If a fetch fails, a warning is logged and, until the next
successful fetch, only `status_up` (`0`), `status_fetch_duration_seconds` and
`status_last_fetch_timestamp_seconds` are exported.

| Metric                                | Type    | Source field                         | Description                                                    |
|---------------------------------------|---------|--------------------------------------|----------------------------------------------------------------|
| `status_up`                           | gauge   | —                                    | `1` if the last fetch of `/srv/status` succeeded.             |
| `status_fetch_duration_seconds`       | gauge   | —                                    | Duration of the last fetch.                                    |
| `status_last_fetch_timestamp_seconds` | gauge   | —                                    | Unix time of the last successful fetch.                        |
| `connected`                           | gauge   | `statusBarConnectionState`           | `1` if the cellular data connection is `Connected`.           |
| `connection_duration_seconds`         | gauge   | `statusBarConnectionDuration`        | Duration of the current cellular data connection.             |
| `roaming`                             | gauge   | `statusBarRoaming`                   | `1` if roaming (any value other than `None`).                 |
| `network_info{network,technology,band,bandwidth,pci}` | gauge | `statusBarNetwork`, `statusBarTechnology`, `statusBarBand`, `statusBarBandwidth`, `statusBarPCI` | Always `1`. The labels carry the operator, technology (e.g. `4G LTE`, `5G`), band (e.g. `B4`, `n71`), channel bandwidth and physical cell ID. |
| `signal_bars`                         | gauge   | `statusBarSignalBars`                | Signal strength in bars, as shown on the device.              |
| `signal_snr_db`                       | gauge   | `statusBarSNR`                       | Signal-to-noise ratio in dB.                                   |
| `received_bytes_total`                | counter | `statusBarBytesReceived`             | Bytes received over the cellular connection.                   |
| `transmitted_bytes_total`             | counter | `statusBarBytesTransmitted`          | Bytes transmitted over the cellular connection.                |
| `battery_percent`                     | gauge   | `statusBarBatteryPercent`            | Battery charge level in percent.                               |
| `battery_charging`                    | gauge   | `statusBarBatteryChargingState`      | `1` if the battery is charging.                                |
| `battery_present`                     | gauge   | `statusBarBatteryDetection`          | `1` if a battery is detected.                                  |
| `battery_charging_source_info{source}`| gauge   | `statusBarBatteryChargingSource`     | Always `1`. The label holds the charging source (e.g. `ChargingSourceNone`). |
| `clients{type="total"}`               | gauge   | `statusBarClientListSize`            | All connected clients.                                         |
| `clients{type="primary"}`             | gauge   | `statusBarPrimaryClientListSize`     | Clients on the primary network.                                |
| `clients{type="guest"}`               | gauge   | `statusBarGuestClientListSize`       | Clients on the guest network.                                  |
| `clients{type="wifi"}`                | gauge   | `statusBarWiFiClientListSize`        | Clients connected over Wi-Fi.                                  |
| `clients_max{type="total"}`           | gauge   | `statusBarMaxClientListSize`         | Maximum number of clients supported.                           |
| `clients_max{type="wifi"}`            | gauge   | `statusBarMaxWiFiClientListSize`     | Maximum number of Wi-Fi clients supported.                     |
| `sms_unread`                          | gauge   | `statusBarSmsUnreadCount`            | Number of unread SMS messages.                                 |
| `wifi_enabled`                        | gauge   | `statusBarWiFiEnabled`               | `1` if Wi-Fi is enabled.                                       |
| `guest_wifi_enabled`                  | gauge   | `statusBarGuestWifiEnabled`          | `1` if the guest Wi-Fi network is enabled.                     |
| `ethernet_enabled`                    | gauge   | `statusBarEthernetPortEnabled`       | `1` if the Ethernet port is enabled.                           |
| `mobile_data_enabled`                 | gauge   | `statusBarMobileDataEnabled`         | `1` if mobile data is enabled.                                 |
| `airplane_mode`                       | gauge   | `statusBarAirplaneMode`              | `1` if airplane mode is on.                                    |
| `sim_info{status}`                    | gauge   | `statusBarSimStatus`                 | Always `1`. The label holds the SIM status (e.g. `Ready`).     |
| `gps_receiver_info{status}`           | gauge   | `statusBarGpsStatus`                 | Always `1`. The label holds the GPS receiver state reported by the device (e.g. `GotFix`). |
| `external_antenna_info{status}`       | gauge   | `statusExtAntennaEnabled`            | Always `1`. The label holds the external antenna state (e.g. `not_connected`). |
| `vpn_info{state}`                     | gauge   | `statusBarVpnConnectionState`        | Always `1`. The label holds the VPN state (e.g. `Not configured`). |
| `wan_info{ipv4_address,ipv6_address,gateway}` | gauge | `internetStatusIPAddress`, `internetStatusIPv6Address`, `internetStatusGateway` | Always `1`. The labels carry the WAN addresses assigned by the carrier. |
| `firmware_info{version}`              | gauge   | `StatusBarSoftwareUpdateSourceVersion` | Always `1`. The label holds the installed firmware version.  |

Notes:

- The `*_info` metrics follow the usual Prometheus pattern: the value is
  always `1` and the data lives in the labels. Join on them, or alert on a
  label changing, e.g. `inseego_network_info{technology!~"5G.*"}`.
- The byte counters are the device's own counters and probably reset when the
  connection or device restarts. `rate()` and `increase()` handle resets
  automatically.
- `/srv/status` does not report RSRP, RSRQ or RSSI. The only radio quality
  values are `signal_snr_db` and `signal_bars`.
- Fields that are not exported: software-update progress fields (internal
  status codes), `statusBarTrafficStatus` (changes from moment to moment),
  `statusBarNetworkID` (contains non-printable characters on some firmware),
  and assorted UI flags.

Example queries:

```promql
# Download throughput in bits per second
rate(inseego_received_bytes_total[5m]) * 8

# Alert when the cellular link is down or the battery is low
inseego_connected == 0
inseego_battery_percent < 20 and inseego_battery_charging == 0
```

### Other metrics

| Metric                                                      | Type  | Description                                                                 |
|-------------------------------------------------------------|-------|-----------------------------------------------------------------------------|
| `inseego_exporter_build_info{version,commit,build_date,goversion}` | gauge | Always `1`. The labels carry the exporter version (from `version.txt`), git commit, build date and Go version. |

The standard Go runtime (`go_*`) and process (`process_*`) metrics are also
exported.

## Prometheus scrape config

```yaml
scrape_configs:
  - job_name: inseego
    static_configs:
      - targets: ["localhost:9877"]
```

## Grafana dashboard

[`grafana/inseego-dashboard.json`](grafana/inseego-dashboard.json) is a
ready-made dashboard for Grafana 10 or later. To import it, go to
**Dashboards → New → Import**, upload the file, and pick your Prometheus data
source. The **Job** and **Instance** dropdowns at the top select which
exporter to show.

It has five sections:

- **Overview** – link state, carrier, technology, band, signal bars, battery,
  SNR, session uptime, client count, data used in the selected time range,
  roaming and GPS fix.
- **Cellular** – download/upload throughput, SNR and bars over time, and
  timelines of technology/band changes and connection/roaming state.
- **Device** – battery level and charging history, connected clients by
  network, on/off state of Wi-Fi, guest Wi-Fi, Ethernet, mobile data and
  airplane mode, SIM, SMS, antenna, firmware, VPN and WAN address.
- **GPS** – the last fix on a map, receiver status, satellites, HDOP,
  altitude, speed, age of the last fix, and fix quality over time.
- **Exporter health** (collapsed) – status endpoint, GPS stream and scrape
  health, age of the last status fetch, fetch durations, fetch failures and
  NMEA parse errors.

Throughput is calculated from the device's byte counters, so it is only as
fine-grained as `status.interval` (30s by default).



## Buffer metrics locally while no access to remote Prometheus storage

If Prometheus or VictoriaMetrics runs on another machine and the MiFi's
connection is unreliable, run a local agent next to the exporter. It scrapes
the exporter, saves samples to disk while the remote server is unreachable,
and pushes them once the connection is back. Use **one** of the two options
below.

Both push using the Prometheus remote-write protocol. The destination URL is:

- VictoriaMetrics: `http://<host>:8428/api/v1/write`
- Prometheus: `http://<host>:9090/api/v1/write` (start Prometheus with
  `--web.enable-remote-write-receiver`)

### Option A: vmagent

1. Download `vmagent` (in the `vmutils` archive) from the
   [VictoriaMetrics releases](https://github.com/VictoriaMetrics/VictoriaMetrics/releases).
2. Create `scrape.yaml`:

   ```yaml
   scrape_configs:
     - job_name: inseego
       scrape_interval: 30s
       static_configs:
         - targets: ["localhost:9877"]
   ```

3. Start it:

   ```sh
   vmagent \
     -promscrape.config=scrape.yaml \
     -remoteWrite.url=http://<host>:8428/api/v1/write \
     -remoteWrite.tmpDataPath=/var/lib/vmagent \
     -remoteWrite.maxDiskUsagePerURL=1GB
   ```

While the destination is unreachable, samples go to
`-remoteWrite.tmpDataPath`. When the buffer reaches
`-remoteWrite.maxDiskUsagePerURL`, the oldest data is dropped.

### Option B: OpenTelemetry Collector

1. Download `otelcol-contrib` from the
   [opentelemetry-collector-releases](https://github.com/open-telemetry/opentelemetry-collector-releases/releases)
   page. The contrib build is required for the Prometheus components.
2. Create `otelcol.yaml`:

   ```yaml
   receivers:
     prometheus:
       config:
         scrape_configs:
           - job_name: inseego
             scrape_interval: 30s
             static_configs:
               - targets: ["localhost:9877"]

   exporters:
     prometheusremotewrite:
       endpoint: http://<host>:8428/api/v1/write
       wal:
         directory: /var/lib/otelcol/wal   # on-disk buffer
       retry_on_failure:
         max_elapsed_time: 0               # keep retrying, never give up

   service:
     pipelines:
       metrics:
         receivers: [prometheus]
         exporters: [prometheusremotewrite]
   ```

3. Start it:

   ```sh
   mkdir -p /var/lib/otelcol/wal
   otelcol-contrib --config otelcol.yaml
   ```

Samples are kept in the WAL directory until the remote write succeeds. The
`wal` option is marked experimental in the collector. If you need the most
reliable offline buffering, prefer vmagent.

## Development

```sh
go test ./...
```

## License

[MIT](LICENSE)
