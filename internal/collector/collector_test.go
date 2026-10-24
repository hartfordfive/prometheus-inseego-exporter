package collector

import (
	"context"
	"io"
	"log/slog"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestCollectorEndToEnd(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		// Same framing as the real device: NUL-padded records. The stream stays
		// open afterwards, so the fetch must stop on its own once it has RMC+GGA.
		io.WriteString(conn, "$GPRMC,bad*FF\r\n\x00\x00")
		io.WriteString(conn, "$GPRMC,144326.00,A,5107.0017737,N,11402.3291611,W,0.080,323.3,210307,0.0,E,A*20\r\n\x00\x00")
		io.WriteString(conn, "$GPGGA,123519,4807.038,N,01131.000,E,1,08,0.9,545.4,M,46.9,M,,*47\r\n\x00\x00")
		time.Sleep(10 * time.Second)
	}()

	c := New(Config{
		Address:     ln.Addr().String(),
		DialTimeout: time.Second,
		ReadTimeout: 5 * time.Second,
		Interval:    time.Minute,
		StaleAfter:  2 * time.Minute,
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	start := time.Now()
	c.fetch(context.Background())
	if d := time.Since(start); d > 2*time.Second {
		t.Fatalf("fetch did not stop after RMC+GGA, took %v", d)
	}

	reg := prometheus.NewPedanticRegistry()
	reg.MustRegister(c)
	want := `
# HELP inseego_gps_fix_valid 1 if the last RMC sentence reported a valid fix (status A).
# TYPE inseego_gps_fix_valid gauge
inseego_gps_fix_valid 1
# HELP inseego_gps_satellites_used Number of satellites used in the fix (GGA).
# TYPE inseego_gps_satellites_used gauge
inseego_gps_satellites_used 8
# HELP inseego_gps_altitude_meters Altitude above mean sea level in meters (GGA).
# TYPE inseego_gps_altitude_meters gauge
inseego_gps_altitude_meters 545.4
# HELP inseego_gps_sentences_total Valid NMEA sentences received, by talker+type.
# TYPE inseego_gps_sentences_total counter
inseego_gps_sentences_total{sentence="GPGGA"} 1
inseego_gps_sentences_total{sentence="GPRMC"} 1
# HELP inseego_gps_parse_errors_total NMEA lines that failed to parse, by reason.
# TYPE inseego_gps_parse_errors_total counter
inseego_gps_parse_errors_total{reason="checksum"} 1
# HELP inseego_gps_up 1 if the last fetch from the MiFi NMEA stream received valid sentences.
# TYPE inseego_gps_up gauge
inseego_gps_up 1
# HELP inseego_gps_fetches_total Fetches from the NMEA stream.
# TYPE inseego_gps_fetches_total counter
inseego_gps_fetches_total 1
`
	if err := testutil.GatherAndCompare(reg, strings.NewReader(want),
		"inseego_gps_fix_valid", "inseego_gps_satellites_used", "inseego_gps_altitude_meters",
		"inseego_gps_sentences_total", "inseego_gps_parse_errors_total", "inseego_gps_up", "inseego_gps_fetches_total",
	); err != nil {
		t.Error(err)
	}
	if n, err := testutil.GatherAndCount(reg, "inseego_gps_latitude_degrees", "inseego_gps_longitude_degrees"); err != nil || n != 2 {
		t.Errorf("expected lat/lon metrics, got n=%d err=%v", n, err)
	}
}

func TestCollectorStale(t *testing.T) {
	c := New(Config{StaleAfter: time.Minute}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	base := time.Now()
	c.now = func() time.Time { return base }
	c.handleLine("$GPRMC,144326.00,A,5107.0017737,N,11402.3291611,W,0.080,323.3,210307,0.0,E,A*20")
	c.now = func() time.Time { return base.Add(2 * time.Minute) }

	reg := prometheus.NewPedanticRegistry()
	reg.MustRegister(c)
	if n, _ := testutil.GatherAndCount(reg, "inseego_gps_latitude_degrees"); n != 0 {
		t.Errorf("expected stale latitude to be dropped, got %d series", n)
	}
	if err := testutil.GatherAndCompare(reg, strings.NewReader(`
# HELP inseego_gps_fix_valid 1 if the last RMC sentence reported a valid fix (status A).
# TYPE inseego_gps_fix_valid gauge
inseego_gps_fix_valid 0
`), "inseego_gps_fix_valid"); err != nil {
		t.Error(err)
	}
}

func TestCollectorSilentStream(t *testing.T) {
	// With GPS turned off, the device accepts connections but sends nothing.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		time.Sleep(2 * time.Second)
	}()

	c := New(Config{
		Address:     ln.Addr().String(),
		DialTimeout: time.Second,
		ReadTimeout: 200 * time.Millisecond,
		Interval:    time.Minute,
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	c.fetch(context.Background())

	reg := prometheus.NewPedanticRegistry()
	reg.MustRegister(c)
	if err := testutil.GatherAndCompare(reg, strings.NewReader(`
# HELP inseego_gps_up 1 if the last fetch from the MiFi NMEA stream received valid sentences.
# TYPE inseego_gps_up gauge
inseego_gps_up 0
# HELP inseego_gps_fetch_failures_total Fetches that failed to connect or received no valid sentences.
# TYPE inseego_gps_fetch_failures_total counter
inseego_gps_fetch_failures_total 1
`), "inseego_gps_up", "inseego_gps_fetch_failures_total"); err != nil {
		t.Error(err)
	}
}
