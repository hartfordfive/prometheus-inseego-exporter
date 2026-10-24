package main

import (
	_ "embed"
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

//go:embed version.txt
var versionFile string

// version is the exporter version, read from version.txt at build time.
var version = strings.TrimSpace(versionFile)

// commit and buildDate are set by the Makefile via -ldflags -X. For a plain
// `go build`, commit falls back to the VCS revision Go embeds in the binary.
var (
	commit    = ""
	buildDate = "unknown"
)

func init() {
	if commit != "" {
		return
	}
	commit = "unknown"
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	var dirty bool
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			commit = s.Value[:min(12, len(s.Value))]
		case "vcs.modified":
			dirty = s.Value == "true"
		}
	}
	if dirty && commit != "unknown" {
		commit += "-dirty"
	}
}

func versionString() string {
	return fmt.Sprintf("prometheus-inseego-exporter %s (commit: %s, build date: %s)", version, commit, buildDate)
}

// buildInfo returns the inseego_exporter_build_info metric: always 1, with the
// build details as labels.
func buildInfo() prometheus.Collector {
	g := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "inseego",
		Name:      "exporter_build_info",
		Help:      "Exporter build information, always 1.",
		ConstLabels: prometheus.Labels{
			"version":    version,
			"commit":     commit,
			"build_date": buildDate,
			"goversion":  runtime.Version(),
		},
	})
	g.Set(1)
	return g
}
