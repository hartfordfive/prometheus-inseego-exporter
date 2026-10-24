package nmea

import (
	"errors"
	"math"
	"strings"
	"testing"
	"time"
)

// withChecksum appends a correct checksum to a sentence body (without '$').
func withChecksum(body string) string {
	var cs byte
	for i := 0; i < len(body); i++ {
		cs ^= body[i]
	}
	return "$" + body + "*" + string("0123456789ABCDEF"[cs>>4]) + string("0123456789ABCDEF"[cs&0xF])
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

func TestParseRMC(t *testing.T) {
	// Sample sentence from the original Python collector.
	line := withChecksum("GPRMC,144326.00,A,5107.0017737,N,11402.3291611,W,0.080,323.3,210307,0.0,E,A")
	s, err := Parse(line)
	if err != nil {
		t.Fatal(err)
	}
	if s.Talker != "GP" || s.Type != "RMC" {
		t.Fatalf("got talker=%q type=%q", s.Talker, s.Type)
	}
	r, err := ParseRMC(s)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Valid || !r.HasPosition {
		t.Fatalf("expected valid fix with position: %+v", r)
	}
	if !near(r.Latitude, 51+7.0017737/60) {
		t.Errorf("lat = %v", r.Latitude)
	}
	if !near(r.Longitude, -(114 + 2.3291611/60)) {
		t.Errorf("lon = %v", r.Longitude)
	}
	if r.SpeedKnots != 0.08 || r.TrackTrue != 323.3 || r.MagVar != 0 || r.Mode != "A" {
		t.Errorf("unexpected fields: %+v", r)
	}
	want := time.Date(2007, 3, 21, 14, 43, 26, 0, time.UTC)
	if !r.Time.Equal(want) {
		t.Errorf("time = %v, want %v", r.Time, want)
	}
}

func TestParseNULPadded(t *testing.T) {
	// Captured from a MiFi X PRO 5G: sentences are NUL-padded fixed-size records,
	// with the NMEA 4.1 navigational status field at the end of RMC.
	line := strings.Repeat("\x00", 120) + "$GPRMC,225413.00,A,4526.537657,N,07353.356082,W,0.0,,221026,13.4,W,A,V*59\r"
	s, err := Parse(line)
	if err != nil {
		t.Fatal(err)
	}
	r, err := ParseRMC(s)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Valid || !near(r.Latitude, 45+26.537657/60) || !near(r.Longitude, -(73+53.356082/60)) || r.MagVar != -13.4 || r.Mode != "A" {
		t.Errorf("unexpected: %+v", r)
	}
}

func TestParseRMCNoFix(t *testing.T) {
	s, err := Parse(withChecksum("GNRMC,,V,,,,,,,,,,N"))
	if err != nil {
		t.Fatal(err)
	}
	r, err := ParseRMC(s)
	if err != nil {
		t.Fatal(err)
	}
	if r.Valid || r.HasPosition || !r.Time.IsZero() || !math.IsNaN(r.TrackTrue) {
		t.Errorf("expected empty fix: %+v", r)
	}
}

func TestParseGGA(t *testing.T) {
	s, err := Parse(withChecksum("GPGGA,123519,4807.038,N,01131.000,E,1,08,0.9,545.4,M,46.9,M,,"))
	if err != nil {
		t.Fatal(err)
	}
	g, err := ParseGGA(s)
	if err != nil {
		t.Fatal(err)
	}
	if g.FixQuality != 1 || g.Satellites != 8 || g.HDOP != 0.9 || !g.HasAltitude || g.AltitudeM != 545.4 || g.GeoidSepM != 46.9 {
		t.Errorf("unexpected: %+v", g)
	}
}

func TestParseErrors(t *testing.T) {
	if _, err := Parse("$GPRMC,1,2,3*FF"); !errors.Is(err, ErrChecksum) {
		t.Errorf("want checksum error, got %v", err)
	}
	for _, l := range []string{"", "garbage", "$GP", "GPRMC,1,2"} {
		if _, err := Parse(l); !errors.Is(err, ErrNotNMEA) {
			t.Errorf("%q: want ErrNotNMEA, got %v", l, err)
		}
	}
}
