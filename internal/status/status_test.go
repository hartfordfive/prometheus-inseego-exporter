package status

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func newCollector(t *testing.T, h http.HandlerFunc) *prometheus.Registry {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c := New(Config{URL: srv.URL + "/srv/status", Timeout: 2 * time.Second, Interval: time.Minute},
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	c.refresh(context.Background())
	reg := prometheus.NewPedanticRegistry()
	reg.MustRegister(c)
	return reg
}

func TestCollectFromFixture(t *testing.T) {
	// testdata/status.json is a real MiFi X PRO 5G response with addresses anonymized.
	fixture, err := os.ReadFile("testdata/status.json")
	if err != nil {
		t.Fatal(err)
	}
	reg := newCollector(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/srv/status" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(fixture)
	})

	want := `
# HELP inseego_status_up 1 if the last fetch of the /srv/status endpoint succeeded.
# TYPE inseego_status_up gauge
inseego_status_up 1
# HELP inseego_connected 1 if the cellular data connection state is Connected.
# TYPE inseego_connected gauge
inseego_connected 1
# HELP inseego_connection_duration_seconds Duration of the current cellular data connection.
# TYPE inseego_connection_duration_seconds gauge
inseego_connection_duration_seconds 5323
# HELP inseego_roaming 1 if the device is roaming.
# TYPE inseego_roaming gauge
inseego_roaming 0
# HELP inseego_network_info Cellular network details, always 1.
# TYPE inseego_network_info gauge
inseego_network_info{band="B4",bandwidth="20 MHz",network="ROGERS",pci="0",technology="4G LTE"} 1
# HELP inseego_signal_bars Signal strength in bars, as shown on the device.
# TYPE inseego_signal_bars gauge
inseego_signal_bars 3
# HELP inseego_signal_snr_db Signal-to-noise ratio in dB.
# TYPE inseego_signal_snr_db gauge
inseego_signal_snr_db 6
# HELP inseego_received_bytes_total Bytes received over the cellular connection, as reported by the device.
# TYPE inseego_received_bytes_total counter
inseego_received_bytes_total 4.48022167e+08
# HELP inseego_battery_percent Battery charge level in percent.
# TYPE inseego_battery_percent gauge
inseego_battery_percent 79
# HELP inseego_battery_charging 1 if the battery is charging.
# TYPE inseego_battery_charging gauge
inseego_battery_charging 0
# HELP inseego_clients Number of connected clients, by type (total, primary, guest, wifi).
# TYPE inseego_clients gauge
inseego_clients{type="guest"} 0
inseego_clients{type="primary"} 2
inseego_clients{type="total"} 2
inseego_clients{type="wifi"} 2
# HELP inseego_wifi_enabled 1 if Wi-Fi is enabled.
# TYPE inseego_wifi_enabled gauge
inseego_wifi_enabled 1
# HELP inseego_ethernet_enabled 1 if the Ethernet port is enabled.
# TYPE inseego_ethernet_enabled gauge
inseego_ethernet_enabled 1
# HELP inseego_airplane_mode 1 if airplane mode is on.
# TYPE inseego_airplane_mode gauge
inseego_airplane_mode 0
# HELP inseego_wan_info WAN addressing, always 1.
# TYPE inseego_wan_info gauge
inseego_wan_info{gateway="192.0.2.1",ipv4_address="192.0.2.2",ipv6_address="2001:db8::2"} 1
# HELP inseego_firmware_info Installed firmware version, always 1.
# TYPE inseego_firmware_info gauge
inseego_firmware_info{version="THN-1.53-5.4-2.594.1.2-28.181.4-28.181.4"} 1
`
	names := []string{
		"inseego_status_up", "inseego_connected", "inseego_connection_duration_seconds", "inseego_roaming",
		"inseego_network_info", "inseego_signal_bars", "inseego_signal_snr_db", "inseego_received_bytes_total",
		"inseego_battery_percent", "inseego_battery_charging", "inseego_clients", "inseego_wifi_enabled",
		"inseego_ethernet_enabled", "inseego_airplane_mode", "inseego_wan_info", "inseego_firmware_info",
	}
	if err := testutil.GatherAndCompare(reg, strings.NewReader(want), names...); err != nil {
		t.Error(err)
	}
}

func TestCollectMissingAndBadFields(t *testing.T) {
	reg := newCollector(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"statusData":{"statusBarSNR":"n/a","statusBarBatteryPercent":55,"statusBarSignalBars":""}}`)
	})
	if n, _ := testutil.GatherAndCount(reg, "inseego_signal_snr_db", "inseego_signal_bars", "inseego_network_info"); n != 0 {
		t.Errorf("expected unparseable/missing fields to be skipped, got %d series", n)
	}
	if err := testutil.GatherAndCompare(reg, strings.NewReader(`
# HELP inseego_battery_percent Battery charge level in percent.
# TYPE inseego_battery_percent gauge
inseego_battery_percent 55
`), "inseego_battery_percent"); err != nil {
		t.Error(err)
	}
}

func TestCollectFailure(t *testing.T) {
	for name, h := range map[string]http.HandlerFunc{
		"http error":   func(w http.ResponseWriter, r *http.Request) { http.Error(w, "nope", http.StatusForbidden) },
		"invalid json": func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "<html>") },
		"no data":      func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, `{"loginStatus":0}`) },
	} {
		t.Run(name, func(t *testing.T) {
			reg := newCollector(t, h)
			if err := testutil.GatherAndCompare(reg, strings.NewReader(`
# HELP inseego_status_up 1 if the last fetch of the /srv/status endpoint succeeded.
# TYPE inseego_status_up gauge
inseego_status_up 0
`), "inseego_status_up"); err != nil {
				t.Error(err)
			}
		})
	}
}
