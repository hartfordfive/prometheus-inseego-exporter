// Package nmea parses the subset of NMEA 0183 sentences emitted by the
// Inseego MiFi "GPS over Wi-Fi" TCP stream that the exporter cares about.
package nmea

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

var (
	ErrNotNMEA     = errors.New("not an NMEA sentence")
	ErrChecksum    = errors.New("checksum mismatch")
	ErrUnsupported = errors.New("unsupported sentence type")
)

// Sentence is a raw, checksum-validated NMEA sentence split into fields.
type Sentence struct {
	Talker string   // e.g. "GP", "GN"
	Type   string   // e.g. "RMC", "GGA"
	Fields []string // data fields, excluding the address field and checksum
}

// RMC is the Recommended Minimum Specific GNSS Data sentence.
type RMC struct {
	Time        time.Time // UTC date+time of the fix; zero if absent
	Valid       bool      // status field 'A' (valid) vs 'V' (warning)
	HasPosition bool
	Latitude    float64 // decimal degrees, negative for south
	Longitude   float64 // decimal degrees, negative for west
	SpeedKnots  float64
	TrackTrue   float64 // degrees true; NaN if absent
	MagVar      float64 // degrees, negative for west; NaN if absent
	Mode        string  // FAA mode indicator (A, D, E, N, ...), may be empty
}

// GGA is the Global Positioning System Fix Data sentence.
type GGA struct {
	FixQuality  int
	Satellites  int
	HDOP        float64 // NaN if absent
	HasAltitude bool
	AltitudeM   float64 // altitude above mean sea level, meters
	GeoidSepM   float64 // NaN if absent
}

// Parse validates the checksum of a single NMEA line and splits it into fields.
func Parse(line string) (Sentence, error) {
	// The MiFi pads each sentence to a fixed-size record with NUL bytes, so
	// every line after the first starts with a run of 0x00.
	line = strings.Trim(line, "\x00 \t\r\n")
	if len(line) < 7 || line[0] != '$' {
		return Sentence{}, ErrNotNMEA
	}
	body := line[1:]
	if i := strings.IndexByte(body, '*'); i >= 0 {
		want, err := strconv.ParseUint(body[i+1:], 16, 8)
		if err != nil {
			return Sentence{}, fmt.Errorf("%w: bad checksum field %q", ErrChecksum, body[i+1:])
		}
		body = body[:i]
		var got byte
		for j := 0; j < len(body); j++ {
			got ^= body[j]
		}
		if uint64(got) != want {
			return Sentence{}, fmt.Errorf("%w: got %02X want %02X", ErrChecksum, got, want)
		}
	}
	parts := strings.Split(body, ",")
	addr := parts[0]
	if len(addr) < 5 {
		return Sentence{}, ErrNotNMEA
	}
	return Sentence{
		Talker: addr[:len(addr)-3],
		Type:   addr[len(addr)-3:],
		Fields: parts[1:],
	}, nil
}

// ParseRMC decodes an RMC sentence.
func ParseRMC(s Sentence) (RMC, error) {
	if s.Type != "RMC" {
		return RMC{}, ErrUnsupported
	}
	f := pad(s.Fields, 12)
	r := RMC{
		Valid:     f[1] == "A",
		TrackTrue: optFloat(f[7]),
		MagVar:    optFloat(f[9]),
		Mode:      f[11],
	}
	if f[10] == "W" {
		r.MagVar = -r.MagVar
	}
	lat, latOK := coord(f[2], f[3], "S")
	lon, lonOK := coord(f[4], f[5], "W")
	if latOK && lonOK {
		r.HasPosition = true
		r.Latitude, r.Longitude = lat, lon
	}
	if f[6] != "" {
		v, err := strconv.ParseFloat(f[6], 64)
		if err != nil {
			return RMC{}, fmt.Errorf("rmc speed: %w", err)
		}
		r.SpeedKnots = v
	}
	if t, err := time.Parse("020106150405.999999999", f[8]+f[0]); err == nil {
		r.Time = t
	}
	return r, nil
}

// ParseGGA decodes a GGA sentence.
func ParseGGA(s Sentence) (GGA, error) {
	if s.Type != "GGA" {
		return GGA{}, ErrUnsupported
	}
	f := pad(s.Fields, 14)
	g := GGA{
		HDOP:      optFloat(f[7]),
		GeoidSepM: optFloat(f[10]),
	}
	g.FixQuality, _ = strconv.Atoi(f[5])
	g.Satellites, _ = strconv.Atoi(f[6])
	if f[8] != "" {
		v, err := strconv.ParseFloat(f[8], 64)
		if err != nil {
			return GGA{}, fmt.Errorf("gga altitude: %w", err)
		}
		g.HasAltitude, g.AltitudeM = true, v
	}
	return g, nil
}

// coord converts an NMEA (d)ddmm.mmmm value plus hemisphere to signed decimal degrees.
func coord(v, hemi, negHemi string) (float64, bool) {
	if v == "" {
		return 0, false
	}
	raw, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0, false
	}
	deg := float64(int(raw / 100))
	dd := deg + (raw-deg*100)/60
	if hemi == negHemi {
		dd = -dd
	}
	return dd, true
}

func optFloat(v string) float64 {
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return math.NaN()
	}
	return f
}

func pad(f []string, n int) []string {
	for len(f) < n {
		f = append(f, "")
	}
	return f
}
