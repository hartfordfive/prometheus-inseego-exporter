// Package collector periodically samples the MiFi NMEA stream over TCP and
// exposes the latest GPS state as Prometheus metrics.
package collector

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/hartfordfive/prometheus-inseego-exporter/internal/nmea"
)

const namespace = "inseego"

// Config controls how the collector connects to the MiFi.
type Config struct {
	Address     string        // host:port of the GPS over Wi-Fi NMEA stream
	DialTimeout time.Duration // TCP connect timeout
	ReadTimeout time.Duration // max time per fetch to wait for RMC and GGA sentences
	Interval    time.Duration // time between fetches
	StaleAfter  time.Duration // drop position metrics if no fix update within this window
}

type state struct {
	up            bool
	fetchDuration time.Duration
	lastSentence  time.Time
	rmc           nmea.RMC
	rmcSeen       time.Time
	gga           nmea.GGA
	ggaSeen       time.Time
	sentences     map[string]float64
	parseErrors   map[string]float64
	fetches       float64
	fetchFailures float64
}

// Collector implements prometheus.Collector.
type Collector struct {
	cfg    Config
	logger *slog.Logger
	now    func() time.Time

	mu sync.Mutex
	st state

	up, fixValid, lat, lon, speed, track, magVar, fixTime, lastSentence *prometheus.Desc
	altitude, geoidSep, satellites, hdop, fixQuality, mode              *prometheus.Desc
	sentences, parseErrors, fetches, fetchFailures, fetchDuration       *prometheus.Desc
}

// New returns a Collector. Call Run to start fetching.
func New(cfg Config, logger *slog.Logger) *Collector {
	d := func(name, help string, labels ...string) *prometheus.Desc {
		return prometheus.NewDesc(prometheus.BuildFQName(namespace, "gps", name), help, labels, nil)
	}
	return &Collector{
		cfg:    cfg,
		logger: logger,
		now:    time.Now,
		st: state{
			sentences:   map[string]float64{},
			parseErrors: map[string]float64{},
		},
		up:            d("up", "1 if the last fetch from the MiFi NMEA stream received valid sentences."),
		fixValid:      d("fix_valid", "1 if the last RMC sentence reported a valid fix (status A)."),
		lat:           d("latitude_degrees", "Latitude of the last fix in decimal degrees (negative is south)."),
		lon:           d("longitude_degrees", "Longitude of the last fix in decimal degrees (negative is west)."),
		speed:         d("speed_knots", "Speed over ground in knots."),
		track:         d("track_true_degrees", "Course over ground in degrees true."),
		magVar:        d("magnetic_variation_degrees", "Magnetic variation in degrees (negative is west)."),
		fixTime:       d("fix_timestamp_seconds", "UTC time of the last RMC fix as a Unix timestamp."),
		lastSentence:  d("last_sentence_timestamp_seconds", "Unix time at which the last valid NMEA sentence was received."),
		altitude:      d("altitude_meters", "Altitude above mean sea level in meters (GGA)."),
		geoidSep:      d("geoid_separation_meters", "Geoid separation in meters (GGA)."),
		satellites:    d("satellites_used", "Number of satellites used in the fix (GGA)."),
		hdop:          d("hdop", "Horizontal dilution of precision (GGA)."),
		fixQuality:    d("fix_quality", "GGA fix quality (0=invalid, 1=GPS, 2=DGPS, ...)."),
		mode:          d("mode_info", "FAA mode indicator of the last RMC sentence, always 1.", "mode"),
		sentences:     d("sentences_total", "Valid NMEA sentences received, by talker+type.", "sentence"),
		parseErrors:   d("parse_errors_total", "NMEA lines that failed to parse, by reason.", "reason"),
		fetches:       d("fetches_total", "Fetches from the NMEA stream."),
		fetchFailures: d("fetch_failures_total", "Fetches that failed to connect or received no valid sentences."),
		fetchDuration: d("fetch_duration_seconds", "Duration of the last fetch from the NMEA stream."),
	}
}

// Run fetches GPS data immediately and then every Interval until ctx is cancelled.
func (c *Collector) Run(ctx context.Context) {
	t := time.NewTicker(c.cfg.Interval)
	defer t.Stop()
	for {
		c.fetch(ctx)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// fetch connects to the stream, reads until it has both an RMC and a GGA
// sentence (or ReadTimeout expires), then disconnects.
func (c *Collector) fetch(ctx context.Context) {
	start := time.Now()
	err := c.read(ctx)
	if ctx.Err() != nil {
		return
	}
	c.mu.Lock()
	c.st.fetches++
	c.st.fetchDuration = time.Since(start)
	c.st.up = err == nil
	if err != nil {
		c.st.fetchFailures++
	}
	c.mu.Unlock()
	if err != nil {
		c.logger.Warn("GPS fetch failed", "address", c.cfg.Address, "err", err)
	}
}

func (c *Collector) read(ctx context.Context) error {
	dialer := net.Dialer{Timeout: c.cfg.DialTimeout}
	conn, err := dialer.DialContext(ctx, "tcp", c.cfg.Address)
	if err != nil {
		return err
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	if c.cfg.ReadTimeout > 0 {
		_ = conn.SetReadDeadline(time.Now().Add(c.cfg.ReadTimeout))
	}

	var gotAny, gotRMC, gotGGA bool
	sc := bufio.NewScanner(conn)
	for sc.Scan() {
		switch c.handleLine(sc.Text()) {
		case "":
			continue
		case "RMC":
			gotRMC = true
		case "GGA":
			gotGGA = true
		}
		gotAny = true
		if gotRMC && gotGGA {
			return nil
		}
	}
	if gotAny {
		// Some sentences arrived (e.g. firmware without GGA); still a usable fetch.
		return nil
	}
	if err := sc.Err(); err != nil {
		return fmt.Errorf("no valid NMEA sentences received: %w", err)
	}
	return errors.New("no valid NMEA sentences received: connection closed by peer")
}

// handleLine records one NMEA line and returns its sentence type, or "" if
// the line was rejected.
func (c *Collector) handleLine(line string) string {
	now := c.now()
	s, err := nmea.Parse(line)
	if err != nil {
		c.logger.Debug("discarding NMEA line", "line", line, "err", err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	switch {
	case errors.Is(err, nmea.ErrChecksum):
		c.st.parseErrors["checksum"]++
		return ""
	case err != nil:
		c.st.parseErrors["malformed"]++
		return ""
	}
	c.st.sentences[s.Talker+s.Type]++
	c.st.lastSentence = now

	switch s.Type {
	case "RMC":
		r, err := nmea.ParseRMC(s)
		if err != nil {
			c.st.parseErrors["rmc"]++
			return ""
		}
		c.st.rmc, c.st.rmcSeen = r, now
	case "GGA":
		g, err := nmea.ParseGGA(s)
		if err != nil {
			c.st.parseErrors["gga"]++
			return ""
		}
		c.st.gga, c.st.ggaSeen = g, now
	}
	return s.Type
}

// Describe implements prometheus.Collector.
func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	for _, d := range []*prometheus.Desc{
		c.up, c.fixValid, c.lat, c.lon, c.speed, c.track, c.magVar, c.fixTime, c.lastSentence,
		c.altitude, c.geoidSep, c.satellites, c.hdop, c.fixQuality, c.mode,
		c.sentences, c.parseErrors, c.fetches, c.fetchFailures, c.fetchDuration,
	} {
		ch <- d
	}
}

// Collect implements prometheus.Collector.
func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()

	gauge := func(d *prometheus.Desc, v float64, labels ...string) {
		ch <- prometheus.MustNewConstMetric(d, prometheus.GaugeValue, v, labels...)
	}
	counter := func(d *prometheus.Desc, v float64, labels ...string) {
		ch <- prometheus.MustNewConstMetric(d, prometheus.CounterValue, v, labels...)
	}
	fresh := func(t time.Time) bool {
		return !t.IsZero() && (c.cfg.StaleAfter <= 0 || now.Sub(t) <= c.cfg.StaleAfter)
	}
	optGauge := func(d *prometheus.Desc, v float64) {
		if !math.IsNaN(v) {
			gauge(d, v)
		}
	}

	gauge(c.up, boolf(c.st.up))
	counter(c.fetches, c.st.fetches)
	counter(c.fetchFailures, c.st.fetchFailures)
	if c.st.fetches > 0 {
		gauge(c.fetchDuration, c.st.fetchDuration.Seconds())
	}
	for k, v := range c.st.sentences {
		counter(c.sentences, v, k)
	}
	for k, v := range c.st.parseErrors {
		counter(c.parseErrors, v, k)
	}
	if !c.st.lastSentence.IsZero() {
		gauge(c.lastSentence, float64(c.st.lastSentence.UnixNano())/1e9)
	}

	if fresh(c.st.rmcSeen) {
		r := c.st.rmc
		gauge(c.fixValid, boolf(r.Valid))
		if r.Mode != "" {
			gauge(c.mode, 1, r.Mode)
		}
		if r.Valid && r.HasPosition {
			gauge(c.lat, r.Latitude)
			gauge(c.lon, r.Longitude)
			gauge(c.speed, r.SpeedKnots)
			optGauge(c.track, r.TrackTrue)
			optGauge(c.magVar, r.MagVar)
		}
		if !r.Time.IsZero() {
			gauge(c.fixTime, float64(r.Time.UnixNano())/1e9)
		}
	} else {
		gauge(c.fixValid, 0)
	}

	if fresh(c.st.ggaSeen) {
		g := c.st.gga
		gauge(c.fixQuality, float64(g.FixQuality))
		gauge(c.satellites, float64(g.Satellites))
		optGauge(c.hdop, g.HDOP)
		if g.FixQuality > 0 && g.HasAltitude {
			gauge(c.altitude, g.AltitudeM)
			optGauge(c.geoidSep, g.GeoidSepM)
		}
	}
}

func boolf(b bool) float64 {
	if b {
		return 1
	}
	return 0
}
