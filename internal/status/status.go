// Package status exposes the MiFi's undocumented /srv/status JSON endpoint
// (radio, connection, traffic, battery and client information) as Prometheus
// metrics. The endpoint is fetched in the background every Interval and
// scrapes are served from the last result.
package status

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const namespace = "inseego"

// Config controls how the collector fetches the status endpoint.
type Config struct {
	URL      string
	Timeout  time.Duration
	Interval time.Duration
}

// Collector implements prometheus.Collector.
type Collector struct {
	cfg    Config
	client *http.Client
	logger *slog.Logger

	mu        sync.Mutex
	data      fields        // last successful response; nil if the last fetch failed
	duration  time.Duration // duration of the last fetch
	lastFetch time.Time     // completion time of the last successful fetch

	up, fetchDuration, lastFetchTime                                    *prometheus.Desc
	connected, connectionDuration, roaming, networkInfo                 *prometheus.Desc
	signalBars, snr                                                     *prometheus.Desc
	bytesRx, bytesTx                                                    *prometheus.Desc
	batteryPercent, batteryCharging, batteryPresent, batteryChargingSrc *prometheus.Desc
	clients, clientsMax, smsUnread                                      *prometheus.Desc
	wifiEnabled, guestWifiEnabled, ethernetEnabled, mobileDataEnabled   *prometheus.Desc
	airplaneMode, simInfo, gpsReceiverInfo, extAntennaInfo, vpnInfo     *prometheus.Desc
	wanInfo, firmwareInfo                                               *prometheus.Desc
}

// New returns a Collector for the given configuration.
func New(cfg Config, logger *slog.Logger) *Collector {
	d := func(name, help string, labels ...string) *prometheus.Desc {
		return prometheus.NewDesc(prometheus.BuildFQName(namespace, "", name), help, labels, nil)
	}
	return &Collector{
		cfg:    cfg,
		client: &http.Client{Timeout: cfg.Timeout},
		logger: logger,

		up:            d("status_up", "1 if the last fetch of the /srv/status endpoint succeeded."),
		fetchDuration: d("status_fetch_duration_seconds", "Time taken by the last fetch of the /srv/status endpoint."),
		lastFetchTime: d("status_last_fetch_timestamp_seconds", "Unix time of the last successful fetch of the /srv/status endpoint."),

		connected:          d("connected", "1 if the cellular data connection state is Connected."),
		connectionDuration: d("connection_duration_seconds", "Duration of the current cellular data connection."),
		roaming:            d("roaming", "1 if the device is roaming."),
		networkInfo:        d("network_info", "Cellular network details, always 1.", "network", "technology", "band", "bandwidth", "pci"),

		signalBars: d("signal_bars", "Signal strength in bars, as shown on the device."),
		snr:        d("signal_snr_db", "Signal-to-noise ratio in dB."),

		bytesRx: d("received_bytes_total", "Bytes received over the cellular connection, as reported by the device."),
		bytesTx: d("transmitted_bytes_total", "Bytes transmitted over the cellular connection, as reported by the device."),

		batteryPercent:     d("battery_percent", "Battery charge level in percent."),
		batteryCharging:    d("battery_charging", "1 if the battery is charging."),
		batteryPresent:     d("battery_present", "1 if a battery is detected."),
		batteryChargingSrc: d("battery_charging_source_info", "Current charging source, always 1.", "source"),

		clients:    d("clients", "Number of connected clients, by type (total, primary, guest, wifi).", "type"),
		clientsMax: d("clients_max", "Maximum number of clients supported, by type (total, wifi).", "type"),
		smsUnread:  d("sms_unread", "Number of unread SMS messages."),

		wifiEnabled:       d("wifi_enabled", "1 if Wi-Fi is enabled."),
		guestWifiEnabled:  d("guest_wifi_enabled", "1 if the guest Wi-Fi network is enabled."),
		ethernetEnabled:   d("ethernet_enabled", "1 if the Ethernet port is enabled."),
		mobileDataEnabled: d("mobile_data_enabled", "1 if mobile data is enabled."),
		airplaneMode:      d("airplane_mode", "1 if airplane mode is on."),

		simInfo:         d("sim_info", "SIM card status, always 1.", "status"),
		gpsReceiverInfo: d("gps_receiver_info", "GPS receiver status as reported by the device, always 1.", "status"),
		extAntennaInfo:  d("external_antenna_info", "External antenna status, always 1.", "status"),
		vpnInfo:         d("vpn_info", "VPN connection state, always 1.", "state"),
		wanInfo:         d("wan_info", "WAN addressing, always 1.", "ipv4_address", "ipv6_address", "gateway"),
		firmwareInfo:    d("firmware_info", "Installed firmware version, always 1.", "version"),
	}
}

// Describe implements prometheus.Collector.
func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	for _, d := range []*prometheus.Desc{
		c.up, c.fetchDuration, c.lastFetchTime,
		c.connected, c.connectionDuration, c.roaming, c.networkInfo,
		c.signalBars, c.snr, c.bytesRx, c.bytesTx,
		c.batteryPercent, c.batteryCharging, c.batteryPresent, c.batteryChargingSrc,
		c.clients, c.clientsMax, c.smsUnread,
		c.wifiEnabled, c.guestWifiEnabled, c.ethernetEnabled, c.mobileDataEnabled,
		c.airplaneMode, c.simInfo, c.gpsReceiverInfo, c.extAntennaInfo, c.vpnInfo,
		c.wanInfo, c.firmwareInfo,
	} {
		ch <- d
	}
}

// Run fetches the endpoint immediately and then every Interval until ctx is cancelled.
func (c *Collector) Run(ctx context.Context) {
	t := time.NewTicker(c.cfg.Interval)
	defer t.Stop()
	for {
		c.refresh(ctx)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// refresh fetches the endpoint once and stores the result for Collect.
func (c *Collector) refresh(ctx context.Context) {
	start := time.Now()
	data, err := c.fetch(ctx)
	if ctx.Err() != nil {
		return
	}
	if err != nil {
		c.logger.Warn("cannot fetch MiFi status", "url", c.cfg.URL, "err", err)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data, c.duration = data, time.Since(start)
	if err == nil {
		c.lastFetch = time.Now()
	}
}

// Collect implements prometheus.Collector. It reports the result of the last
// background fetch; metrics from a failed fetch are not exported.
func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	c.mu.Lock()
	data, duration, lastFetch := c.data, c.duration, c.lastFetch
	c.mu.Unlock()

	if !lastFetch.IsZero() {
		ch <- prometheus.MustNewConstMetric(c.lastFetchTime, prometheus.GaugeValue, float64(lastFetch.UnixNano())/1e9)
	}
	if duration > 0 {
		ch <- prometheus.MustNewConstMetric(c.fetchDuration, prometheus.GaugeValue, duration.Seconds())
	}
	if data == nil {
		ch <- prometheus.MustNewConstMetric(c.up, prometheus.GaugeValue, 0)
		return
	}
	ch <- prometheus.MustNewConstMetric(c.up, prometheus.GaugeValue, 1)
	c.emit(ch, data)
}

func (c *Collector) fetch(ctx context.Context) (fields, error) {
	ctx, cancel := context.WithTimeout(ctx, c.cfg.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.cfg.URL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected HTTP status %s", resp.Status)
	}
	var body struct {
		StatusData fields `json:"statusData"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	if body.StatusData == nil {
		return nil, fmt.Errorf("response has no statusData object")
	}
	return body.StatusData, nil
}

func (c *Collector) emit(ch chan<- prometheus.Metric, f fields) {
	gauge := func(d *prometheus.Desc, v float64, labels ...string) {
		ch <- prometheus.MustNewConstMetric(d, prometheus.GaugeValue, v, labels...)
	}
	// num exports a numeric field as a gauge, skipping it if absent or non-numeric.
	num := func(d *prometheus.Desc, key string, labels ...string) {
		if v, ok := f.number(key); ok {
			gauge(d, v, labels...)
		}
	}
	counter := func(d *prometheus.Desc, key string) {
		if v, ok := f.number(key); ok {
			ch <- prometheus.MustNewConstMetric(d, prometheus.CounterValue, v)
		}
	}
	// is exports 1 if a string field equals want, 0 otherwise; skipped if absent.
	is := func(d *prometheus.Desc, key string, want ...string) {
		if s, ok := f.str(key); ok {
			gauge(d, boolf(contains(want, s)))
		}
	}
	// info exports an always-1 gauge carrying the field as a label; skipped if absent.
	info := func(d *prometheus.Desc, key string) {
		if s, ok := f.str(key); ok {
			gauge(d, 1, s)
		}
	}

	is(c.connected, "statusBarConnectionState", "Connected")
	num(c.connectionDuration, "statusBarConnectionDuration")
	if s, ok := f.str("statusBarRoaming"); ok {
		gauge(c.roaming, boolf(s != "None" && s != "" && s != "0"))
	}
	if network, ok := f.str("statusBarNetwork"); ok {
		tech, _ := f.str("statusBarTechnology")
		band, _ := f.str("statusBarBand")
		bw, _ := f.str("statusBarBandwidth")
		pci, _ := f.str("statusBarPCI")
		gauge(c.networkInfo, 1, network, tech, band, bw, pci)
	}

	num(c.signalBars, "statusBarSignalBars")
	num(c.snr, "statusBarSNR")
	counter(c.bytesRx, "statusBarBytesReceived")
	counter(c.bytesTx, "statusBarBytesTransmitted")

	num(c.batteryPercent, "statusBarBatteryPercent")
	is(c.batteryCharging, "statusBarBatteryChargingState", "true", "1")
	is(c.batteryPresent, "statusBarBatteryDetection", "Present")
	info(c.batteryChargingSrc, "statusBarBatteryChargingSource")

	num(c.clients, "statusBarClientListSize", "total")
	num(c.clients, "statusBarPrimaryClientListSize", "primary")
	num(c.clients, "statusBarGuestClientListSize", "guest")
	num(c.clients, "statusBarWiFiClientListSize", "wifi")
	num(c.clientsMax, "statusBarMaxClientListSize", "total")
	num(c.clientsMax, "statusBarMaxWiFiClientListSize", "wifi")
	num(c.smsUnread, "statusBarSmsUnreadCount")

	is(c.wifiEnabled, "statusBarWiFiEnabled", "1", "true", "enabled")
	is(c.guestWifiEnabled, "statusBarGuestWifiEnabled", "1", "true", "enabled")
	is(c.ethernetEnabled, "statusBarEthernetPortEnabled", "1", "true", "enabled")
	is(c.mobileDataEnabled, "statusBarMobileDataEnabled", "1", "true", "enabled")
	is(c.airplaneMode, "statusBarAirplaneMode", "AirplaneModeOn")

	info(c.simInfo, "statusBarSimStatus")
	info(c.gpsReceiverInfo, "statusBarGpsStatus")
	info(c.extAntennaInfo, "statusExtAntennaEnabled")
	info(c.vpnInfo, "statusBarVpnConnectionState")
	if ip, ok := f.str("internetStatusIPAddress"); ok {
		ip6, _ := f.str("internetStatusIPv6Address")
		gw, _ := f.str("internetStatusGateway")
		gauge(c.wanInfo, 1, ip, ip6, gw)
	}
	info(c.firmwareInfo, "StatusBarSoftwareUpdateSourceVersion")
}

// fields holds the raw statusData object. The device returns a mix of JSON
// strings and numbers, sometimes with stray whitespace.
type fields map[string]json.RawMessage

// str returns a field as a trimmed string, accepting JSON strings and numbers.
func (f fields) str(key string) (string, bool) {
	raw, ok := f[key]
	if !ok {
		return "", false
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return strings.TrimSpace(s), true
	}
	var n json.Number
	if err := json.Unmarshal(raw, &n); err == nil {
		return n.String(), true
	}
	return "", false
}

// number returns a field as a float64, accepting JSON numbers and numeric strings.
func (f fields) number(key string) (float64, bool) {
	s, ok := f.str(key)
	if !ok || s == "" {
		return 0, false
	}
	v, err := strconv.ParseFloat(s, 64)
	return v, err == nil
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if strings.EqualFold(v, s) {
			return true
		}
	}
	return false
}

func boolf(b bool) float64 {
	if b {
		return 1
	}
	return 0
}
